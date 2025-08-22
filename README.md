# gotobranch (working title)

Interactive CLI to navigate and switch git branches. Built in Go with Bubble Tea. Business logic is UI-agnostic and defined via an OpenAPI contract so the TUI/CLI can stay decoupled.

- Spec: `spec/openapi.yaml`

## Usage

# gotobranch — Usage

Prerequisites:
- macOS with Git installed
- Go 1.21+ (to build)

Install/Build:
- Using Makefile (recommended):
  - make build
  - make install        # installs to /usr/local/bin (may require sudo)
  - make dev-install    # installs to $HOME/.local/bin (no sudo)
- Manual build:
  - go build -o bin/gotobranch ./cmd/gotobranch
- Go install into a bin dir on your PATH:
  - GOBIN=$HOME/.local/bin go install ./cmd/gotobranch

Add $HOME/.local/bin to your PATH (zsh):
- echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.zshrc
- source ~/.zshrc

Basic usage:
- Run inside a Git repo:
  - gotobranch
  - gotobranch <pattern>
- Or target a specific repo:
  - gotobranch [pattern] --repo /path/to/repo

Flags:
- --repo <path>            Path to the git repository (defaults to CWD)
- --scope <local|remote|all>  Branch scope (default: local)
- --page-size <n>          Items per page (default: 50)

Interactive keys:
- Move: Up/Down or k/j
- Page: PageUp/PageDown or h/l
- Filter: type to update the pattern
- Clear filter: Tab
- Select/Switch: Enter
- Quit: q or Ctrl+C

Examples:
- List all local branches interactively:
  - gotobranch
- Start filtered to “feat”:
  - gotobranch feat
- Browse remote branches:
  - gotobranch --scope remote
- Target a different repo:
  - gotobranch fix --repo ~/src/myrepo

## Make targets

- make build         # build to bin/gotobranch
- make install       # install to /usr/local/bin (may need sudo)
- make dev-install   # install to $HOME/.local/bin
- make clean         # remove bin/

## Features

- Interactive branch navigation (Bubble Tea TUI)
- Pattern filtering (case-insensitive) with live updates
- Pagination (page/pageSize) with navigation keys
- Sorting by name or recency, asc/desc
- Scope selection: local, remote, or all branches
- Current branch detection (handles detached HEAD)
- Branch metadata: name, full ref, upstream, head commit SHA/time, last message
- Switch to selected branch (git switch/checkout)
- Error handling for dirty working tree (prevent destructive switches)
- CLI flags: --repo, --scope, --page-size; optional [pattern] arg
- Core logic decoupled from UI; defined by OpenAPI spec
- Reusable core for multiple use cases/commands

Planned:
- Create branch if missing (with track remote)
- Sort hotkeys; toggle sort direction
- Remote-to-local tracking checkout UX
- Additional actions framework (delete, merge, diff, etc.)
- Fuzzy matching, recent-history boosting
- Configurable keybindings and page size
- Preview pane (e.g., last commit, diff summary)
- Tests with temp git repo fixtures

## Ideas

- Name/recency sort hotkeys
- “create branch if missing” toggle
- Remote→local tracking checkout support
- Tests for core with a temp repo fixture?