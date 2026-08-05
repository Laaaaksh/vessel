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

func TestHumanBytes(t *testing.T) {
	cases := map[int64]string{0: "0 B", 512: "512 B", 1024: "1 KiB", 5 * 1024 * 1024: "5 MiB"}
	for in, want := range cases {
		if got := HumanBytes(in); got != want {
			t.Errorf("HumanBytes(%d) = %q, want %q", in, got, want)
		}
	}
}
