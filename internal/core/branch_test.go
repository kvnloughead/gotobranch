package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Test helpers for creating a temporary git repository, making commits,
// creating branches, and wiring a bare remote. These tests exercise the
// real git CLI via the package's git() helper; no external libs are used.

// newTempDir creates a temporary directory and schedules cleanup.
func newTempDir(t *testing.T, prefix string) string {
	t.Helper()
	dir, err := os.MkdirTemp("", prefix)
	if err != nil {
		t.Fatalf("MkdirTemp failed: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

// runGit wraps the package's git function for tests.
func runGit(t *testing.T, repo string, args ...string) string {
	t.Helper()
	out, err := git(repo, args...)
	if err != nil {
		t.Fatalf("git %v failed: %v", args, err)
	}
	return out
}

// initRepo initializes a repo with an initial commit on the provided branch name.
func initRepo(t *testing.T, branch string) string {
	t.Helper()
	repo := newTempDir(t, "gotobranch-repo-")
	// Initialize repository
	runGit(t, repo, "init")
	// Configure identity for commits
	runGit(t, repo, "config", "user.email", "test@example.com")
	runGit(t, repo, "config", "user.name", "Test User")
	// Create initial file and commit
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("init\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "init")
	// Create/switch to the requested branch explicitly so tests are deterministic.
	// If the default branch already has this name, just stay on it.
	cur := strings.TrimSpace(runGit(t, repo, "rev-parse", "--abbrev-ref", "HEAD"))
	if cur != branch {
		// Create the branch if it doesn't exist yet and switch to it
		// Use `switch -c` which is clearer on modern git; fall back to checkout -b on error
		if _, err := git(repo, "switch", "-c", branch); err != nil {
			// Fallback: use checkout -b
			runGit(t, repo, "checkout", "-b", branch)
		}
	}
	// Add one more commit on that branch so there is a unique tip
	if err := os.WriteFile(filepath.Join(repo, "file.txt"), []byte("on "+branch+"\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "on "+branch)
	return repo
}

// createBranch creates and checks out a new local branch with a commit.
func createBranch(t *testing.T, repo, name string) {
	t.Helper()
	runGit(t, repo, "checkout", "-b", name)
	fn := filepath.Join(repo, strings.ReplaceAll(name, "/", "_")+".txt")
	if err := os.WriteFile(fn, []byte("branch "+name+"\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "commit on "+name)
}

// addBareRemote creates a bare repo and wires it as "origin"; returns bare path.
func addBareRemote(t *testing.T, worktree string) string {
	t.Helper()
	bare := newTempDir(t, "gotobranch-remote-")
	runGit(t, bare, "init", "--bare")
	runGit(t, worktree, "remote", "add", "origin", bare)
	return bare
}

// pushAll pushes the named local branches to origin with upstreams.
func pushAll(t *testing.T, repo string, branches ...string) {
	t.Helper()
	for _, b := range branches {
		runGit(t, repo, "push", "-u", "origin", b)
	}
	// Ensure remote-tracking refs exist locally
	runGit(t, repo, "fetch", "origin")
}

func TestGetCurrentBranch(t *testing.T) {
	repo := initRepo(t, "main")
	// Switch to a known branch and verify
	createBranch(t, repo, "feature/x")
	br, err := GetCurrentBranch(repo)
	if err != nil {
		t.Fatalf("GetCurrentBranch error: %v", err)
	}
	if br.Name != "feature/x" || !br.IsCurrent || br.IsRemote {
		t.Fatalf("unexpected branch: %+v", br)
	}
}

func TestGetCurrentBranch_Detached(t *testing.T) {
	repo := initRepo(t, "main")
	// Get HEAD sha and detach
	sha := strings.TrimSpace(runGit(t, repo, "rev-parse", "HEAD"))
	runGit(t, repo, "switch", "--detach", sha)
	_, err := GetCurrentBranch(repo)
	if err == nil || !strings.Contains(err.Error(), "detached HEAD") {
		t.Fatalf("expected detached HEAD error, got: %v", err)
	}
}

func TestCheckout(t *testing.T) {
	repo := initRepo(t, "main")
	// Create new branch via core.Checkout(create=true)
	prev, err := Checkout(repo, "feature/new", true)
	if err != nil {
		t.Fatalf("Checkout(create) error: %v", err)
	}
	if prev != "main" {
		t.Fatalf("expected prev=main, got %q", prev)
	}
	// Verify current
	cur, err := GetCurrentBranch(repo)
	if err != nil || cur.Name != "feature/new" {
		t.Fatalf("after Checkout, current=%v err=%v", cur, err)
	}
	// Create another branch, switch without create
	createBranch(t, repo, "bugfix/one")
	// Go back to main then switch to bugfix/one without create
	runGit(t, repo, "switch", "main")
	prev, err = Checkout(repo, "bugfix/one", false)
	if err != nil {
		t.Fatalf("Checkout(switch) error: %v", err)
	}
	if prev != "main" {
		t.Fatalf("expected prev=main, got %q", prev)
	}
}

func TestListBranches_ScopesAndFilter(t *testing.T) {
	repo := initRepo(t, "main")
	// Create two more locals
	createBranch(t, repo, "feat/alpha")
	createBranch(t, repo, "fix/beta")
	// Wire a bare remote and push all three locals
	_ = addBareRemote(t, repo)
	pushAll(t, repo, "main", "feat/alpha", "fix/beta")

	tests := []struct {
		name   string
		scope  Scope
		pat    string
		assert func(t *testing.T, got ListBranchesResponse)
	}{
		{
			name:  "local only",
			scope: ScopeLocal,
			pat:   "",
			assert: func(t *testing.T, got ListBranchesResponse) {
				// At least the three locals we created must be present
				req := map[string]bool{"main": false, "feat/alpha": false, "fix/beta": false}
				for _, b := range got.Items {
					if !b.IsRemote {
						req[b.Name] = true
					}
				}
				for n, ok := range req {
					if !ok {
						t.Fatalf("missing local branch %q in %+v", n, got.Items)
					}
				}
			},
		},
		{
			name:  "remote only contains origin/*",
			scope: ScopeRemote,
			pat:   "",
			assert: func(t *testing.T, got ListBranchesResponse) {
				// Must include the pushed remote-tracking refs
				req := map[string]bool{"origin/main": false, "origin/feat/alpha": false, "origin/fix/beta": false}
				for _, b := range got.Items {
					if b.IsRemote {
						req[b.Name] = true
					}
				}
				for n, ok := range req {
					if !ok {
						t.Fatalf("missing remote branch %q in %+v", n, got.Items)
					}
				}
			},
		},
		{
			name:  "all with filter",
			scope: ScopeAll,
			pat:   "feat",
			assert: func(t *testing.T, got ListBranchesResponse) {
				// Expect at least the local and remote matching "feat"
				want := map[string]bool{"feat/alpha": false, "origin/feat/alpha": false}
				for _, b := range got.Items {
					if strings.Contains(b.Name, "feat/alpha") {
						want[b.Name] = true
					}
				}
				if !(want["feat/alpha"] && want["origin/feat/alpha"]) {
					t.Fatalf("filter did not include both local and remote feat/alpha: %+v", got.Items)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := ListBranches(ListBranchesRequest{
				RepoPath: repo,
				Scope:    tc.scope,
				Pattern:  tc.pat,
			})
			if err != nil {
				t.Fatalf("ListBranches error: %v", err)
			}
			tc.assert(t, resp)
		})
	}
}
