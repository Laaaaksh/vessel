package logs

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
)

func makeLines(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = fmt.Sprintf("line %d", i)
	}
	return out
}

func keyMsg(s string) tea.KeyPressMsg {
	r := rune(s[0])
	return tea.KeyPressMsg(tea.Key{Code: r, Text: s})
}

// spaceKey mirrors a real space bar press: KeyPressMsg.String() returns
// "space", never a literal space.
func spaceKey() tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Code: ' ', Text: " "})
}

// enterKey mirrors a real enter press (KeyPressMsg.String() == "enter").
func enterKey() tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter})
}

// The search prompt used to drop a space press (it arrives as "space") and
// every multi-byte rune (Key.String() is byte-lengthed), so a phrase could
// never be typed; backspace also trimmed single bytes, corrupting input.
func TestSearchQueryAcceptsSpacesAndUnicode(t *testing.T) {
	m := New().SetSize(100, 20).Open("web", []string{"alpha beta", "gamma", "alpha beta again"})
	m, _ = m.Update(keyMsg("/"))
	if !m.searching {
		t.Fatal("/ did not open the search prompt")
	}
	m, _ = m.Update(spaceKey())
	if m.query != " " {
		t.Fatalf("query = %q after space, want a literal space", m.query)
	}
	for _, r := range "beta" {
		m, _ = m.Update(keyMsg(string(r)))
	}
	m, _ = m.Update(keyMsg("é"))
	if m.query != " betaé" {
		t.Fatalf("query = %q, want the multi-byte rune preserved", m.query)
	}
	m, _ = m.Update(keyMsg("backspace"))
	if m.query != " beta" || !utf8.ValidString(m.query) {
		t.Fatalf("query = %q after backspace, want exactly the trailing rune removed", m.query)
	}
	m, _ = m.Update(enterKey())
	if len(m.matches) != 2 || m.searching {
		t.Fatalf("matches = %v searching = %v, want both beta lines found and the prompt closed", m.matches, m.searching)
	}
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
