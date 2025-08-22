package core

import (
	"errors"
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"time"
)

// Scope defines which branches to include.
// Matches the OpenAPI: local | remote | all
//
//go:generate stringer -type=Scope

type Scope int

const (
	ScopeLocal Scope = iota
	ScopeRemote
	ScopeAll
)

// Branch represents a git branch with minimal metadata.
type Branch struct {
	Name              string // short name, e.g., feature/x
	FullRef           string // e.g., refs/heads/feature/x or refs/remotes/origin/x
	IsCurrent         bool
	IsRemote          bool
	Upstream          *string
	HeadCommitSHA     *string
	HeadCommitAt      *time.Time
	LastCommitMessage *string
}

// ListBranchesRequest mirrors listBranches params.
type ListBranchesRequest struct {
	RepoPath string
	Pattern  string
	Scope    Scope
	SortBy   string // "name" | "recency"
	SortDir  string // "asc" | "desc"
	Page     int
	PageSize int
}

// ListBranchesResponse mirrors the OpenAPI response.
type ListBranchesResponse struct {
	Items    []Branch
	Page     int
	PageSize int
	Total    int
	HasPrev  bool
	HasNext  bool
}

// GetCurrentBranch returns the current branch, or an error if detached.
func GetCurrentBranch(repoPath string) (*Branch, error) {
	name, err := git(repoPath, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return nil, err
	}
	name = strings.TrimSpace(name)
	if name == "HEAD" {
		return nil, errors.New("detached HEAD")
	}
	return &Branch{
		Name:      name,
		FullRef:   "refs/heads/" + name,
		IsCurrent: true,
		IsRemote:  false,
	}, nil
}

// ListBranches lists branches with filtering and pagination.
func ListBranches(req ListBranchesRequest) (ListBranchesResponse, error) {
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 50
	}

	var branches []Branch

	// Local branches
	if req.Scope == ScopeLocal || req.Scope == ScopeAll {
		out, err := git(req.RepoPath, "for-each-ref", "--format=%(refname)\t%(objectname)\t%(committerdate:iso-strict)\t%(contents:subject)", "refs/heads/")
		if err != nil {
			return ListBranchesResponse{}, err
		}
		branches = append(branches, parseForEachRef(out, false)...)
	}
	// Remote branches
	if req.Scope == ScopeRemote || req.Scope == ScopeAll {
		out, err := git(req.RepoPath, "for-each-ref", "--format=%(refname)\t%(objectname)\t%(committerdate:iso-strict)\t%(contents:subject)", "refs/remotes/")
		if err != nil {
			return ListBranchesResponse{}, err
		}
		branches = append(branches, parseForEachRef(out, true)...)
	}

	// Mark current
	if cur, err := GetCurrentBranch(req.RepoPath); err == nil {
		for i := range branches {
			if !branches[i].IsRemote && branches[i].Name == cur.Name {
				branches[i].IsCurrent = true
			}
		}
	}

	// Filter by pattern (case-insensitive contains)
	if req.Pattern != "" {
		needle := strings.ToLower(req.Pattern)
		filtered := branches[:0]
		for _, b := range branches {
			if strings.Contains(strings.ToLower(b.Name), needle) {
				filtered = append(filtered, b)
			}
		}
		branches = filtered
	}

	// Sort
	sort.Slice(branches, func(i, j int) bool {
		if req.SortBy == "name" {
			if req.SortDir == "asc" {
				return branches[i].Name < branches[j].Name
			}
			return branches[i].Name > branches[j].Name
		}
		// recency by HeadCommitAt (nil last)
		var ti, tj time.Time
		if branches[i].HeadCommitAt != nil {
			ti = *branches[i].HeadCommitAt
		}
		if branches[j].HeadCommitAt != nil {
			tj = *branches[j].HeadCommitAt
		}
		if req.SortDir == "asc" {
			return ti.Before(tj)
		}
		return ti.After(tj)
	})

	// Paginate
	total := len(branches)
	start := (req.Page - 1) * req.PageSize
	if start > total {
		start = total
	}
	end := start + req.PageSize
	if end > total {
		end = total
	}
	pageItems := append([]Branch(nil), branches[start:end]...)

	resp := ListBranchesResponse{
		Items:    pageItems,
		Page:     req.Page,
		PageSize: req.PageSize,
		Total:    total,
		HasPrev:  req.Page > 1,
		HasNext:  end < total,
	}
	return resp, nil
}

// Checkout switches to a branch (optionally creating/tracking).
func Checkout(repoPath, name string, create bool) (string, error) {
	if strings.TrimSpace(name) == "" {
		return "", errors.New("branch name required")
	}
	var prev string
	if cur, err := GetCurrentBranch(repoPath); err == nil && cur != nil {
		prev = cur.Name
	}

	var args []string
	if create {
		args = []string{"switch", "-c", name}
	} else {
		args = []string{"switch", name}
	}
	if _, err := git(repoPath, args...); err != nil {
		return prev, err
	}
	return prev, nil
}

func parseForEachRef(out string, isRemote bool) []Branch {
	lines := strings.Split(strings.TrimSpace(out), "\n")
	res := make([]Branch, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 4)
		if len(parts) < 4 {
			continue
		}
		fullRef := parts[0]
		sha := parts[1]
		dateStr := parts[2]
		msg := parts[3]
		var tPtr *time.Time
		// iso8601 from git is typically RFC3339 or close enough
		if ts, err := time.Parse(time.RFC3339, dateStr); err == nil {
			tPtr = &ts
		}
		name := fullRef
		if isRemote {
			name = strings.TrimPrefix(fullRef, "refs/remotes/")
		} else {
			name = strings.TrimPrefix(fullRef, "refs/heads/")
		}
		shaCopy := sha
		msgCopy := msg
		b := Branch{
			Name:              name,
			FullRef:           fullRef,
			IsCurrent:         false,
			IsRemote:          isRemote,
			HeadCommitSHA:     &shaCopy,
			HeadCommitAt:      tPtr,
			LastCommitMessage: &msgCopy,
		}
		res = append(res, b)
	}
	return res
}

func git(repoPath string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	if repoPath != "" {
		cmd.Dir = repoPath
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %v failed: %w: %s", args, err, string(out))
	}
	return string(out), nil
}
