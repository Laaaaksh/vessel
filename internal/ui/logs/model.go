package logs

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/Laaaaksh/vessel/internal/ui/uiutil"
)

// Model is a full-screen log viewer.
type Model struct {
	title   string
	lines   []string
	width   int
	height  int
	offset  int // scroll from bottom: 0 = follow tail
	errText string
}

// New creates an empty logs model.
func New() Model { return Model{} }

// SetSize updates viewport dimensions.
func (m Model) SetSize(w, h int) Model {
	m.width = w
	m.height = h
	return m
}

// Open prepares the viewer for a container.
func (m Model) Open(title string, initial []string) Model {
	m.title = title
	m.lines = append([]string{}, initial...)
	m.offset = 0
	m.errText = ""
	return m
}

// Append adds streamed lines.
func (m Model) Append(line string) Model {
	m.lines = append(m.lines, line)
	// Cap memory
	if len(m.lines) > 5000 {
		m.lines = m.lines[len(m.lines)-4000:]
	}
	return m
}

// SetError records an error string.
func (m Model) SetError(err string) Model {
	m.errText = err
	return m
}

// Update handles scroll keys.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	kp, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	switch kp.String() {
	case "j", "down":
		if m.offset > 0 {
			m.offset--
		}
	case "k", "up":
		m.offset = min(m.offset+1, m.maxOffset())
	case "g":
		m.offset = m.maxOffset()
	case "G":
		m.offset = 0
	}
	return m, nil
}

// maxOffset is the scrollback position that puts the first line at the top of
// the viewport; scrolling further would shrink the visible window.
func (m Model) maxOffset() int {
	return max(0, len(m.lines)-m.bodyHeight())
}

func (m Model) bodyHeight() int { return max(1, m.height-3) }

// View renders the log panel.
func (m Model) View() string {
	title := lipgloss.NewStyle().Foreground(lipgloss.Color("#a78bfa")).Bold(true).
		Render("logs · " + m.title)
	hint := lipgloss.NewStyle().Foreground(lipgloss.Color("#6b7280")).
		Render("  [esc] back  [j/k] scroll  [G] tail")
	header := lipgloss.JoinHorizontal(lipgloss.Top, title, hint)

	bodyH := m.bodyHeight()
	visible := m.visibleLines(bodyH)
	body := lipgloss.NewStyle().Foreground(lipgloss.Color("#e2e8f0")).
		Width(m.width).Height(bodyH).
		Render(strings.Join(visible, "\n"))

	footer := ""
	if m.errText != "" {
		footer = lipgloss.NewStyle().Foreground(lipgloss.Color("#f87171")).
			Width(m.width).Render("error: " + m.errText)
	}

	return lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
}

func (m Model) visibleLines(n int) []string {
	if len(m.lines) == 0 {
		return []string{"  (no log output)"}
	}
	end := len(m.lines) - m.offset
	if end < 1 {
		end = 1
	}
	start := end - n
	if start < 0 {
		start = 0
	}
	out := make([]string, 0, end-start)
	for _, line := range m.lines[start:end] {
		if m.width > 4 {
			line = uiutil.Truncate(line, m.width-2)
		}
		out = append(out, "  "+line)
	}
	return out
}
