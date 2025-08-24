// Package tui implements a small interactive terminal UI for browsing and
// switching git branches. It uses the Bubble Tea framework and exposes a
// Model type that implements the Bubble Tea Model interface. The UI
// supports filtering, pagination, and switching branches.
package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"gotobranch/internal/core"
)

var (
	titleStyle = func() lipgloss.Style {
		b := lipgloss.RoundedBorder()
		b.Right = "├"
		return lipgloss.NewStyle().BorderStyle(b).Padding(0, 1)
	}()

	infoStyle = func() lipgloss.Style {
		b := lipgloss.RoundedBorder()
		b.Left = "┤"
		return titleStyle.BorderStyle(b)
	}()
)

type Model struct {
	RepoPath string
	Scope    core.Scope

	input textinput.Model

	Items []core.Branch
	total int
	error error

	cursor int // index of cursor

	// mode controls the input semantics:
	// - selectMode: digits buffer + Enter selects a branch by number
	// - filterMode: type-to-filter using the text input
	mode         mode
	numberBuffer string

	ready    bool
	viewport viewport.Model

	Content string
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
	Items    []core.Branch
}

// New constructs a TUI Model configured with the provided options.
// - RepoPath: path to the git repository (empty = CWD)
// - Scope: which branches to include (local/remote/all)
// - Pattern: initial filter string
// - Items: the initial items to render
func New(opts Options) Model {
	inp := textinput.New()
	inp.Placeholder = "Filter pattern (press f to edit)"
	inp.SetValue(opts.Pattern)

	m := Model{
		RepoPath: opts.RepoPath,
		Scope:    opts.Scope,
		input:    inp,
		mode:     selectMode,
		Items:    opts.Items,
	}
	return m
}

// Init requests the first page of branches when the Bubble Tea
// program starts.
func (m Model) Init() tea.Cmd {
	return m.refreshList()
}

// refreshList returns a command which queries core.ListBranches for
// the CWD and filter. The command posts a listMsg with the items and total
// count which Update will apply to the model.
func (m Model) refreshList() tea.Cmd {
	return func() tea.Msg {
		resp, err := core.ListBranches(core.ListBranchesRequest{
			RepoPath: m.RepoPath,
			Pattern:  strings.TrimSpace(m.input.Value()),
			Scope:    m.Scope,
			SortBy:   "recency",
			SortDir:  "desc",
		})
		if err != nil {
			return listMsg{err: err}
		}
		return listMsg{items: resp.Items, total: resp.Total}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)
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

			case "f": // Switch to filter mode
				m.mode = filterMode
				m.input.Focus()
				if m.ready {
					m.viewport.SetContent(m.contentView())
				}
				return m, nil

			case "enter": // Select the chosen number
				if strings.TrimSpace(m.numberBuffer) != "" {
					n, err := strconv.Atoi(strings.TrimSpace(m.numberBuffer))
					m.numberBuffer = ""
					if err != nil || n <= 0 {
						m.error = fmt.Errorf("invalid selection")
						return m, nil
					}
					return m, m.selectByNumber(n)
				}
				if len(m.Items) == 0 {
					return m, nil
				}
				name := m.Items[m.cursor].Name
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
				if len(m.Items) == 0 {
					return m, nil
				}
				if m.cursor > 0 {
					m.cursor--
				} else {
					m.cursor = len(m.Items) - 1
				}
				return m, nil
			case "down", "j":
				if len(m.Items) == 0 {
					return m, nil
				}
				if m.cursor < len(m.Items)-1 {
					m.cursor++
				} else {
					m.cursor = 0
				}
				return m, nil

			case "pgup", "p", "left":
				// TODO - go to top of visible list
				return m, nil
			case "pgdn", "n", "right":
				// TODO - go to bottom of visible list
				return m, m.refreshList()

			case "tab": // Clear numeric buffer
				m.numberBuffer = ""
				return m, nil

			default:
				// TODO - show message if user types invalid key
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

			case "esc": // Return to select mode
				m.mode = selectMode
				m.input.Blur()
				if m.ready {
					m.viewport.SetContent(m.contentView())
				}
				return m, nil

			case "tab": // Clear input value
				m.input.SetValue("")
				return m, m.refreshList()

			case "pgup", "p", "right":
				// TODO - go to bottom of visible list
				return m, nil
			case "pgdn", "n", "left":
				// TODO - go to top of visible list
				return m, m.refreshList()
			}
		}

	case tea.WindowSizeMsg:
		headerHeight := lipgloss.Height(m.headerView())
		footerHeight := lipgloss.Height(m.footerView())
		verticalMarginHeight := headerHeight + footerHeight

		if !m.ready {
			// Wait until we've dimensions are received before initializing viewport
			m.viewport = viewport.New(msg.Width, msg.Height-verticalMarginHeight)
			m.viewport.YPosition = headerHeight
			m.viewport.SetContent(m.contentView())
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - verticalMarginHeight
		}

		// Handle keyboard and mouse events in the viewport
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)

		return m, tea.Batch(cmds...)

	// listMsg tells the model to update the list of items
	case listMsg:
		m.error = msg.err
		if msg.err == nil {
			m.Items = msg.items
			m.total = msg.total
		}
		return m, nil

	case switchMsg:
		m.error = msg.err
		if msg.err == nil {
			return m, tea.Quit
		}
	}

	// Handle text input updates in filter mode
	if m.mode == filterMode {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		if _, ok := msg.(tea.KeyMsg); ok {
			if m.ready {
				m.viewport.SetContent(m.contentView())
			}
			return m, tea.Batch(cmd, m.refreshList())
		}
		return m, cmd
	}
	return m, nil
}

