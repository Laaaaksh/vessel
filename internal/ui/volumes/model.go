package volumes

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/Laaaaksh/vessel/internal/backend"
	"github.com/Laaaaksh/vessel/internal/ui/uiutil"
)

// Model is the volumes panel.
type Model struct {
	items  []backend.Volume
	cursor int
}

// New creates an empty volumes model.
func New() Model { return Model{} }

// SetItems replaces the volume list.
func (m Model) SetItems(items []backend.Volume) Model {
	m.items = items
	if m.cursor >= len(m.items) {
		m.cursor = max(0, len(m.items)-1)
	}
	return m
}

// Selected returns the highlighted volume.
func (m Model) Selected() *backend.Volume {
	if len(m.items) == 0 {
		return nil
	}
	v := m.items[m.cursor]
	return &v
}

// Update handles navigation.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	kp, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	switch kp.String() {
	case "j", "down":
		if m.cursor < len(m.items)-1 {
			m.cursor++
		}
	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
		}
	case "g":
		m.cursor = 0
	case "G":
		m.cursor = max(0, len(m.items)-1)
	}
	return m, nil
}

// ListView renders the volume list.
func (m Model) ListView(width, height int) string {
	header := lipgloss.NewStyle().Foreground(lipgloss.Color("#6b7280")).
		BorderBottom(true).BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("#374151")).Width(width).
		Render(fmt.Sprintf("%-28s %-10s %s", "NAME", "DRIVER", "CREATED"))

	sel := lipgloss.NewStyle().Background(lipgloss.Color("#2d1b69")).Foreground(lipgloss.Color("#c4b5fd"))
	row := lipgloss.NewStyle().Foreground(lipgloss.Color("#e2e8f0"))

	// The header renders as two lines (text plus its bottom border).
	start, end := uiutil.Window(len(m.items), m.cursor, max(1, height-2))

	var rows []string
	for i := start; i < end; i++ {
		v := m.items[i]
		line := fmt.Sprintf("%-28s %-10s %s", uiutil.Truncate(v.Name, 28), v.Driver, uiutil.Ago(v.Created))
		st := row
		if i == m.cursor {
			st = sel
		}
		rows = append(rows, st.Width(width).Render(line))
	}
	if len(rows) == 0 {
		rows = append(rows, lipgloss.NewStyle().Foreground(lipgloss.Color("#6b7280")).Render("  no volumes"))
	}
	return lipgloss.NewStyle().Width(width).Height(height).
		Render(lipgloss.JoinVertical(lipgloss.Left, append([]string{header}, rows...)...))
}

// DetailView renders volume details.
func (m Model) DetailView(width, height int) string {
	sel := m.Selected()
	if sel == nil {
		return lipgloss.NewStyle().Width(width).Height(height).
			Foreground(lipgloss.Color("#6b7280")).Render("  no volume selected")
	}
	lines := []string{
		lipgloss.NewStyle().Foreground(lipgloss.Color("#a78bfa")).Bold(true).Render(sel.Name),
		"",
		uiutil.KV("Driver", sel.Driver),
		uiutil.KV("Created", uiutil.Ago(sel.Created)),
		uiutil.KV("Path", uiutil.Truncate(sel.Mountpoint, width-12)),
		"",
		lipgloss.NewStyle().Foreground(lipgloss.Color("#6b7280")).Render("[d] delete volume"),
	}
	return lipgloss.NewStyle().Width(width).Height(height).PaddingLeft(1).
		Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}
