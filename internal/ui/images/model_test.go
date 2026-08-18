package images

import (
	"strings"
	"testing"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/Laaaaksh/vessel/internal/backend"
)

func keyMsg(s string) tea.KeyPressMsg {
	if s == "esc" {
		return tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape})
	}
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

// typeFilter opens the filter prompt, types s one character per key press
// (mirroring the real UI), then applies it with enter so subsequent keys act
// on the filtered selection rather than the filter input.
func typeFilter(m Model, s string) Model {
	m, _ = m.Update(keyMsg("/"))
	for _, r := range s {
		m, _ = m.Update(keyMsg(string(r)))
	}
	m, _ = m.Update(enterKey())
	return m
}

func img(id, repo string) backend.Image {
	return backend.Image{ID: id, Repository: repo, Tag: "latest"}
}

func TestImageSpaceTogglesMarkOnSelected(t *testing.T) {
	m := New().SetItems([]backend.Image{img("a", "alpine"), img("b", "busybox")})
	m, _ = m.Update(spaceKey())
	if got := m.MarkedIDs(); len(got) != 1 || got[0] != "a" {
		t.Fatalf("space should mark the selected image, got %v", got)
	}
	m, _ = m.Update(spaceKey())
	if got := m.MarkedIDs(); len(got) != 0 {
		t.Fatalf("space on a marked image should unmark it, got %v", got)
	}
}

func TestImageMarkedIDsFollowsSelection(t *testing.T) {
	m := New().SetItems([]backend.Image{img("a", "alpine"), img("b", "busybox"), img("c", "debian")})
	m, _ = m.Update(spaceKey())
	m, _ = m.Update(keyMsg("j"))
	m, _ = m.Update(spaceKey())
	want := []string{"a", "b"}
	got := m.MarkedIDs()
	if len(got) != len(want) {
		t.Fatalf("MarkedIDs = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("MarkedIDs = %v, want %v (order must follow the list)", got, want)
		}
	}
}

// The app hands the panel its binding, so a rebound mark key has to reach it
// and the old one has to stop working.
func TestImageToggleMarkKeyIsConfigurable(t *testing.T) {
	m := New().SetItems([]backend.Image{img("a", "alpine"), img("b", "busybox")}).SetToggleMarkKey("m")
	m, _ = m.Update(spaceKey())
	if got := m.MarkedIDs(); len(got) != 0 {
		t.Fatalf("space still marks after rebinding: %v", got)
	}
	m, _ = m.Update(keyMsg("m"))
	if got := m.MarkedIDs(); len(got) != 1 || got[0] != "a" {
		t.Fatalf("MarkedIDs = %v, want [a] from the rebound key", got)
	}
}

func TestImageEmptyToggleMarkKeyKeepsTheDefault(t *testing.T) {
	m := New().SetItems([]backend.Image{img("a", "alpine")}).SetToggleMarkKey("")
	m, _ = m.Update(spaceKey())
	if got := m.MarkedIDs(); len(got) != 1 || got[0] != "a" {
		t.Fatalf("MarkedIDs = %v, want [a]: an empty binding must not disable marking", got)
	}
}

func TestImageMarksDropWhenRefreshRemovesThem(t *testing.T) {
	m := New().SetItems([]backend.Image{img("a", "alpine"), img("b", "busybox")})
	m, _ = m.Update(spaceKey())
	// Alpine is deleted underneath us; the next refresh no longer contains it.
	m = m.SetItems([]backend.Image{img("b", "busybox")})
	if got := m.MarkedIDs(); len(got) != 0 {
		t.Fatalf("stale mark survived refresh: %v", got)
	}
	// Alpine is pulled again under the same digest: the old mark must not
	// resurface, whichever path removed it (delete, prune, another terminal).
	m = m.SetItems([]backend.Image{img("a", "alpine"), img("b", "busybox")})
	if got := m.MarkedIDs(); len(got) != 0 {
		t.Fatalf("mark resurfaced after the image was recreated: %v", got)
	}
	if strings.Contains(m.ListView(80, 10), "*") {
		t.Fatal("recreated image still renders a mark")
	}
}

func TestImageMarksSurviveARefreshThatKeepsThem(t *testing.T) {
	items := []backend.Image{img("a", "alpine"), img("b", "busybox")}
	m := New().SetItems(items)
	m, _ = m.Update(spaceKey())
	m = m.SetItems(items)
	got := m.MarkedIDs()
	if len(got) != 1 || got[0] != "a" {
		t.Fatalf("MarkedIDs = %v, want [a]", got)
	}
}

