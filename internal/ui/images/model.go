package images

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/Laaaaksh/vessel/internal/backend"
	"github.com/Laaaaksh/vessel/internal/ui/uiutil"
)

// Model is the images panel.
type Model struct {
	items  []backend.Image
	cursor int
}

// New creates an empty images model.
func New() Model { return Model{} }

// SetItems replaces the image list.
func (m Model) SetItems(items []backend.Image) Model {
	m.items = items
	if m.cursor >= len(m.items) {
		m.cursor = max(0, len(m.items)-1)
	}
	return m
}

// Selected returns the highlighted image.
func (m Model) Selected() *backend.Image {
	if len(m.items) == 0 {
		return nil
	}
	img := m.items[m.cursor]
	return &img
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

// ListView renders the image list.
func (m Model) ListView(width, height int) string {
	header := lipgloss.NewStyle().Foreground(lipgloss.Color("#6b7280")).
		BorderBottom(true).BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("#374151")).Width(width).
		Render(fmt.Sprintf("%-40s %-12s %s", "REPOSITORY:TAG", "SIZE", "CREATED"))

	sel := lipgloss.NewStyle().Background(lipgloss.Color("#2d1b69")).Foreground(lipgloss.Color("#c4b5fd"))
	row := lipgloss.NewStyle().Foreground(lipgloss.Color("#e2e8f0"))

	var rows []string
	for i, img := range m.items {
		line := fmt.Sprintf("%-40s %-12s %s",
			uiutil.Truncate(backend.FormatRef(img), 40),
			uiutil.HumanBytes(img.Size),
			uiutil.Ago(img.Created),
		)
		st := row
		if i == m.cursor {
			st = sel
		}
		rows = append(rows, st.Width(width).Render(line))
	}
	if len(rows) == 0 {
		rows = append(rows, lipgloss.NewStyle().Foreground(lipgloss.Color("#6b7280")).Render("  no images"))
	}
	return lipgloss.NewStyle().Width(width).Height(height).
		Render(lipgloss.JoinVertical(lipgloss.Left, append([]string{header}, rows...)...))
}

// DetailView renders image details.
func (m Model) DetailView(width, height int) string {
	sel := m.Selected()
	if sel == nil {
		return lipgloss.NewStyle().Width(width).Height(height).
			Foreground(lipgloss.Color("#6b7280")).Render("  no image selected")
	}
	lines := []string{
		lipgloss.NewStyle().Foreground(lipgloss.Color("#a78bfa")).Bold(true).Render(backend.FormatRef(*sel)),
		"",
		uiutil.KV("ID", uiutil.Truncate(sel.ID, 16)),
		uiutil.KV("Size", uiutil.HumanBytes(sel.Size)),
		uiutil.KV("Created", uiutil.Ago(sel.Created)),
		"",
		lipgloss.NewStyle().Foreground(lipgloss.Color("#6b7280")).Render("[d] delete image"),
	}
	return lipgloss.NewStyle().Width(width).Height(height).PaddingLeft(1).
		Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}
