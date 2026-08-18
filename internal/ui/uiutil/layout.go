package uiutil

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// paneStyle is the geometry every detail pane renders with. Budgeting and
// rendering share it, so a row is charged exactly the number of rows it will
// occupy once wrapped.
func paneStyle(width int) lipgloss.Style {
	return lipgloss.NewStyle().Width(width).PaddingLeft(1)
}

// RowsFor reports how many rendered rows s occupies in a pane of that width.
// A row wider than the pane wraps and costs more than one.
func RowsFor(s string, width int) int {
	if width < 2 {
		return lipgloss.Height(s)
	}
	return lipgloss.Height(paneStyle(width).Render(s))
}

// RenderPane renders the rows of a detail pane: short content is padded up to
// height and anything still taller is clipped, so the pane can never push the
// rest of the layout off screen.
func RenderPane(width, height int, lines []string) string {
	body := paneStyle(width).Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
	return lipgloss.NewStyle().Height(height).Render(ClampHeight(body, height))
}

// ClampHeight trims a rendered block to at most height lines. lipgloss pads
// short content up to a style's Height but never clips content that is taller.
func ClampHeight(s string, height int) string {
	if height <= 0 {
		return ""
	}
	lines := strings.Split(s, "\n")
	if len(lines) <= height {
		return s
	}
	return strings.Join(lines[:height], "\n")
}

// Reserve bounds a trailing-block reservation to half the pane, so a tall
// trailing block can never crowd out the rows that say what is selected.
func Reserve(rows, height int) int {
	if rows > height/2 {
		return height / 2
	}
	return rows
}

// KeyBar sizes a pane's trailing key-hint row. The bar is allowed to wrap
// while it stays well under half the pane; beyond that it is truncated to a
// single row so it cannot crowd out the rows that say what is selected. It
// returns the text to render and the rows to reserve for it, spacer included.
func KeyBar(text string, width, height int) (string, int) {
	if rows := 1 + RowsFor(text, width); rows < height/2 {
		return text, rows
	}
	short := Truncate(text, width-1)
	return short, Reserve(1+RowsFor(short, width), height)
}

// Headline sizes a pane's title row. Like KeyBar it may wrap while it stays
// well under half the pane, so a long name still shows in full where there is
// room; beyond that it is truncated to a single row rather than pushing the
// rows that describe the selection out of the pane.
func Headline(text string, width, height int) string {
	if 1+RowsFor(text, width) < height/2 {
		return text
	}
	return Truncate(text, width-1)
}

// Pane accumulates detail-pane rows within a budget counted in rendered rows.
// Once a row does not fit the pane is full: later rows are dropped even when
// they would fit, so what renders is always a prefix of what was asked for
// rather than an arbitrary subset. AddReserved spends a reservation without
// reopening the pane for anything else.
type Pane struct {
	width  int
	budget int
	used   int
	full   bool
	lines  []string
}

// NewPane starts a pane of the given width that may occupy budget rendered
// rows. Reserve room for trailing content by passing a reduced budget and
// spending it with AddReserved once the earlier rows are in.
func NewPane(width, budget int) *Pane {
	return &Pane{width: width, budget: max(0, budget)}
}

// AddReserved appends the rows a reservation of that many rendered rows was
// held for. A pane that filled up before the reservation is spent stays full
// afterwards, so rows added later cannot jump ahead of rows already dropped.
func (p *Pane) AddReserved(reserved int, rows ...string) {
	wasFull := p.full
	p.budget += reserved
	p.full = false
	p.Add(rows...)
	if wasFull {
		p.full = true
	}
}

// Remaining reports how many rendered rows are still free.
func (p *Pane) Remaining() int { return p.budget - p.used }

// Add appends rows while the budget allows and drops the rest.
func (p *Pane) Add(more ...string) {
	for _, l := range more {
		n := RowsFor(l, p.width)
		if p.full || p.used+n > p.budget {
			p.full = true
			return
		}
		p.used += n
		p.lines = append(p.lines, l)
	}
}

// Section appends a spacer, a header and as many items as fit. The section is
// skipped whole when the header and at least one item do not fit, so a full
// pane never shows a dangling heading.
func (p *Pane) Section(header string, items []string) {
	if len(items) == 0 {
		return
	}
	need := 1 + RowsFor(header, p.width) + RowsFor(items[0], p.width)
	if p.full || p.used+need > p.budget {
		p.full = true
		return
	}
	p.Add("", header)
	p.Add(items...)
}

// Lines returns the accumulated rows.
func (p *Pane) Lines() []string { return p.lines }
