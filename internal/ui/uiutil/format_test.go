package uiutil

import (
	"strings"
	"testing"
)

func TestTruncate(t *testing.T) {
	cases := []struct {
		s    string
		max  int
		want string
	}{
		{"abc", 5, "abc"},
		{"abc", 3, "abc"},
		{"abcdef", 4, "abc…"},
		{"abc", 1, "…"},
		{"/var/lib/vessel/volumes/data", 0, ""},
		{"/var/lib/vessel/volumes/data", -8, ""},
		{"héllo wörld", 6, "héllo…"},
	}
	for _, c := range cases {
		if got := Truncate(c.s, c.max); got != c.want {
			t.Errorf("Truncate(%q, %d) = %q, want %q", c.s, c.max, got, c.want)
		}
	}
}

func TestPad(t *testing.T) {
	if got := Pad("ab", 4); got != "ab  " {
		t.Errorf("Pad = %q", got)
	}
	if got := Pad("abcdef", 4); got != "abc…" {
		t.Errorf("Pad = %q", got)
	}
}

func TestWindow(t *testing.T) {
	cases := []struct {
		name                string
		total, cursor, size int
		wantStart, wantEnd  int
	}{
		{"fits", 3, 0, 10, 0, 3},
		{"empty", 0, 0, 10, 0, 0},
		{"no room", 5, 0, 0, 0, 0},
		{"cursor at top", 100, 0, 10, 0, 10},
		{"cursor centred", 100, 50, 10, 45, 55},
		{"cursor at bottom", 100, 99, 10, 90, 100},
		{"clamped near end", 100, 96, 10, 90, 100},
	}
	for _, c := range cases {
		start, end := Window(c.total, c.cursor, c.size)
		if start != c.wantStart || end != c.wantEnd {
			t.Errorf("%s: Window(%d,%d,%d) = (%d,%d), want (%d,%d)",
				c.name, c.total, c.cursor, c.size, start, end, c.wantStart, c.wantEnd)
		}
		if c.total > 0 && c.size > 0 && (c.cursor < start || c.cursor >= end) {
			t.Errorf("%s: cursor %d outside window [%d,%d)", c.name, c.cursor, start, end)
		}
	}
}

func TestHumanBytes(t *testing.T) {
	cases := map[int64]string{0: "0 B", 512: "512 B", 1024: "1 KiB", 5 * 1024 * 1024: "5 MiB"}
	for in, want := range cases {
		if got := HumanBytes(in); got != want {
			t.Errorf("HumanBytes(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestClampHeight(t *testing.T) {
	block := "a\nb\nc\nd"
	if got := ClampHeight(block, 2); got != "a\nb" {
		t.Errorf("ClampHeight(4 lines, 2) = %q", got)
	}
	if got := ClampHeight(block, 9); got != block {
		t.Errorf("short block must pass through, got %q", got)
	}
	if got := ClampHeight(block, 0); got != "" {
		t.Errorf("zero height must render nothing, got %q", got)
	}
}

func TestPane_sectionSkippedWhenHeaderAndRowDoNotFit(t *testing.T) {
	p := NewPane(20, 4)
	p.Add("one", "two")
	p.Section("-- Labels --", []string{"a=1"})
	if got := p.Lines(); len(got) != 2 {
		t.Errorf("section must be dropped whole when it cannot fit, got %v", got)
	}

	p = NewPane(20, 5)
	p.Add("one", "two")
	p.Section("-- Labels --", []string{"a=1", "b=2"})
	got := p.Lines()
	if len(got) != 5 || got[3] != "-- Labels --" || got[4] != "a=1" {
		t.Errorf("section must fill exactly the budget, got %v", got)
	}
}

func TestPane_chargesWrappedRowsAgainstTheBudget(t *testing.T) {
	// 30 columns of text in a 20-column pane occupies two rendered rows.
	wide := strings.Repeat("x", 30)
	p := NewPane(20, 3)
	p.Add(wide, "short", "dropped")
	got := p.Lines()
	if len(got) != 2 || got[1] != "short" {
		t.Fatalf("a wrapped row must cost two rows, got %v", got)
	}
	if p.Remaining() != 0 {
		t.Errorf("remaining = %d, want 0", p.Remaining())
	}
}

func TestRowsFor_countsWrappedRows(t *testing.T) {
	if got := RowsFor("short", 20); got != 1 {
		t.Errorf("RowsFor(short) = %d, want 1", got)
	}
	if got := RowsFor(strings.Repeat("x", 30), 20); got != 2 {
		t.Errorf("RowsFor(30 cols in a 20-col pane) = %d, want 2", got)
	}
}

func TestRenderPane_neverExceedsHeight(t *testing.T) {
	lines := make([]string, 10)
	for i := range lines {
		lines[i] = strings.Repeat("y", 30)
	}
	out := RenderPane(20, 6, lines)
	if got := strings.Count(out, "\n") + 1; got != 6 {
		t.Errorf("RenderPane rendered %d rows into 6", got)
	}
}

func TestReserve_boundedToHalfThePane(t *testing.T) {
	if got := Reserve(6, 8); got != 4 {
		t.Errorf("Reserve(6, 8) = %d, want 4", got)
	}
	if got := Reserve(2, 8); got != 2 {
		t.Errorf("Reserve(2, 8) = %d, want 2", got)
	}
	if got := Reserve(3, 4); got != 2 {
		t.Errorf("Reserve(3, 4) = %d, want 2", got)
	}
}

func TestPane_dropsEverythingAfterTheFirstRowThatDoesNotFit(t *testing.T) {
	p := NewPane(20, 3)
	p.Add("one")
	p.Add(strings.Repeat("x", 50)) // three rendered rows: does not fit
	p.Add("short")                 // would fit, but must not jump the queue

	got := p.Lines()
	if len(got) != 1 || got[0] != "one" {
		t.Fatalf("rows must render as a prefix of what was asked for, got %v", got)
	}

	p.Grow(2)
	p.Add("tail")
	if got := p.Lines(); len(got) != 2 || got[1] != "tail" {
		t.Errorf("Grow must reopen the pane for its reserved content, got %v", got)
	}
}
