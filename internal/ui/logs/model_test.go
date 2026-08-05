package logs

import (
	"fmt"
	"strings"
	"testing"
)

func makeLines(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = fmt.Sprintf("line %d", i)
	}
	return out
}

// A stored offset only stays valid until the buffer or the viewport changes.
// Both cases below used to render a single row into a full-height body.
func TestVisibleLines_offsetSurvivesBufferTrim(t *testing.T) {
	m := New().SetSize(100, 43)
	m = m.Open("web", makeLines(5000))
	m.offset = m.maxOffset()

	m = m.Append("one more")

	if got, want := len(m.visibleLines()), m.bodyHeight(); got != want {
		t.Errorf("after trim: %d rows, want %d", got, want)
	}
}

func TestVisibleLines_offsetSurvivesResize(t *testing.T) {
	m := New().SetSize(100, 13)
	m = m.Open("web", makeLines(100))
	m.offset = m.maxOffset()

	m = m.SetSize(100, 53)

	if got, want := len(m.visibleLines()), m.bodyHeight(); got != want {
		t.Errorf("after resize: %d rows, want %d", got, want)
	}
}

func TestVisibleLines_topAndTail(t *testing.T) {
	m := New().SetSize(100, 13)
	m = m.Open("web", makeLines(100))

	tail := m.visibleLines()
	if got, want := len(tail), m.bodyHeight(); got != want {
		t.Fatalf("tail: %d rows, want %d", got, want)
	}
	if !strings.Contains(tail[len(tail)-1], "line 99") {
		t.Errorf("tail should end at the last line, got %q", tail[len(tail)-1])
	}

	m.offset = m.maxOffset()
	top := m.visibleLines()
	if got, want := len(top), m.bodyHeight(); got != want {
		t.Fatalf("top: %d rows, want %d", got, want)
	}
	if !strings.Contains(top[0], "line 0") {
		t.Errorf("scrolled to top should start at the first line, got %q", top[0])
	}
}

func TestVisibleLines_shorterThanViewport(t *testing.T) {
	m := New().SetSize(100, 43)
	m = m.Open("web", makeLines(3))
	if got := len(m.visibleLines()); got != 3 {
		t.Errorf("%d rows, want 3", got)
	}
	if got := m.maxOffset(); got != 0 {
		t.Errorf("maxOffset = %d, want 0 when the buffer fits", got)
	}
}

func TestVisibleLines_truncatesByRunes(t *testing.T) {
	m := New().SetSize(12, 13)
	m = m.Open("web", []string{strings.Repeat("é", 40)})

	got := m.visibleLines()[0]
	if strings.Contains(got, "�") {
		t.Errorf("line cut mid-rune: %q", got)
	}
	if want := "  " + strings.Repeat("é", 9) + "…"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
