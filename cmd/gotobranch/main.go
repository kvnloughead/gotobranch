// Package main contains the gotobranch CLI entrypoint. It wires up
// command-line flags, constructs the TUI model, and runs the Bubble Tea
// program loop.
package main

import (
	"flag"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"gotobranch/internal/core"
	"gotobranch/internal/tui"
)

func main() {
	repo := flag.String("repo", "", "Path to git repository (defaults to CWD)")
	scopeFlag := flag.String("scope", "local", "Branch scope: local|remote|all")
	pageSize := flag.Int("page-size", 50, "Page size for pagination")
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

	m := tui.New(tui.Options{
		RepoPath: *repo,
		Scope:    scope,
		PageSize: *pageSize,
		Pattern:  pattern,
	})

	if _, err := tea.NewProgram(m).Run(); err != nil {
		fmt.Printf("error: %v\n", err)
	}
}
