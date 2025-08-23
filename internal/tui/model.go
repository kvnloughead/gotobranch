// Package tui implements a small interactive terminal UI for browsing and
// switching git branches. It uses the Bubble Tea framework and exposes a
// Model type that implements the Bubble Tea Model interface. The UI
// supports filtering, pagination, and switching branches.
package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/paginator"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"gotobranch/internal/core"
)

type Model struct {
	RepoPath string
	Scope    core.Scope

	input     textinput.Model
	paginator paginator.Model

	items []core.Branch
	total int
	error error

	cursor int // index within current page items

	// mode controls the input semantics:
	// - selectMode: digits buffer + Enter selects a branch by number
	// - filterMode: type-to-filter using the text input
	mode         mode
	numberBuffer string
}

// mode enumerates input modes for the TUI.
type mode int

const (
	selectMode mode = iota
	filterMode
)

// listMsg is a message that tells the model to update the list of branches.
type listMsg struct {
	// Slice of the items to display on the current page.
	items []core.Branch

	// A count of all matches, not just on the current page.
	total int
	err   error
}

type switchMsg struct{ err error }

type Options struct {
	RepoPath string
	Scope    core.Scope
	PageSize int
	Pattern  string
}

// New constructs a TUI Model configured with the provided options.
// - RepoPath: path to the git repository (empty = CWD)
// - Scope: which branches to include (local/remote/all)
// - PageSize: number of items per page (defaults to 50)
// - Pattern: initial filter string
func New(opts Options) Model {
	inp := textinput.New()
	inp.Placeholder = "Filter pattern (press f to edit)"
	inp.SetValue(opts.Pattern)
	// Start in select mode; 'f' will focus the input.

	p := paginator.New()
	if opts.PageSize <= 0 {
		opts.PageSize = 25
	}
	p.PerPage = opts.PageSize

	m := Model{
		RepoPath:  opts.RepoPath,
		Scope:     opts.Scope,
		input:     inp,
		paginator: p,
		mode:      selectMode,
	}
	return m
}

// Init requests the first page of branches when the Bubble Tea
// program starts.
func (m Model) Init() tea.Cmd {
	return m.refreshList()
}

// refreshList returns a command which queries core.ListBranches for
// the current page and filter. The command posts a listMsg with the
// page items and total count which Update will apply to the model.
func (m Model) refreshList() tea.Cmd {
	return func() tea.Msg {
		resp, err := core.ListBranches(core.ListBranchesRequest{
			RepoPath: m.RepoPath,
			Pattern:  strings.TrimSpace(m.input.Value()),
			Scope:    m.Scope,
			SortBy:   "recency",
			SortDir:  "desc",
			Page:     m.paginator.Page + 1,
			PageSize: m.paginator.PerPage,
		})
		if err != nil {
			return listMsg{err: err}
		}
		return listMsg{items: resp.Items, total: resp.Total}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	// Key presses
	case tea.KeyMsg:
		key := msg.String()
		switch m.mode {

		// Select mode key presses
		case selectMode:
			switch key {
			case "ctrl+c", "q":
				return m, tea.Quit
			case "f":
				m.mode = filterMode
				m.input.Focus()
				return m, nil
			case "enter":
				if strings.TrimSpace(m.numberBuffer) != "" {
					n, err := strconv.Atoi(strings.TrimSpace(m.numberBuffer))
					m.numberBuffer = ""
					if err != nil || n <= 0 {
						m.error = fmt.Errorf("invalid selection")
						return m, nil
					}
					return m, m.selectByNumber(n)
				}
				if len(m.items) == 0 {
					return m, nil
				}
				name := m.items[m.cursor].Name
				return m, func() tea.Msg {
					_, err := core.Checkout(m.RepoPath, name, false)
					return switchMsg{err: err}
				}
			case "backspace":
				if len(m.numberBuffer) > 0 {
					m.numberBuffer = m.numberBuffer[:len(m.numberBuffer)-1]
				}
				return m, nil
			case "up", "k":
				if len(m.items) == 0 {
					return m, nil
				}
				if m.cursor > 0 {
					m.cursor--
				} else {
					m.cursor = len(m.items) - 1
				}
				return m, nil
			case "down", "j":
				if len(m.items) == 0 {
					return m, nil
				}
				if m.cursor < len(m.items)-1 {
					m.cursor++
				} else {
					m.cursor = 0
				}
				return m, nil
			case "pgup", "p", "left":
				if m.paginator.Page > 0 {
					m.paginator.PrevPage()
					m.cursor = 0
					return m, m.refreshList()
				}
				return m, nil
			case "pgdn", "n", "right":
				m.paginator.NextPage()
				m.cursor = 0
				return m, m.refreshList()
			case "tab":
				// Clear numeric buffer
				m.numberBuffer = ""
				return m, nil
			default:
				if len(key) == 1 && key[0] >= '0' && key[0] <= '9' {
					m.numberBuffer += key
					return m, nil
				}
			}

		// Filter mode key presses
		case filterMode:
			switch key {
			case "ctrl+c", "q":
				return m, tea.Quit
			case "escape", "f":
				// Exit filter mode
				m.mode = selectMode
				m.input.Blur()
				return m, nil
			case "tab":
				m.input.SetValue("")
				return m, m.refreshList()
			case "pgup", "p", "right":
				if m.paginator.Page > 0 {
					m.paginator.PrevPage()
					m.cursor = 0
					return m, m.refreshList()
				}
				return m, nil
			case "pgdn", "n", "left":
				m.paginator.NextPage()
				m.cursor = 0
				return m, m.refreshList()
			}
		}

	// listMsg tells the model to update the list of items
	case listMsg:
		m.error = msg.err
		if msg.err == nil {
			// If no error, update the model with the data from the message, setup
			// pagination, and clamp cursor between lines 0 and len(msg.items)-1 to
			// ensure it is always visible.
			m.items = msg.items
			m.total = msg.total
			perPage := m.paginator.PerPage
			if perPage <= 0 {
				perPage = 50
			}
			m.paginator.SetTotalPages((m.total + perPage - 1) / perPage)
			if len(m.items) == 0 {
				m.cursor = 0
			} else if m.cursor >= len(m.items) {
				m.cursor = len(m.items) - 1
			}
		}
		return m, nil

	case switchMsg:
		m.error = msg.err
		if msg.err == nil {
			return m, tea.Quit
		}
	}
	if m.mode == filterMode {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		if _, ok := msg.(tea.KeyMsg); ok {
			return m, tea.Batch(cmd, m.refreshList())
		}
		return m, cmd
	}
	return m, nil
}

