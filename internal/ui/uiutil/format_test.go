package uiutil

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
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

	p.AddReserved(2, "tail")
	if got := p.Lines(); len(got) != 2 || got[1] != "tail" {
		t.Errorf("AddReserved must spend the reservation, got %v", got)
	}
}

func TestPane_reservationDoesNotReopenThePaneForLaterRows(t *testing.T) {
	p := NewPane(20, 3)
	p.Add("one")
	p.Add(strings.Repeat("x", 50)) // three rendered rows: latches the pane full
	p.AddReserved(2, "reserved")
	p.Section("-- Labels --", []string{"a=1"})

	got := p.Lines()
	if len(got) != 2 || got[1] != "reserved" {
		t.Fatalf("the reservation must still be spent, got %v", got)
	}
	for _, l := range got {
		if strings.Contains(l, "Labels") {
			t.Errorf("a section rendered after rows were dropped for space: %v", got)
		}
	}
}

func TestPane_reservationLeavesTheOpenPaneOpen(t *testing.T) {
	p := NewPane(20, 4)
	p.Add("one")
	p.AddReserved(2, "reserved")
	p.Section("-- Labels --", []string{"a=1"})

	got := p.Lines()
	if len(got) != 5 || got[1] != "reserved" || got[3] != "-- Labels --" {
		t.Errorf("a pane that never filled up must keep accepting rows, got %v", got)
	}
}

func TestKVFit_occupiesOneRenderedRow(t *testing.T) {
	digest := "sha256:e7a1a92a5bfeee40966aea60f0796b0e6d4b2c1a9f8e7d6c5b4a39281706f5e4"
	for _, width := range []int{18, 20, 40, 60} {
		row := KVFit("Digest", digest, width)
		if got := RowsFor(row, width); got != 1 {
			t.Errorf("KVFit at width %d occupies %d rendered rows, want 1: %q", width, got, row)
		}
	}
}

func TestKVFit_keepsAsMuchOfTheValueAsFits(t *testing.T) {
	row := KVFit("Digest", "sha256:abcdefghijklmnop", 40)
	if !strings.Contains(row, "sha256:abcdefghijklmnop") {
		t.Errorf("a value that fits must not be truncated: %q", row)
	}
	if got := RowsFor(row, 40); got != 1 {
		t.Errorf("row occupies %d rendered rows, want 1", got)
	}
}

func TestHeadline_wrapsWhileItIsASmallPartOfThePane(t *testing.T) {
	title := "docker.io/library/alpine:latest"
	got := Headline(title, 18, 20)
	if got != title {
		t.Errorf("a tall pane must show the whole title, got %q", got)
	}
	if RowsFor(got, 18) != 2 {
		t.Errorf("expected the title to wrap to two rows at width 18")
	}
}

func TestHeadline_truncatesWhenItWouldCrowdThePane(t *testing.T) {
	title := "docker.io/library/alpine:latest"
	got := Headline(title, 10, 8)
	if got == title {
		t.Fatal("a title that would take half a short pane must be truncated")
	}
	if rows := RowsFor(got, 10); rows != 1 {
		t.Errorf("truncated title occupies %d rows, want 1: %q", rows, got)
	}
}

func TestHeadline_leavesShortTitlesAlone(t *testing.T) {
	if got := Headline("web", 40, 8); got != "web" {
		t.Errorf("Headline(short) = %q, want it unchanged", got)
	}
}

func TestIndentedRows_occupyOneRenderedRowEach(t *testing.T) {
	values := []string{
		"/Users/someone/Library/Application Support/com.apple.container/volumes/data/volume.img → /data",
		"short → /x",
	}
	for _, width := range []int{18, 20, 40, 60} {
		for i, row := range IndentedRows(values, lipgloss.NewStyle(), width) {
			if got := RowsFor(row, width); got != 1 {
				t.Errorf("row %d at width %d occupies %d rendered rows, want 1: %q", i, width, got, row)
			}
		}
	}
}

func TestIndentedRows_indentsAndKeepsOrder(t *testing.T) {
	rows := IndentedRows([]string{"b", "a"}, lipgloss.NewStyle(), 40)
	want := []string{"  b", "  a"}
	if len(rows) != len(want) {
		t.Fatalf("got %d rows, want %d", len(rows), len(want))
	}
	for i := range want {
		if rows[i] != want[i] {
			t.Errorf("row %d = %q, want %q: order must follow the caller's slice", i, rows[i], want[i])
		}
	}
}

// Go map iteration order is randomised, so an unsorted pane would reshuffle
// its label rows between frames.
func TestPairRows_areStablyOrderedByPair(t *testing.T) {
	pairs := map[string]string{"zeta": "1", "alpha": "2", "mid": "3"}
	want := []string{"  alpha=2", "  mid=3", "  zeta=1"}
	for range 20 {
		got := PairRows(pairs, lipgloss.NewStyle(), 40)
		if len(got) != len(want) {
			t.Fatalf("got %d rows, want %d", len(got), len(want))
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("row %d = %q, want %q", i, got[i], want[i])
			}
		}
	}
}

func TestPairRows_occupyOneRenderedRowEach(t *testing.T) {
	pairs := map[string]string{
		"org.opencontainers.image.source": "https://github.com/example/a-fairly-long-repository-name",
	}
	for _, width := range []int{18, 40, 60} {
		for _, row := range PairRows(pairs, lipgloss.NewStyle(), width) {
			if got := RowsFor(row, width); got != 1 {
				t.Errorf("pair row at width %d occupies %d rendered rows, want 1: %q", width, got, row)
			}
		}
	}
}

func TestPairRows_emptyMapYieldsNoRows(t *testing.T) {
	if got := PairRows(nil, lipgloss.NewStyle(), 40); len(got) != 0 {
		t.Errorf("PairRows(nil) = %v, want no rows", got)
	}
}
