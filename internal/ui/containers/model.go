package containers

import (
	"strings"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/Laaaaksh/vessel/internal/backend"
	"github.com/Laaaaksh/vessel/internal/ui/uiutil"
)

// Model is the containers panel model.
type Model struct {
	items      []backend.Container
	filtered   []backend.Container
	cursor     int
	filter     string
	filtering  bool
	marked     map[string]bool
	toggleMark string
	pageRows   int

	styleSelected lipgloss.Style
	styleRow      lipgloss.Style
	styleRunning  lipgloss.Style
	styleExited   lipgloss.Style
}

// New creates the containers Model with default styles.
func New() Model {
	return Model{
		marked:        make(map[string]bool),
		toggleMark:    defaultToggleMark,
		pageRows:      10,
		styleSelected: lipgloss.NewStyle().Background(lipgloss.Color("#2d1b69")).Foreground(lipgloss.Color("#c4b5fd")),
		styleRow:      lipgloss.NewStyle().Foreground(lipgloss.Color("#e2e8f0")),
		styleRunning:  lipgloss.NewStyle().Foreground(lipgloss.Color("#34d399")),
		styleExited:   lipgloss.NewStyle().Foreground(lipgloss.Color("#6b7280")),
	}
}

// Filtering reports whether the filter prompt is active.
func (m Model) Filtering() bool { return m.filtering }

// Filter returns the active filter string.
func (m Model) Filter() string { return m.filter }

// Cursor returns the cursor index in the filtered list.
func (m Model) Cursor() int { return m.cursor }

// Len returns filtered length.
func (m Model) Len() int { return len(m.filtered) }

// SetPageRows sets the page-scroll window hint.
func (m Model) SetPageRows(n int) Model {
	if n > 0 {
		m.pageRows = n
	}
	return m
}

// SetItems replaces the container list, reapplies the current filter, and drops
// marks for containers it no longer contains, so a mark can never outlive the
// row it points at.
func (m Model) SetItems(items []backend.Container) Model {
	m.items = items
	m.filtered = applyFilter(items, m.filter)
	marked := make(map[string]bool, len(m.marked))
	for _, c := range items {
		if m.marked[c.ID] {
			marked[c.ID] = true
		}
	}
	m.marked = marked
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

// defaultToggleMark is the fallback binding for a panel the app has not handed
// its key map to; a real space bar press serialises as "space", never " ".
const defaultToggleMark = "space"

// SetToggleMarkKey sets the key that toggles a mark on the selected row. An
// empty binding is ignored so the panel can never end up unmarkable.
func (m Model) SetToggleMarkKey(k string) Model {
	if k != "" {
		m.toggleMark = k
	}
	return m
}

// MarkedIDs returns multi-selected container IDs.
func (m Model) MarkedIDs() []string {
	var out []string
	for _, c := range m.filtered {
		if m.marked[c.ID] {
			out = append(out, c.ID)
		}
	}
	return out
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

// Update handles keyboard events for the containers panel (navigation/filter only).
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	kp, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	return m.handleKey(kp)
}

// MoveBy moves the cursor by delta (used by mouse wheel / page keys from root).
func (m Model) MoveBy(delta int) Model {
	m.cursor = uiutil.MoveCursor(m.cursor, len(m.filtered), delta)
	return m
}

// SetCursor sets an absolute cursor.
func (m Model) SetCursor(i int) Model {
	m.cursor = uiutil.MoveCursor(i, len(m.filtered), 0)
	return m
}

func (m Model) handleKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	k := msg.String()

	if m.filtering {
		switch k {
		case "enter", "esc":
			m.filtering = false
		case "backspace":
			if m.filter != "" {
				_, size := utf8.DecodeLastRuneInString(m.filter)
				m.filter = m.filter[:len(m.filter)-size]
				m.filtered = applyFilter(m.items, m.filter)
				m.cursor = 0
			}
		default:
			// Bubble Tea reports a space press as "space", and Key.String()
			// is byte-lengthed, so accept one full rune of text either way.
			if k == "space" {
				k = " "
			} else if utf8.RuneCountInString(k) != 1 {
				return m, nil
			}
			m.filter += k
			m.filtered = applyFilter(m.items, m.filter)
			m.cursor = 0
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
	case m.toggleMark:
		if sel := m.Selected(); sel != nil {
			if m.marked == nil {
				m.marked = make(map[string]bool)
			}
			if m.marked[sel.ID] {
				delete(m.marked, sel.ID)
			} else {
				m.marked[sel.ID] = true
			}
		}
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
