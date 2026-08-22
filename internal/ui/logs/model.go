package logs

import (
	"fmt"
	"strings"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/Laaaaksh/vessel/internal/ui/uiutil"
)

// Model is a full-screen log viewer.
type Model struct {
	title     string
	lines     []string
	width     int
	height    int
	offset    int // scroll from bottom: 0 = follow tail
	errText   string
	following bool
	searching bool
	query     string
	matchIdx  int
	matches   []int
}

// New creates an empty logs model.
func New() Model { return Model{following: true} }

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
	m.following = true
	m.searching = false
	m.query = ""
	m.matches = nil
	m.matchIdx = 0
	return m
}

// Append adds streamed lines.
func (m Model) Append(line string) Model {
	if !m.following {
		// Keep relative place when frozen: bump offset so view stays put.
		m.offset++
	}
	m.lines = append(m.lines, line)
	if len(m.lines) > 5000 {
		trim := len(m.lines) - 4000
		m.lines = m.lines[trim:]
		m.offset = max(0, m.offset-trim)
		m.recomputeMatches()
	}
	return m
}

// SetError records an error string.
func (m Model) SetError(err string) Model {
	m.errText = err
	return m
}

// Following reports follow mode.
func (m Model) Following() bool { return m.following }

// SelectedLine returns the bottom-most visible line (for yank).
func (m Model) SelectedLine() string {
	vis := m.visibleLines()
	if len(vis) == 0 {
		return ""
	}
	return strings.TrimSpace(vis[len(vis)-1])
}

// Update handles scroll / search / follow keys.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	kp, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	k := kp.String()

	if m.searching {
		switch k {
		case "enter", "esc":
			m.searching = false
			if k == "esc" {
				m.query = ""
				m.matches = nil
			} else {
				m.recomputeMatches()
				m.jumpMatch(1)
			}
		case "backspace":
			if m.query != "" {
				_, size := utf8.DecodeLastRuneInString(m.query)
				m.query = m.query[:len(m.query)-size]
			}
		default:
			// Bubble Tea reports a space press as "space", and Key.String()
			// is byte-lengthed, so accept one full rune of text either way.
			if k == "space" {
				k = " "
			} else if utf8.RuneCountInString(k) != 1 {
				return m, nil
			}
			m.query += k
		}
		return m, nil
	}

	switch k {
	case "j", "down":
		if m.offset > 0 {
			m.offset--
		}
		m.following = m.offset == 0
	case "k", "up":
		m.offset = min(m.offset+1, m.maxOffset())
		m.following = false
	case "pgdown", "ctrl+d":
		m.offset = max(0, m.offset-m.bodyHeight()/2)
		m.following = m.offset == 0
	case "pgup", "ctrl+u":
		m.offset = min(m.offset+m.bodyHeight()/2, m.maxOffset())
		m.following = false
	case "g":
		m.offset = m.maxOffset()
		m.following = false
	case "G":
		m.offset = 0
		m.following = true
	case "f":
		m.following = !m.following
		if m.following {
			m.offset = 0
		}
	case "/":
		m.searching = true
		m.query = ""
	case "n":
		m.jumpMatch(1)
	case "N":
		m.jumpMatch(-1)
	}
	return m, nil
}

func (m *Model) recomputeMatches() {
	m.matches = nil
	if m.query == "" {
		return
	}
	q := strings.ToLower(m.query)
	for i, line := range m.lines {
		if strings.Contains(strings.ToLower(line), q) {
			m.matches = append(m.matches, i)
		}
	}
	m.matchIdx = 0
}

func (m *Model) jumpMatch(dir int) {
	if len(m.matches) == 0 {
		return
	}
	m.matchIdx = (m.matchIdx + dir) % len(m.matches)
	if m.matchIdx < 0 {
		m.matchIdx = len(m.matches) - 1
	}
	// Convert line index to offset-from-bottom.
	line := m.matches[m.matchIdx]
	m.offset = max(0, len(m.lines)-1-line)
	m.following = false
}

func (m Model) maxOffset() int {
	return max(0, len(m.lines)-m.bodyHeight())
}

func (m Model) bodyHeight() int { return max(1, m.height-3) }

// View renders the log panel.
func (m Model) View() string {
	follow := "FOLLOW"
	if !m.following {
		follow = "FROZEN"
	}
	title := lipgloss.NewStyle().Foreground(lipgloss.Color("#a78bfa")).Bold(true).
		Render("logs · " + m.title + " · " + follow)
	hint := lipgloss.NewStyle().Foreground(lipgloss.Color("#6b7280")).
		Render("  [esc] back  [f] follow  [/] search  [n/N] next  [y] yank")
	header := lipgloss.JoinHorizontal(lipgloss.Top, title, hint)

	bodyH := m.bodyHeight()
	body := lipgloss.NewStyle().Foreground(lipgloss.Color("#e2e8f0")).
		Width(m.width).Height(bodyH).
		Render(strings.Join(m.visibleLines(), "\n"))

	footer := ""
	switch {
	case m.searching:
		footer = lipgloss.NewStyle().Foreground(lipgloss.Color("#60a5fa")).
			Width(m.width).Render("search: " + m.query + "_")
	case m.query != "":
		footer = lipgloss.NewStyle().Foreground(lipgloss.Color("#6b7280")).
			Width(m.width).Render(fmtMatches(len(m.matches), m.matchIdx, m.query))
	case m.errText != "":
		footer = lipgloss.NewStyle().Foreground(lipgloss.Color("#f87171")).
			Width(m.width).Render("error: " + m.errText)
	}

	return lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
}

func fmtMatches(n, idx int, q string) string {
	if n == 0 {
		return "no matches for " + q
	}
	return fmt.Sprintf("match %d/%d for %s", idx+1, n, q)
}

func (m Model) visibleLines() []string {
	if len(m.lines) == 0 {
		return []string{"  (no log output)"}
	}
	end := len(m.lines) - min(m.offset, m.maxOffset())
	start := max(0, end-m.bodyHeight())
	out := make([]string, 0, end-start)
	q := strings.ToLower(m.query)
	for _, line := range m.lines[start:end] {
		display := line
		if m.width > 4 {
			display = uiutil.Truncate(display, m.width-2)
		}
		if q != "" && strings.Contains(strings.ToLower(line), q) {
			display = lipgloss.NewStyle().Foreground(lipgloss.Color("#fbbf24")).Render(display)
		}
		out = append(out, "  "+display)
	}
	return out
}
