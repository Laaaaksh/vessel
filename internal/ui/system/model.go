// Package system is the read-only system panel: service status and disk
// usage. It has no filter, no marks and no delete path - every method here
// only ever stores the latest poll result, never issues a mutating command.
package system

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/Laaaaksh/vessel/internal/backend"
	"github.com/Laaaaksh/vessel/internal/ui/uiutil"
)

// row is one of the fixed rows this panel shows: overall service status,
// plus one disk-usage category per resource kind.
type row int

const (
	rowService row = iota
	rowContainers
	rowImages
	rowVolumes
)

// rowCount is the number of fixed rows. Unlike the other panels this count
// never comes from a CLI response, so it is a constant rather than len(items).
const rowCount = 4

const colLabel = 12

// Model is the system panel.
type Model struct {
	status    *backend.SystemStatus
	statusErr error
	usage     *backend.DiskUsage
	usageErr  error
	cursor    int
	pageRows  int
}

// New creates an empty system model.
func New() Model {
	return Model{pageRows: 10}
}

// SetPageRows sets page scroll size.
func (m Model) SetPageRows(n int) Model {
	if n > 0 {
		m.pageRows = n
	}
	return m
}

// SetStatus stores the latest service-status poll result.
func (m Model) SetStatus(status *backend.SystemStatus, err error) Model {
	m.status = status
	m.statusErr = err
	return m
}

// SetDiskUsage stores the latest disk-usage poll result.
func (m Model) SetDiskUsage(usage *backend.DiskUsage, err error) Model {
	m.usage = usage
	m.usageErr = err
	return m
}

// Cursor returns the highlighted row index, for the footer.
func (m Model) Cursor() int { return m.cursor }

// Len returns the number of rows, for the footer.
func (m Model) Len() int { return rowCount }

// ServicesDown reports whether the last status poll found the container
// services not running. It stays false until a status has actually arrived,
// so a slow first poll never flashes a false "down" state.
func (m Model) ServicesDown() bool {
	return m.status != nil && !m.status.IsRunning()
}

// MoveBy moves the cursor by delta rows.
func (m Model) MoveBy(delta int) Model {
	m.cursor = uiutil.MoveCursor(m.cursor, rowCount, delta)
	return m
}

// SetCursor moves the cursor to row i, clamped to the visible rows.
func (m Model) SetCursor(i int) Model {
	m.cursor = uiutil.MoveCursor(i, rowCount, 0)
	return m
}

// YankText returns the text the app copies to the clipboard for the
// selected row: the service version, or a category's total size.
func (m Model) YankText() string {
	if row(m.cursor) == rowService {
		if m.status != nil {
			return m.status.Version
		}
		return ""
	}
	if cat, ok := m.category(row(m.cursor)); ok {
		return uiutil.HumanBytes(int64(cat.SizeBytes))
	}
	return ""
}

// Update handles navigation.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	kp, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	k := kp.String()
	switch k {
	case "j", "down":
		m.cursor = uiutil.MoveCursor(m.cursor, rowCount, 1)
	case "k", "up":
		m.cursor = uiutil.MoveCursor(m.cursor, rowCount, -1)
	case "pgdown", "ctrl+d":
		m.cursor = uiutil.MoveCursor(m.cursor, rowCount, uiutil.PageDelta(m.pageRows, k == "ctrl+d", true))
	case "pgup", "ctrl+u":
		m.cursor = uiutil.MoveCursor(m.cursor, rowCount, uiutil.PageDelta(m.pageRows, k == "ctrl+u", false))
	case "g":
		m.cursor = 0
	case "G":
		m.cursor = rowCount - 1
	}
	return m, nil
}

func rowLabel(r row) string {
	switch r {
	case rowService:
		return "Service"
	case rowContainers:
		return "Containers"
	case rowImages:
		return "Images"
	case rowVolumes:
		return "Volumes"
	default:
		return ""
	}
}

// category returns the disk-usage category for row r, if usage has arrived.
func (m Model) category(r row) (backend.DiskUsageCategory, bool) {
	if m.usage == nil {
		return backend.DiskUsageCategory{}, false
	}
	switch r {
	case rowContainers:
		return m.usage.Containers, true
	case rowImages:
		return m.usage.Images, true
	case rowVolumes:
		return m.usage.Volumes, true
	default:
		return backend.DiskUsageCategory{}, false
	}
}

