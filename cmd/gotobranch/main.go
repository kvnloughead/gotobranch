// Package main contains the gotobranch CLI entrypoint. It wires up
// command-line flags, calls core.ListBranches to list git branches in the
// provided directory, constructs the TUI model, and runs the Bubble Tea
// program loop.
package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"gotobranch/internal/core"
	"gotobranch/internal/tui"
)

func main() {
	repo := flag.String("repo", "", "Path to git repository (defaults to CWD)")
	scopeFlag := flag.String("scope", "local", "Branch scope: local|remote|all")
	pageSize := flag.Int("page-size", 25, "Page size for pagination")
	flag.Parse()

	var scope core.Scope
	switch *scopeFlag {
	case "local":
		scope = core.ScopeLocal
	case "remote":
		scope = core.ScopeRemote
	case "all":
		scope = core.ScopeAll
	default:
		fmt.Println("invalid --scope; use local|remote|all")
		return
	}
	var pattern string
	if flag.NArg() > 0 {
		pattern = flag.Arg(0)
	}

	// Obtain branch data from for the supplied repository (or CWD).
	branchesResp, err := core.ListBranches(core.ListBranchesRequest{
		RepoPath: *repo,
		Pattern:  pattern,
		Scope:    scope,
		SortBy:   "recency",
		SortDir:  "desc",
		PageSize: *pageSize,
	})
	if err != nil {
		fmt.Printf("error listing branches:'%v'", err)
	}

	p := tea.NewProgram(
		tui.New(tui.Options{
			Items: branchesResp.Items,
		}),

		// use the full size of the terminal in its "alternate screen buffer"
		// turn on mouse support so we can track the mouse wheel
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	if _, err := p.Run(); err != nil {
		fmt.Println("could not run program:", err)
		os.Exit(1)
	}
}
