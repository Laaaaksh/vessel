package volumes

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

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
	// inspectRow fingerprints the list row inspectName was resolved against,
	// so a failed inspect is invalidated on the same terms as a successful
	// one instead of outliving the row that produced it.
	inspectRow volumeFingerprint
}

// volumeFingerprint is the identity of a list row for inspect-cache
// invalidation: a volume recreated or resized under the same name is a
// different row.
type volumeFingerprint struct {
	sizeBytes uint64
	created   time.Time
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
	if m.inspectName != "" && !inspectMatchesList(items, m.inspectName, m.inspectRow) {
		m.inspect = nil
		m.inspectName = ""
		m.inspectErr = nil
		m.inspectRow = volumeFingerprint{}
	}
	return m
}

func inspectMatchesList(items []backend.Volume, name string, row volumeFingerprint) bool {
	for _, it := range items {
		if it.Name == name {
			return it.SizeBytes == row.sizeBytes && it.Created.Equal(row.created)
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

// SingleMarked returns the one marked volume when exactly one visible row
// carries a mark, and nil otherwise. A lone mark states its target more
// precisely than the cursor does, so the delete path prefers it; with zero or
// several marks the cursor keeps deciding.
func (m Model) SingleMarked() *backend.Volume {
	var found *backend.Volume
	for i := range m.filtered {
		if !m.marked[m.filtered[i].Name] {
			continue
		}
		if found != nil {
			return nil
		}
		found = &m.filtered[i]
	}
	return found
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
	// A successful inspect reports the volume's own size and age; a failure
	// leaves only the list row it was asked for. Either way the fingerprint is
	// what a later list must still agree with.
	m.inspectRow = volumeFingerprint{sizeBytes: sel.SizeBytes, created: sel.Created}
	if ins != nil {
		m.inspectRow = volumeFingerprint{sizeBytes: ins.SizeBytes, created: ins.Created}
	}
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

// DetailView renders volume details.
func (m Model) DetailView(width, height int) string {
	sel := m.Selected()
	if sel == nil {
		return lipgloss.NewStyle().Width(width).Height(height).
			Foreground(lipgloss.Color("#6b7280")).Render("  no volume selected")
	}
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("#6b7280"))
	errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#f87171"))
	hints, reserved := uiutil.KeyBar("[c] create  [d] delete  [P] prune  [y] yank path", width, height)
	keybar := dim.Render(hints)

	p := uiutil.NewPane(width, height-reserved)
	p.Add(
		lipgloss.NewStyle().Foreground(lipgloss.Color("#a78bfa")).Bold(true).
			Render(uiutil.Headline(sel.Name, width, height)),
		"",
		uiutil.KV("Driver", sel.Driver),
		uiutil.KV("Created", uiutil.Ago(sel.Created)),
		uiutil.KVFit("Path", sel.Mountpoint, width),
	)

	// The list already carries size, format, labels and options, so the pane
	// shows them from the moment it paints and keeps them when the inspect
	// fails. A successful inspect is the more authoritative source and wins.
	same := sel.Name == m.inspectName && m.inspect != nil
	format, sizeBytes := sel.Format, sel.SizeBytes
	labels, options := sel.Labels, sel.Options
	if same {
		format, sizeBytes = m.inspect.Format, m.inspect.SizeBytes
		labels, options = m.inspect.Labels, m.inspect.Options
	}
	if format != "" {
		p.Add(uiutil.KV("Format", format))
	}
	if sizeBytes > 0 {
		p.Add(uiutil.KV("Quota", uiutil.HumanBytes(int64(sizeBytes))))
	}
	p.Section(dim.Render("-- Labels --"), uiutil.PairRows(labels, dim, width))
	p.Section(dim.Render("-- Options --"), uiutil.PairRows(options, dim, width))
	if !same && m.inspectErr != nil && sel.Name == m.inspectName {
		p.Add("")
		p.Add(uiutil.IndentedRows([]string{m.inspectErr.Error()}, errStyle, width)...)
	}

	p.AddReserved(reserved, "", keybar)
	return uiutil.RenderPane(width, height, p.Lines())
}