func (m Model) View() string {
	if !m.ready {
		return "\n  Initializing..."
	}
	m.viewport.SetContent(m.contentView())
	return fmt.Sprintf("%s\n%s\n%s", m.headerView(), m.viewport.View(), m.footerView())
}

func (m Model) headerView() string {
	title := titleStyle.Render("Enter a number to go to that branch.\nf: filter mode\n?: more help")
	line := strings.Repeat("─", max(0, m.viewport.Width-lipgloss.Width(title)))
	return lipgloss.JoinHorizontal(lipgloss.Center, title, line)
}

func (m Model) footerView() string {
	info := infoStyle.Render(fmt.Sprintf("%3.f%%", m.viewport.ScrollPercent()*100))
	line := strings.Repeat("─", max(0, m.viewport.Width-lipgloss.Width(info)))
	return lipgloss.JoinHorizontal(lipgloss.Center, line, info)
}

func (m Model) contentView() string {
	content := m.Content
	if m.mode == filterMode {
		content = fmt.Sprintf("Filter: %s\n\n", m.input.View()) + content
	} else {
		content = fmt.Sprintf("Select #: > %s\n", m.numberBuffer) + content
	}
	if m.error != nil {
		content += fmt.Sprintf("Error: %v\n\n", m.error)
	}

	start := 0
	for i, item := range m.Items {
		prefix := "  "
		if i == m.cursor {
			prefix = "> " // > marks currently selected item
		}
		line := item.Name
		if item.IsCurrent {
			line = "* " + line // * marks current branch
		}
		// Numbered line items
		content += fmt.Sprintf("%s%3d. %s\n", prefix, start+i+1, line)
	}
	return content
}

// selectByNumber accepts a selection a line item and attempts to switch to
// that branch.
func (m Model) selectByNumber(n int) tea.Cmd {
	return func() tea.Msg {
		if n <= 0 {
			return switchMsg{err: fmt.Errorf("invalid selection")}
		}
		idx := n - 1
		resp, err := core.ListBranches(core.ListBranchesRequest{
			RepoPath: m.RepoPath,
			Pattern:  strings.TrimSpace(m.input.Value()),
			Scope:    m.Scope,
			SortBy:   "recency",
			SortDir:  "desc",
		})
		if err != nil {
			return switchMsg{err: err}
		}
		name := resp.Items[idx].Name
		_, err = core.Checkout(m.RepoPath, name, false)
		return switchMsg{err: err}
	}
}