// summary is the one-line value shown in the list for row r.
func (m Model) summary(r row) string {
	if r == rowService {
		switch {
		case m.status == nil && m.statusErr != nil:
			return "error"
		case m.status == nil:
			return "…"
		case m.status.IsRunning():
			return "● running  " + m.status.Version
		default:
			return "○ not running"
		}
	}
	// The services-down state is this view's own subject, not a failure to
	// report on it, so it reads the same as every other row rather than as
	// an error.
	if m.ServicesDown() {
		return "services not running"
	}
	cat, ok := m.category(r)
	switch {
	case !ok && m.usageErr != nil:
		return "error"
	case !ok:
		return "…"
	default:
		return fmt.Sprintf("%d total · %d active · %s", cat.Total, cat.Active, uiutil.HumanBytes(int64(cat.SizeBytes)))
	}
}

// ListView renders the fixed row list.
func (m Model) ListView(width, height int) string {
	header := lipgloss.NewStyle().Foreground(lipgloss.Color("#6b7280")).
		BorderBottom(true).BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("#374151")).Width(width).
		Render(fmt.Sprintf("%-*s %s", colLabel, "RESOURCE", "STATUS"))

	sel := lipgloss.NewStyle().Background(lipgloss.Color("#2d1b69")).Foreground(lipgloss.Color("#c4b5fd"))
	rowStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#e2e8f0"))

	// The leading space, the label column and its separating space, charged
	// against width so the value truncates to keep every row a single
	// rendered line - the service version string is long enough to wrap a
	// row across several lines at a narrow width otherwise, pushing the rows
	// below it out of the pane.
	valueWidth := max(1, width-colLabel-2)

	var lines []string
	for i := 0; i < rowCount; i++ {
		r := row(i)
		line := fmt.Sprintf(" %-*s %s", colLabel, rowLabel(r), uiutil.Truncate(m.summary(r), valueWidth))
		st := rowStyle
		if i == m.cursor {
			st = sel
		}
		lines = append(lines, st.Width(width).Render(line))
	}
	content := lipgloss.JoinVertical(lipgloss.Left, append([]string{header}, lines...)...)
	return lipgloss.NewStyle().Width(width).Height(height).Render(content)
}

// DetailView renders the details for the selected row.
func (m Model) DetailView(width, height int) string {
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("#6b7280"))
	errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#f87171"))
	title := lipgloss.NewStyle().Foreground(lipgloss.Color("#a78bfa")).Bold(true)

	hints, reserved := uiutil.KeyBar("[j/k] navigate  [y] yank  (read-only)", width, height)
	keybar := dim.Render(hints)

	r := row(m.cursor)
	p := uiutil.NewPane(width, height-reserved)
	p.Add(title.Render(uiutil.Headline(rowLabel(r), width, height)), "")

	if r == rowService {
		m.addServiceDetail(p, dim, errStyle, width)
	} else {
		m.addCategoryDetail(p, r, dim, errStyle, width)
	}

	p.AddReserved(reserved, "", keybar)
	return uiutil.RenderPane(width, height, p.Lines())
}

func (m Model) addServiceDetail(p *uiutil.Pane, dim, errStyle lipgloss.Style, width int) {
	switch {
	case m.status == nil && m.statusErr != nil:
		p.Add(uiutil.IndentedRows([]string{m.statusErr.Error()}, errStyle, width)...)
	case m.status == nil:
		p.Add(dim.Render("loading…"))
	default:
		// KVFit throughout, not KV: the version string is long enough that an
		// unbounded value wraps across several rendered rows, which can blow
		// the pane's row budget and drop it - and everything after it -
		// entirely at a narrow width. KV's label column is also a fixed 9
		// columns (including the colon), so every label here stays short
		// enough to fit it: a longer label wraps onto a second row and drags
		// the value with it.
		p.Add(
			uiutil.KVFit("Status", m.status.Status, width),
			uiutil.KVFit("Version", m.status.Version, width),
			uiutil.KVFit("App root", m.status.AppRoot, width),
			uiutil.KVFit("Install", m.status.InstallRoot, width),
		)
	}
}

func (m Model) addCategoryDetail(p *uiutil.Pane, r row, dim, errStyle lipgloss.Style, width int) {
	if m.ServicesDown() {
		p.Add(dim.Render("services not running"), "", dim.Render("start the container services to see disk usage."))
		return
	}
	cat, ok := m.category(r)
	switch {
	case !ok && m.usageErr != nil:
		p.Add(uiutil.IndentedRows([]string{m.usageErr.Error()}, errStyle, width)...)
	case !ok:
		p.Add(dim.Render("loading…"))
	default:
		p.Add(
			uiutil.KV("Total", fmt.Sprintf("%d", cat.Total)),
			uiutil.KV("Active", fmt.Sprintf("%d", cat.Active)),
			uiutil.KV("Size", uiutil.HumanBytes(int64(cat.SizeBytes))),
			// "Reclaimable" itself would overflow KV's fixed 9-column label
			// and wrap onto a second row, dragging the value with it.
			uiutil.KV("Reclaim", uiutil.HumanBytes(int64(cat.ReclaimableBytes))),
		)
	}
}
