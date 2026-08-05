package uiutil

import "testing"

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
