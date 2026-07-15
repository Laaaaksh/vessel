package containers

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/Laaaaksh/vessel/internal/backend"
)

// Model is the containers panel model.
type Model struct {
	items    []backend.Container
	filtered []backend.Container
	cursor   int
	filter   string
	filtering bool

	styleSelected lipgloss.Style
	styleRow      lipgloss.Style
	styleRunning  lipgloss.Style
	styleExited   lipgloss.Style
}

// New creates the containers Model with default styles.
func New() Model {
	return Model{
		styleSelected: lipgloss.NewStyle().Background(lipgloss.Color("#2d1b69")).Foreground(lipgloss.Color("#c4b5fd")),
		styleRow:      lipgloss.NewStyle().Foreground(lipgloss.Color("#e2e8f0")),
		styleRunning:  lipgloss.NewStyle().Foreground(lipgloss.Color("#34d399")),
		styleExited:   lipgloss.NewStyle().Foreground(lipgloss.Color("#6b7280")),
	}
}

// SetItems replaces the container list and reapplies the current filter.
func (m Model) SetItems(items []backend.Container) Model {
	m.items = items
	m.filtered = applyFilter(items, m.filter)
	if m.cursor >= len(m.filtered) {
		m.cursor = max(0, len(m.filtered)-1)
	}
	return m
}

// Selected returns the currently highlighted container, if any.
func (m Model) Selected() *backend.Container {
	if len(m.filtered) == 0 {
		return nil
	}
	c := m.filtered[m.cursor]
	return &c
}

// RunningCount returns how many containers are running.
func (m Model) RunningCount() int {
	n := 0
	for _, c := range m.items {
		if c.IsRunning() {
			n++
		}
	}
	return n
}

// StoppedCount returns how many containers are not running.
func (m Model) StoppedCount() int {
	return len(m.items) - m.RunningCount()
}

// Update handles keyboard events for the containers panel.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	kp, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	return m.handleKey(kp)
}

func (m Model) handleKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	k := msg.String()

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
		if m.cursor < len(m.filtered)-1 {
			m.cursor++
		}
	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
		}
	case "g":
		m.cursor = 0
	case "G":
		m.cursor = max(0, len(m.filtered)-1)
	case "/":
		m.filtering = true
		m.filter = ""
		m.filtered = m.items
		m.cursor = 0
	}
	return m, nil
}

func applyFilter(items []backend.Container, filter string) []backend.Container {
	if filter == "" {
		return items
	}
	f := strings.ToLower(filter)
	var out []backend.Container
	for _, c := range items {
		if strings.Contains(strings.ToLower(c.Name), f) ||
			strings.Contains(strings.ToLower(c.Image), f) {
			out = append(out, c)
		}
	}
	return out
}

