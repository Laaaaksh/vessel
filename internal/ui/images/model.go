package images

import (
	"fmt"
	"strings"
	"unicode/utf8"

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
	items      []backend.Image
	filtered   []backend.Image
	cursor     int
	filter     string
	filtering  bool
	marked     map[string]bool
	toggleMark string
	pageRows   int
	notice     string
	noticeRef  string

	// inspect holds the latest ImageInspect for inspectRef, if it matches the
	// currently selected image. It is carried separately from the list so the
	// detail pane can keep showing list info while inspection is in flight.
	inspect    *backend.ImageInspect
	inspectRef string
	inspectErr error
	// inspectKey identifies the row the result was accepted for. A reference
	// alone is not an identity - every dangling image has an empty one - so a
	// result keyed by reference would surface under all of them.
	inspectKey string
	// inspectID is the content id inspectKey was resolved against, so a failed
	// inspect is invalidated on the same terms as a successful one instead of
	// outliving the row that produced it.
	inspectID string
}

// SetNotice attaches a standing message to one image's detail pane. The pane
// wraps, so unlike the footer it can carry an instruction too long to fit one
// row. The notice belongs to ref, not to the panel: a refusal reported for one
// image must not follow the cursor onto the next row, so DetailView renders it
// only while ref is the image on show. An empty notice clears both.
func (m Model) SetNotice(ref, notice string) Model {
	if notice == "" {
		m.notice, m.noticeRef = "", ""
		return m
	}
	m.notice, m.noticeRef = notice, ref
	return m
}

// New creates an empty images model.
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

// markKey identifies a row for multi-select. Two references can resolve to the
// same digest, so the id alone would key both rows to a single mark.
func markKey(img backend.Image) string {
	return img.ID + "\x00" + backend.FormatRef(img)
}

// SetItems replaces the image list. Marks for images the list no longer
// contains are dropped, so a mark can never outlive the row it points at. So
// is a cached inspect whose image is gone from the list, or whose reference
// now resolves to different content (a re-pulled tag keeps the ref but changes
// the index digest), so the detail pane cannot pair a fresh list row with a
// stale inspect.
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
	if m.noticeRef != "" {
		present := false
		for _, img := range items {
			if backend.FormatRef(img) == m.noticeRef {
				present = true
				break
			}
		}
		if !present {
			m.notice, m.noticeRef = "", ""
		}
	}
	if m.cursor >= len(m.filtered) {
		m.cursor = max(0, len(m.filtered)-1)
	}
	if m.inspectKey != "" && !inspectMatchesList(items, m.inspectKey, m.inspectID) {
		m.inspect = nil
		m.inspectRef = ""
		m.inspectErr = nil
		m.inspectKey = ""
		m.inspectID = ""
	}
	return m
}

