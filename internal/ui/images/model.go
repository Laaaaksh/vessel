package images

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/Laaaaksh/vessel/internal/backend"
	"github.com/Laaaaksh/vessel/internal/ui/uiutil"
)

const (
	colRef  = 40
	colSize = 12
)

// Model is the images panel.
type Model struct {
	items     []backend.Image
	filtered  []backend.Image
	cursor    int
	filter    string
	filtering bool
	marked    map[string]bool
	pageRows  int
}

// New creates an empty images model.
func New() Model {
	return Model{marked: make(map[string]bool), pageRows: 10}
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

// markKey identifies a row for multi-select. Two references can resolve to the
// same digest, so the id alone would key both rows to a single mark.
func markKey(img backend.Image) string {
	return img.ID + "\x00" + backend.FormatRef(img)
}

// SetItems replaces the image list and drops marks for images it no longer
// contains, so a mark can never outlive the row it points at.
func (m Model) SetItems(items []backend.Image) Model {
	m.items = items
	m.filtered = applyFilter(items, m.filter)
	marked := make(map[string]bool, len(m.marked))
	for _, img := range items {
		if k := markKey(img); m.marked[k] {
			marked[k] = true
		}
	}
	m.marked = marked
	if m.cursor >= len(m.filtered) {
		m.cursor = max(0, len(m.filtered)-1)
	}
	return m
}

// Selected returns the highlighted image.
func (m Model) Selected() *backend.Image {
	if len(m.filtered) == 0 {
		return nil
	}
	img := m.filtered[m.cursor]
	return &img
}

// MarkedIDs returns multi-selected image IDs, each at most once: several
// references can share one digest, and the delete takes digests.
func (m Model) MarkedIDs() []string {
	var out []string
	seen := make(map[string]bool)
	for _, img := range m.filtered {
		if m.marked[markKey(img)] && !seen[img.ID] {
			seen[img.ID] = true
			out = append(out, img.ID)
		}
	}
	return out
}

// ClearMarks clears multi-select.
func (m Model) ClearMarks() Model {
	m.marked = make(map[string]bool)
	return m
}

// MoveBy adjusts cursor.
func (m Model) MoveBy(delta int) Model {
	m.cursor = uiutil.MoveCursor(m.cursor, len(m.filtered), delta)
	return m
}

// SetCursor sets absolute cursor.
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
	case "space":
		if sel := m.Selected(); sel != nil {
			if m.marked == nil {
				m.marked = make(map[string]bool)
			}
			k := markKey(*sel)
			if m.marked[k] {
				delete(m.marked, k)
			} else {
				m.marked[k] = true
			}
		}
	}
	return m, nil
}

func applyFilter(items []backend.Image, filter string) []backend.Image {
	if filter == "" {
		return items
	}
	f := strings.ToLower(filter)
	var out []backend.Image
	for _, img := range items {
		ref := strings.ToLower(backend.FormatRef(img))
		if strings.Contains(ref, f) || strings.Contains(strings.ToLower(img.ID), f) {
			out = append(out, img)
		}
	}
	return out
}

// ListView renders the image list.
func (m Model) ListView(width, height int) string {
	header := lipgloss.NewStyle().Foreground(lipgloss.Color("#6b7280")).
		BorderBottom(true).BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("#374151")).Width(width).
		Render(fmt.Sprintf("%-*s %-*s %s", colRef, "REPOSITORY:TAG", colSize, "SIZE", "CREATED"))

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
		img := m.filtered[i]
		mark := " "
		if m.marked[markKey(img)] {
			mark = "*"
		}
		line := fmt.Sprintf("%s%s %-*s %s",
			mark,
			uiutil.Pad(backend.FormatRef(img), colRef-1),
			colSize, uiutil.HumanBytes(img.Size),
			uiutil.Ago(img.Created),
		)
		st := row
		if i == m.cursor {
			st = sel
		}
		rows = append(rows, st.Width(width).Render(line))
	}
	if len(rows) == 0 {
		msg := "  no images"
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
		lipgloss.NewStyle().Foreground(lipgloss.Color("#6b7280")).Render("[p] pull  [c] run  [d] delete  [P] prune"),
	}
	return lipgloss.NewStyle().Width(width).Height(height).PaddingLeft(1).
		Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}
