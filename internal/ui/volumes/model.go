package volumes

import (
	"fmt"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/Laaaaksh/vessel/internal/backend"
	"github.com/Laaaaksh/vessel/internal/ui/uiutil"
)

const (
	colName   = 28
	colDriver = 10
)

// Model is the volumes panel.
type Model struct {
	items      []backend.Volume
	filtered   []backend.Volume
	cursor     int
	filter     string
	filtering  bool
	marked     map[string]bool
	toggleMark string
	pageRows   int

	// inspect holds the latest VolumeInspect for inspectName, if it matches the
	// currently selected volume. Carried separately so list info shows while
	// inspection is in flight and a slow response never labels a wrong volume.
	inspect     *backend.VolumeInspect
	inspectName string
	inspectErr  error
}

// New creates an empty volumes model.
func New() Model {
	return Model{marked: make(map[string]bool), toggleMark: defaultToggleMark, pageRows: 10}
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

// SetItems replaces the volume list. Marks for volumes the list no longer
// contains are dropped, so a mark can never outlive the row it points at. So
// is a cached inspect whose volume is gone from the list, or whose list row no
// longer agrees with it (the volume was resized or recreated under the same
// name), so the detail pane cannot pair a fresh list row with a stale inspect.
func (m Model) SetItems(items []backend.Volume) Model {
	m.items = items
	m.filtered = applyFilter(items, m.filter)
	marked := make(map[string]bool, len(m.marked))
	for _, v := range items {
		if m.marked[v.Name] {
			marked[v.Name] = true
		}
	}
	m.marked = marked
	if m.cursor >= len(m.filtered) {
		m.cursor = max(0, len(m.filtered)-1)
	}
	if m.inspect != nil && !inspectMatchesList(items, m.inspectName, *m.inspect) {
		m.inspect = nil
		m.inspectName = ""
		m.inspectErr = nil
	}
	return m
}

func inspectMatchesList(items []backend.Volume, name string, ins backend.VolumeInspect) bool {
	for _, it := range items {
		if it.Name == name {
			return it.SizeBytes == ins.SizeBytes && it.Created.Equal(ins.Created)
		}
	}
	return false
}

// Selected returns the highlighted volume.
func (m Model) Selected() *backend.Volume {
	if len(m.filtered) == 0 {
		return nil
	}
	v := m.filtered[m.cursor]
	return &v
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

// MarkedIDs returns multi-selected volume names.
func (m Model) MarkedIDs() []string {
	var out []string
	for _, v := range m.filtered {
		if m.marked[v.Name] {
			out = append(out, v.Name)
		}
	}
	return out
}

// SetInspect stores the inspected detail for the given volume name, keyed by
// name so a slow response never labels the wrong volume. A result for a volume
// that is no longer selected is discarded rather than replacing the cache, so
// it cannot evict a valid inspect for the current selection.
func (m Model) SetInspect(name string, ins *backend.VolumeInspect, err error) Model {
	sel := m.Selected()
	if sel == nil || sel.Name != name {
		return m
	}
	m.inspect = ins
	m.inspectName = name
	m.inspectErr = err
	return m
}

// InspectedName returns the volume name the panel already holds a successful
// inspect for, or "" when the last inspect failed or none has arrived. The app
// uses it to avoid re-inspecting an unchanged selection on every poll tick.
func (m Model) InspectedName() string {
	if m.inspect == nil || m.inspectErr != nil {
		return ""
	}
	return m.inspectName
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
	case m.toggleMark:
		if sel := m.Selected(); sel != nil {
			if m.marked == nil {
				m.marked = make(map[string]bool)
			}
			if m.marked[sel.Name] {
				delete(m.marked, sel.Name)
			} else {
				m.marked[sel.Name] = true
			}
		}
	}
	return m, nil
}

func applyFilter(items []backend.Volume, filter string) []backend.Volume {
	if filter == "" {
		return items
	}
	f := strings.ToLower(filter)
	var out []backend.Volume
	for _, v := range items {
		if strings.Contains(strings.ToLower(v.Name), f) ||
			strings.Contains(strings.ToLower(v.Mountpoint), f) {
			out = append(out, v)
		}
	}
	return out
}

// ListView renders the volume list.
func (m Model) ListView(width, height int) string {
	header := lipgloss.NewStyle().Foreground(lipgloss.Color("#6b7280")).
		BorderBottom(true).BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("#374151")).Width(width).
		Render(fmt.Sprintf("%-*s %-*s %s", colName, "NAME", colDriver, "DRIVER", "CREATED"))

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
		v := m.filtered[i]
		mark := " "
		if m.marked[v.Name] {
			mark = "*"
		}
		line := fmt.Sprintf("%s%s %-*s %s", mark, uiutil.Pad(v.Name, colName-1), colDriver, v.Driver, uiutil.Ago(v.Created))
		st := row
		if i == m.cursor {
			st = sel
		}
		rows = append(rows, st.Width(width).Render(line))
	}
	if len(rows) == 0 {
		msg := "  no volumes"
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

// keybarLines is the spacer plus the key hint row the detail pane always ends
// with; sections must leave room for them.
const keybarLines = 2

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
	}

	same := sel.Name == m.inspectName && m.inspect != nil
	if same {
		if m.inspect.Format != "" {
			lines = append(lines, uiutil.KV("Format", m.inspect.Format))
		}
		if m.inspect.SizeBytes > 0 {
			lines = append(lines, uiutil.KV("Size", uiutil.HumanBytes(int64(m.inspect.SizeBytes))))
		}
	}
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("#6b7280"))
	budget := height - keybarLines
	if same {
		lines = uiutil.Section(lines, budget, dim.Render("-- Labels --"),
			pairRows(m.inspect.Labels, dim, width))
		lines = uiutil.Section(lines, budget, dim.Render("-- Options --"),
			pairRows(m.inspect.Options, dim, width))
	} else if m.inspectErr != nil && sel.Name == m.inspectName {
		lines = uiutil.AppendLines(lines, budget, "",
			lipgloss.NewStyle().Foreground(lipgloss.Color("#f87171")).
				Render("  "+uiutil.Truncate(m.inspectErr.Error(), width-6)))
	}

	lines = append(lines, "", dim.Render("[c] create  [d] delete  [P] prune  [y] yank path"))
	return lipgloss.NewStyle().Width(width).Height(height).PaddingLeft(1).
		Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}

func pairRows(pairs map[string]string, style lipgloss.Style, width int) []string {
	rows := make([]string, 0, len(pairs))
	for k, v := range pairs {
		rows = append(rows, style.Render("  "+uiutil.Truncate(k+"="+v, width-6)))
	}
	sort.Strings(rows)
	return rows
}