func inspectMatchesList(items []backend.Image, key, id string) bool {
	for _, it := range items {
		if markKey(it) == key {
			return it.ID == id
		}
	}
	return false
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

// SetInspect stores the inspected detail for the given image reference. The
// panel keeps the result keyed by ref and only renders it while that image is
// selected, so a slow response never labels the wrong image. A result for an
// image that is no longer selected is discarded rather than replacing the
// cache, so it cannot evict a valid inspect for the current selection.
func (m Model) SetInspect(ref string, ins *backend.ImageInspect, err error) Model {
	sel := m.Selected()
	if sel == nil || backend.FormatRef(*sel) != ref {
		return m
	}
	m.inspect = ins
	m.inspectRef = ref
	m.inspectErr = err
	m.inspectKey = markKey(*sel)
	// A successful inspect reports the digest it actually resolved; a failure
	// leaves only the list row it was asked for. Either way that id is what a
	// later list must still agree with for the result to stay valid.
	m.inspectID = sel.ID
	if ins != nil {
		m.inspectID = ins.ID
	}
	return m
}

// InspectedRef returns the image reference the panel already holds a
// successful inspect for, or "" when the last inspect failed or none has
// arrived. The app uses it to avoid re-inspecting an unchanged selection on
// every poll tick.
func (m Model) InspectedRef() string {
	if m.inspect == nil || m.inspectErr != nil {
		return ""
	}
	return m.inspectRef
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

// DetailView renders image details. Height is a floor for lipgloss, never a
// cap, so the pane is capped explicitly: anything longer would grow the body row
// and push the header off the alt-screen. The notice leads the pane so that when
// the cap bites — 18x4 at the smallest supported frame — it is the reference and
// the static fields that get cut, not the instruction the user has to act on. It
// shows only on the image it was recorded against, so moving the cursor hides it
// and moving back brings it into view again.
func (m Model) DetailView(width, height int) string {
	pane := lipgloss.NewStyle().Width(width).Height(height).MaxHeight(max(1, height))
	sel := m.Selected()
	if sel == nil {
		return pane.Foreground(lipgloss.Color("#6b7280")).Render("  no image selected")
	}
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("#6b7280"))
	errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#f87171"))
	hints, reserved := uiutil.KeyBar("[p] pull  [c] run  [d] delete  [P] prune", width, height)
	keybar := dim.Render(hints)

	// A standing notice must never be dropped by the pane's row budget: it is
	// the instruction the user has to act on, so it leads the pane and its own
	// row cost is added on top of the normal budget rather than charged
	// against it. RenderPane's final ClampHeight still caps the whole pane at
	// height, so when the notice is large it is the reference/static fields
	// added after it that get cut, never the notice itself.
	budget := height - reserved
	var notice string
	if m.notice != "" && m.noticeRef == backend.FormatRef(*sel) {
		notice = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#fbbf24")).
			Width(max(1, width-2)).
			Render(m.notice)
		budget += uiutil.RowsFor(notice, width) + 1
	}
	p := uiutil.NewPane(width, budget)
	if notice != "" {
		p.Add(notice, "")
	}
	p.Add(
		lipgloss.NewStyle().Foreground(lipgloss.Color("#a78bfa")).Bold(true).
			Render(uiutil.Headline(backend.FormatRef(*sel), width, height)),
		"",
		// Deliberately not KVFit: the id is the identity content the pane must
		// still show at minimum geometry, so binding it to the pane width would
		// defeat the rule that binding its siblings serves.
		uiutil.KV("ID", uiutil.Truncate(sel.ID, 16)),
		uiutil.KV("Size", uiutil.HumanBytes(sel.Size)),
		uiutil.KV("Created", uiutil.Ago(sel.Created)),
	)

	if m.inspect != nil && m.inspectKey == markKey(*sel) {
		if d := m.inspect.Digest; d != "" {
			p.Add(uiutil.KVFit("Digest", d, width))
		}
		if len(m.inspect.Cmd) > 0 {
			p.Add(uiutil.KVFit("Cmd", strings.Join(m.inspect.Cmd, " "), width))
		}
		if m.inspect.WorkingDir != "" {
			p.Add(uiutil.KVFit("Workdir", m.inspect.WorkingDir, width))
		}
		if m.inspect.LayerCount > 0 {
			p.Add(uiutil.KV("Layers", fmt.Sprintf("%d", m.inspect.LayerCount)))
		}

		p.Section(dim.Render("-- Env --"), uiutil.IndentedRows(m.inspect.Env, dim, width))

		platforms := make([]string, 0, len(m.inspect.Platforms))
		for _, pf := range m.inspect.Platforms {
			name := pf.OS + "/" + pf.Architecture
			if pf.Variant != "" {
				name += "/" + pf.Variant
			}
			platforms = append(platforms, name+"  "+uiutil.HumanBytes(pf.Size))
		}
		p.Section(dim.Render("-- Platforms --"), uiutil.IndentedRows(platforms, dim, width))
	} else if m.inspectErr != nil && m.inspectKey == markKey(*sel) {
		p.Add("")
		p.Add(uiutil.IndentedRows([]string{m.inspectErr.Error()}, errStyle, width)...)
	}

	p.AddReserved(reserved, "", keybar)
	return uiutil.RenderPane(width, height, p.Lines())
}
