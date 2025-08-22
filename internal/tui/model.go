package tui

import (
	"fmt"
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
}

type listMsg struct{}

type switchMsg struct{ err error }

type Options struct {
	RepoPath string
	Scope    core.Scope
	PageSize int
	Pattern  string
}

func New(opts Options) Model {
	inp := textinput.New()
	inp.Placeholder = "Filter pattern (type to filter)"
	inp.SetValue(opts.Pattern)
	inp.Focus()

	p := paginator.New()
	if opts.PageSize <= 0 {
		opts.PageSize = 50
	}
	p.PerPage = opts.PageSize

	m := Model{
		RepoPath:  opts.RepoPath,
		Scope:     opts.Scope,
		input:     inp,
		paginator: p,
	}
	return m
}

func (m Model) Init() tea.Cmd {
	return m.refreshList()
}

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
			m.error = err
			return listMsg{}
		}
		m.items = resp.Items
		m.total = resp.Total
		m.paginator.SetTotalPages((resp.Total + m.paginator.PerPage - 1) / m.paginator.PerPage)
		return listMsg{}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "enter":
			// Switch to highlighted item (top of current page)
			idx := m.cursor
			if len(m.items) == 0 {
				return m, nil
			}
			name := m.items[idx].Name
			return m, func() tea.Msg {
				_, err := core.Checkout(m.RepoPath, name, false)
				return switchMsg{err: err}
			}
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil
		case "down", "j":
			if m.cursor < len(m.items)-1 {
				m.cursor++
			}
			return m, nil
		case "tab":
			// Clear pattern
			m.input.SetValue("")
			return m, m.refreshList()
		case "pgup", "left", "h":
			if m.paginator.Page > 0 {
				m.paginator.PrevPage()
				m.cursor = 0
				return m, m.refreshList()
			}
		case "pgdn", "right", "l":
			m.paginator.NextPage()
			m.cursor = 0
			return m, m.refreshList()
		}
	case listMsg:
		// refreshed
	case switchMsg:
		m.error = msg.err
		if msg.err == nil {
			return m, tea.Quit
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	if _, ok := msg.(tea.KeyMsg); ok {
		return m, tea.Batch(cmd, m.refreshList())
	}
	return m, cmd
}

func (m Model) View() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Filter: %s\n", m.input.View())
	b.WriteString("\n")
	if m.error != nil {
		fmt.Fprintf(&b, "Error: %v\n\n", m.error)
	}
	start := m.paginator.Page * m.paginator.PerPage
	for i, it := range m.items {
		prefix := "  "
		if i == m.cursor {
			prefix = "> "
		}
		line := it.Name
		if it.IsCurrent {
			line = "* " + line
		}
		fmt.Fprintf(&b, "%s%3d. %s\n", prefix, start+i+1, line)
	}
	b.WriteString("\n")
	b.WriteString(m.paginator.View())
	b.WriteString("\n")
	b.WriteString("↑/k ↓/j: move • Enter: switch • Tab: clear • PgUp/PgDn or h/l: pages • q: quit\n")
	return b.String()
}