// Two references can resolve to one digest, so marks key on digest+reference:
// marking alpine:latest must not also mark alpine:3.22.
func TestImageSharedDigestMarksOneRowAtATime(t *testing.T) {
	m := New().SetItems([]backend.Image{
		{ID: "sha256:same", Repository: "alpine", Tag: "latest"},
		{ID: "sha256:same", Repository: "alpine", Tag: "3.22"},
	})
	m, _ = m.Update(spaceKey())
	if n := strings.Count(m.ListView(80, 10), "*"); n != 1 {
		t.Fatalf("%d rows marked, want 1: one press must mark one row", n)
	}
	if got := m.MarkedIDs(); len(got) != 1 || got[0] != "sha256:same" {
		t.Fatalf("MarkedIDs = %v, want [sha256:same]", got)
	}
	// Marking the second reference too must not repeat the digest: the delete
	// takes digests, and `image delete <d> <d>` fails on its second argument.
	m, _ = m.Update(keyMsg("j"))
	m, _ = m.Update(spaceKey())
	if got := m.MarkedIDs(); len(got) != 1 || got[0] != "sha256:same" {
		t.Fatalf("MarkedIDs = %v, want the digest exactly once", got)
	}
}

// Dangling images all render as an empty reference, so the reference alone is
// not a usable key either: each row keeps its own mark.
func TestImageDanglingRowsMarkIndependently(t *testing.T) {
	m := New().SetItems([]backend.Image{
		{ID: "sha256:one"},
		{ID: "sha256:two"},
	})
	m, _ = m.Update(spaceKey())
	if n := strings.Count(m.ListView(80, 10), "*"); n != 1 {
		t.Fatalf("%d dangling rows marked, want 1", n)
	}
	if got := m.MarkedIDs(); len(got) != 1 || got[0] != "sha256:one" {
		t.Fatalf("MarkedIDs = %v, want [sha256:one]", got)
	}
}

func TestImageMarksOnlySurfaceItemsInFilteredView(t *testing.T) {
	m := New().SetItems([]backend.Image{img("a", "alpine"), img("b", "busybox")})
	// Mark both while unfiltered.
	m, _ = m.Update(spaceKey())
	m, _ = m.Update(keyMsg("j"))
	m, _ = m.Update(spaceKey())
	// Filter down to alpine only: busybox is hidden, so its mark must not appear.
	m = typeFilter(m, "alp")
	got := m.MarkedIDs()
	if len(got) != 1 || got[0] != "a" {
		t.Fatalf("MarkedIDs under filter = %v, want [a]", got)
	}
}

func TestImageSpaceMarksFilteredSelection(t *testing.T) {
	m := New().SetItems([]backend.Image{img("a", "alpine"), img("b", "busybox")})
	m = typeFilter(m, "alp")
	m.marked = nil // exercise lazy init
	m, _ = m.Update(spaceKey())
	if got := m.MarkedIDs(); len(got) != 1 || got[0] != "a" {
		t.Fatalf("space should mark the filtered selection, got %v", got)
	}
}

func TestImageListViewShowsMark(t *testing.T) {
	m := New().SetItems([]backend.Image{img("a", "alpine"), img("b", "busybox")})
	m, _ = m.Update(spaceKey())
	v := m.ListView(60, 10)
	if !strings.Contains(v, "*") {
		t.Fatalf("expected a mark column in list view, got %q", v)
	}
}

// column returns the rune offset of sub within s, ignoring styling escapes.
func column(t *testing.T, s, sub string) int {
	t.Helper()
	i := strings.Index(s, sub)
	if i < 0 {
		t.Fatalf("%q not found in %q", sub, s)
	}
	return utf8.RuneCountInString(s[:i])
}

// The mark cell is absorbed into the first column, so a marked row and an
// unmarked one both keep SIZE under its header instead of sliding right.
func TestRenderRowAlignsWithHeader(t *testing.T) {
	m := New().SetItems([]backend.Image{
		{ID: "a", Repository: "alpine", Tag: "latest", Size: 3 * 1024 * 1024},
		{ID: "b", Repository: "a-very-long-registry.example.com/team/service", Tag: "v1", Size: 3 * 1024 * 1024},
	})
	m, _ = m.Update(spaceKey())

	var header string
	var rows []string
	for _, ln := range strings.Split(ansi.Strip(m.ListView(100, 10)), "\n") {
		switch {
		case strings.Contains(ln, "REPOSITORY:TAG"):
			header = ln
		case strings.Contains(ln, "MiB"):
			rows = append(rows, ln)
		}
	}
	if header == "" || len(rows) != 2 {
		t.Fatalf("expected a header and 2 rows, got header=%q rows=%v", header, rows)
	}
	want := column(t, header, "SIZE")
	for i, r := range rows {
		if got := column(t, r, "3 MiB"); got != want {
			t.Errorf("row %d: size at column %d, header SIZE at %d", i, got, want)
		}
	}
}
