// Package networks is the read-only network panel: list and inspect only.
// Create/delete/prune are deliberately absent — see the project brief this
// package was built against.
package networks

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/Laaaaksh/vessel/internal/backend"
	"github.com/Laaaaksh/vessel/internal/ui/uiutil"
)

const (
	colName = 20
	colMode = 10
)

// Model is the networks panel.
type Model struct {
	items     []backend.Network
	filtered  []backend.Network
	cursor    int
	filter    string
	filtering bool
	pageRows  int
}

// New creates an empty networks model.
func New() Model {
	return Model{pageRows: 10}
}

// Filtering reports whether the filter prompt is active.
func (m Model) Filtering() bool { return m.filtering }

// Filter returns the filter string.
func (m Model) Filter() string { return m.filter }

// Cursor returns the highlighted row index, for the footer.
func (m Model) Cursor() int { return m.cursor }

// Len returns the number of visible rows, for the footer.
func (m Model) Len() int { return len(m.filtered) }

// SetPageRows sets page scroll size.
func (m Model) SetPageRows(n int) Model {
	if n > 0 {
		m.pageRows = n
	}
	return m
}

// SetItems replaces the network list.
func (m Model) SetItems(items []backend.Network) Model {
	m.items = items
	m.filtered = applyFilter(items, m.filter)
	if m.cursor >= len(m.filtered) {
		m.cursor = max(0, len(m.filtered)-1)
	}
	return m
}

// Selected returns the highlighted network.
func (m Model) Selected() *backend.Network {
	if len(m.filtered) == 0 {
		return nil
	}
	n := m.filtered[m.cursor]
	return &n
}

// MoveBy moves the cursor by delta rows.
func (m Model) MoveBy(delta int) Model {
	m.cursor = uiutil.MoveCursor(m.cursor, len(m.filtered), delta)
	return m
}

// SetCursor moves the cursor to row i, clamped to the visible rows.
func (m Model) SetCursor(i int) Model {
	m.cursor = uiutil.MoveCursor(i, len(m.filtered), 0)
	return m
}

// Update handles navigation and filter.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	kp, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	k := kp.String()
	if m.filtering {
		switch k {
		case "enter", "esc":
			m.filtering = false
		case "backspace":
			if len(m.filter) > 0 {
				m.filter = m.filter[:len(m.filter)-1]
				m.filtered = applyFilter(m.items, m.filter)
				m.cursor = 0
			}
		default:
			if len(k) == 1 {
				m.filter += k
				m.filtered = applyFilter(m.items, m.filter)
				m.cursor = 0
			}
		}
		return m, nil
	}
	switch k {
	case "j", "down":
		m.cursor = uiutil.MoveCursor(m.cursor, len(m.filtered), 1)
	case "k", "up":
		m.cursor = uiutil.MoveCursor(m.cursor, len(m.filtered), -1)
	case "pgdown", "ctrl+d":
		m.cursor = uiutil.MoveCursor(m.cursor, len(m.filtered), uiutil.PageDelta(m.pageRows, k == "ctrl+d", true))
	case "pgup", "ctrl+u":
		m.cursor = uiutil.MoveCursor(m.cursor, len(m.filtered), uiutil.PageDelta(m.pageRows, k == "ctrl+u", false))
	case "g":
		m.cursor = 0
	case "G":
		m.cursor = max(0, len(m.filtered)-1)
	case "/":
		m.filtering = true
		m.filter = ""
		m.filtered = m.items
		m.cursor = 0
	case "esc":
		if m.filter != "" {
			m.filter = ""
			m.filtered = m.items
			m.cursor = 0
		}
	}
	return m, nil
}

func applyFilter(items []backend.Network, filter string) []backend.Network {
	if filter == "" {
		return items
	}
	f := strings.ToLower(filter)
	var out []backend.Network
	for _, n := range items {
		if strings.Contains(strings.ToLower(n.Name), f) || strings.Contains(strings.ToLower(n.Mode), f) {
			out = append(out, n)
		}
	}
	return out
}

// ListView renders the network list.
func (m Model) ListView(width, height int) string {
	header := lipgloss.NewStyle().Foreground(lipgloss.Color("#6b7280")).
		BorderBottom(true).BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("#374151")).Width(width).
		Render(fmt.Sprintf("%-*s %-*s %s", colName, "NAME", colMode, "MODE", "CREATED"))

	sel := lipgloss.NewStyle().Background(lipgloss.Color("#2d1b69")).Foreground(lipgloss.Color("#c4b5fd"))
	row := lipgloss.NewStyle().Foreground(lipgloss.Color("#e2e8f0"))

	var filterBar string
	if m.filtering {
		filterBar = lipgloss.NewStyle().Foreground(lipgloss.Color("#60a5fa")).
			Render(fmt.Sprintf("  filter: %s_  (%d/%d)", m.filter, len(m.filtered), len(m.items)))
	} else if m.filter != "" {
		filterBar = lipgloss.NewStyle().Foreground(lipgloss.Color("#6b7280")).
			Render(fmt.Sprintf("  filter: %s  (%d/%d)", m.filter, len(m.filtered), len(m.items)))
	}

	rowsH := height - 2
	if filterBar != "" {
		rowsH--
	}
	start, end := uiutil.Window(len(m.filtered), m.cursor, max(1, rowsH))

	var rows []string
	for i := start; i < end; i++ {
		n := m.filtered[i]
		line := fmt.Sprintf(" %s %-*s %s", uiutil.Pad(n.Name, colName-1), colMode, n.Mode, uiutil.Ago(n.Created))
		st := row
		if i == m.cursor {
			st = sel
		}
		rows = append(rows, st.Width(width).Render(line))
	}
	if len(rows) == 0 {
		msg := "  no networks"
		if m.filter != "" {
			msg = "  no matches"
		}
		rows = append(rows, lipgloss.NewStyle().Foreground(lipgloss.Color("#6b7280")).Render(msg))
	}
	content := lipgloss.JoinVertical(lipgloss.Left, append([]string{header}, rows...)...)
	if filterBar != "" {
		content = lipgloss.JoinVertical(lipgloss.Left, content, filterBar)
	}
	return lipgloss.NewStyle().Width(width).Height(height).Render(content)
}

// DetailView renders network details. Name is rendered first so it survives
// the panel's habit of dropping trailing rows at short heights.
func (m Model) DetailView(width, height int) string {
	sel := m.Selected()
	if sel == nil {
		return lipgloss.NewStyle().Width(width).Height(height).
			Foreground(lipgloss.Color("#6b7280")).Render("  no network selected")
	}
	lines := []string{
		lipgloss.NewStyle().Foreground(lipgloss.Color("#a78bfa")).Bold(true).Render(sel.Name),
		uiutil.KV("Mode", sel.Mode),
		uiutil.KV("Gateway", sel.Gateway),
		uiutil.KV("Subnet", sel.Subnet),
		uiutil.KV("Created", uiutil.Ago(sel.Created)),
	}
	return lipgloss.NewStyle().Width(width).Height(height).PaddingLeft(1).
		Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}