func (m Model) View() string {
	var b strings.Builder
	if m.mode == filterMode {
		fmt.Fprintf(&b, "Filter: %s\n", m.input.View())
	} else {
		fmt.Fprintf(&b, "Select #: > %s\n", m.numberBuffer)
	}
	b.WriteString("\n")
	if m.error != nil {
		fmt.Fprintf(&b, "Error: %v\n\n", m.error)
	}

	// Cursor starts on first entry on a given page
	start := m.paginator.Page * m.paginator.PerPage
	for i, it := range m.items {
		prefix := "  "
		if i == m.cursor {
			prefix = "> " // > marks currently selected item
		}
		line := it.Name
		if it.IsCurrent {
			line = "* " + line // * marks current branch
		}
		// Numbered line items
		fmt.Fprintf(&b, "%s%3d. %s\n", prefix, start+i+1, line)
	}
	b.WriteString("\n")
	b.WriteString(m.paginator.View())
	b.WriteString("\n")
	if m.mode == filterMode {
		b.WriteString("esc/f: select • Tab: clear filter • ↑/k ↓/j: move • PgUp/PgDn, p/n, left/right: pages • Enter: switch • q: quit\n")
	} else {
		b.WriteString("f: filter • digits+Enter: select by number • Backspace: erase • ↑/k ↓/j: move (wrap) • PgUp/PgDn, p/n, left/right: pages • Enter: switch • q: quit\n")
	}
	return b.String()
}

// selectByNumber resolves an absolute selection number to a page and offset
// and attempts to switch to that branch. It fetches just the page that
// contains the requested index using the current filter and scope.
func (m Model) selectByNumber(n int) tea.Cmd {
	return func() tea.Msg {
		perPage := m.paginator.PerPage
		if perPage <= 0 {
			perPage = 50
		}
		if n <= 0 {
			return switchMsg{err: fmt.Errorf("invalid selection")}
		}
		idx := n - 1
		page := idx/perPage + 1
		offset := idx % perPage
		resp, err := core.ListBranches(core.ListBranchesRequest{
			RepoPath: m.RepoPath,
			Pattern:  strings.TrimSpace(m.input.Value()),
			Scope:    m.Scope,
			SortBy:   "recency",
			SortDir:  "desc",
			Page:     page,
			PageSize: perPage,
		})
		if err != nil {
			return switchMsg{err: err}
		}
		if offset < 0 || offset >= len(resp.Items) {
			return switchMsg{err: fmt.Errorf("selection out of range")}
		}
		name := resp.Items[offset].Name
		_, err = core.Checkout(m.RepoPath, name, false)
		return switchMsg{err: err}
	}
}
