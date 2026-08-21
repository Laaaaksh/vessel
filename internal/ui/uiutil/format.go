// Package uiutil holds the small formatting helpers shared by the panel models.
package uiutil

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// kvLabelWidth is the column width of a KV row's label, which is followed by a
// single separating space.
const kvLabelWidth = 9

// KV renders a dim "key:" label followed by its value.
func KV(key, val string) string {
	k := lipgloss.NewStyle().Foreground(lipgloss.Color("#6b7280")).Width(kvLabelWidth).Render(key + ":")
	v := lipgloss.NewStyle().Foreground(lipgloss.Color("#e2e8f0")).Render(val)
	return lipgloss.JoinHorizontal(lipgloss.Top, k, " ", v)
}

// KVFit renders a KV row whose value is shortened to whatever is left after the
// label and its separating space, so the row occupies a single rendered row in
// a pane of that width. Callers must not compute that budget themselves: the
// label geometry belongs here, and getting it one column wrong costs the row an
// extra rendered row.
func KVFit(key, val string, width int) string {
	return KV(key, Truncate(val, width-kvLabelWidth-2))
}

// Truncate shortens s to at most max runes, ending in an ellipsis when cut.
// A max of zero or less yields an empty string rather than panicking.
func Truncate(s string, max int) string {
	if max <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	if max == 1 {
		return "…"
	}
	return string(r[:max-1]) + "…"
}

// Pad truncates s to w runes and left-aligns it in a field of that width.
func Pad(s string, w int) string {
	return fmt.Sprintf("%-*s", w, Truncate(s, w))
}

// TruncateCells shortens s to at most w terminal cells, ending in an ellipsis
// when cut. A terminal — and lipgloss, which wraps rather than truncates —
// measures display width, so a double-width character such as 世 takes two
// cells: cutting by rune count lets a line overflow the width it was fitted to
// and wrap onto a second row. A w of zero or less yields an empty string.
func TruncateCells(s string, w int) string {
	if w <= 0 {
		return ""
	}
	return ansi.Truncate(s, w, "…")
}

// PadCells truncates s to w terminal cells and left-aligns it in a field of
// that width.
func PadCells(s string, w int) string {
	s = TruncateCells(s, w)
	if pad := w - lipgloss.Width(s); pad > 0 {
		return s + strings.Repeat(" ", pad)
	}
	return s
}

// Window returns the [start, end) bounds of a scroll window of at most size
// rows over total items, keeping cursor inside it. lipgloss pads a panel to its
// declared height but never truncates, so a list that renders every item pushes
// the footer off screen; every list slices through this instead.
func Window(total, cursor, size int) (start, end int) {
	if total <= 0 || size <= 0 {
		return 0, 0
	}
	if total <= size {
		return 0, total
	}
	start = cursor - size/2
	if start > total-size {
		start = total - size
	}
	if start < 0 {
		start = 0
	}
	return start, start + size
}

// Ago renders t as a coarse relative age, or "-" for the zero time.
func Ago(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return t.Format("2006-01-02")
	}
}

// HumanBytes renders a byte count with a binary unit suffix.
func HumanBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.0f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}
