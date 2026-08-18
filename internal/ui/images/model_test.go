package images

import (
	"fmt"
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

func imageWithID(id string) backend.Image {
	return backend.Image{ID: id, Repository: "docker.io/library/alpine", Tag: "latest", Size: 3848024}
}

func cachedInspect(id, digest string) *backend.ImageInspect {
	return &backend.ImageInspect{
		ID:         id,
		Repository: "docker.io/library/alpine",
		Tag:        "latest",
		Digest:     digest,
		Size:       3848024,
	}
}

func TestSetItems_dropsInspectWhenTagIsRepulled(t *testing.T) {
	m := New().SetItems([]backend.Image{imageWithID("oldid")})
	m = m.SetInspect(testRef, cachedInspect("oldid", "sha256:olddigest"), nil)
	if m.InspectedRef() != testRef {
		t.Fatalf("inspect not cached: %q", m.InspectedRef())
	}

	m = m.SetItems([]backend.Image{imageWithID("newid")})

	if got := m.InspectedRef(); got != "" {
		t.Errorf("inspect kept after re-pull: %q", got)
	}
	if v := ansi.Strip(m.DetailView(60, 40)); strings.Contains(v, "olddigest") {
		t.Errorf("stale digest still rendered after re-pull: %q", v)
	}
}

func TestSetItems_dropsInspectWhenImageIsGone(t *testing.T) {
	m := New().SetItems([]backend.Image{imageWithID("oldid")})
	m = m.SetInspect(testRef, cachedInspect("oldid", "sha256:olddigest"), nil)

	m = m.SetItems([]backend.Image{{ID: "other", Repository: "nginx", Tag: "1.27"}})

	if got := m.InspectedRef(); got != "" {
		t.Errorf("inspect kept after image removal: %q", got)
	}
}

func TestSetItems_keepsInspectForUnchangedImage(t *testing.T) {
	m := New().SetItems([]backend.Image{imageWithID("sameid")})
	m = m.SetInspect(testRef, cachedInspect("sameid", "sha256:livedigest"), nil)

	m = m.SetItems([]backend.Image{imageWithID("sameid")})

	if got := m.InspectedRef(); got != testRef {
		t.Errorf("inspect dropped for unchanged image: %q", got)
	}
	if v := ansi.Strip(m.DetailView(60, 40)); !strings.Contains(v, "sha256:lived") {
		t.Errorf("digest missing after unchanged refresh: %q", v)
	}
}

func TestSetInspect_lateResultForOtherImageKeepsCurrentCache(t *testing.T) {
	m := New().SetItems([]backend.Image{
		imageWithID("sameid"),
		{ID: "nginxid", Repository: "nginx", Tag: "1.27"},
	})
	m = m.SetInspect(testRef, cachedInspect("sameid", "sha256:livedigest"), nil)

	// A response for nginx (selected briefly, then left) arrives late.
	m = m.SetInspect("nginx:1.27", &backend.ImageInspect{ID: "nginxid", Digest: "sha256:nginxdigest"}, nil)

	if got := m.InspectedRef(); got != testRef {
		t.Errorf("late foreign result evicted the cache: %q", got)
	}
	v := ansi.Strip(m.DetailView(60, 40))
	if !strings.Contains(v, "sha256:lived") {
		t.Errorf("current image's digest lost: %q", v)
	}
	if strings.Contains(v, "nginxdigest") {
		t.Errorf("foreign digest rendered: %q", v)
	}
}

func manyEnv(n int) []string {
	env := make([]string, n)
	for i := range env {
		env[i] = fmt.Sprintf("VAR_NUMBER_%d=some-value", i)
	}
	return env
}

func TestDetailView_staysWithinHeightBudget(t *testing.T) {
	ins := cachedInspect("id1", "sha256:digest")
	ins.Env = manyEnv(12)
	ins.Platforms = []backend.ImagePlatform{
		{OS: "linux", Architecture: "arm64", Size: 5242880},
		{OS: "linux", Architecture: "amd64", Size: 2097152},
	}
	m := New().SetItems([]backend.Image{imageWithID("id1")}).SetInspect(testRef, ins, nil)

	const height = 20
	v := ansi.Strip(m.DetailView(60, height))

	assertFitsHeight(t, v, height)
	assertNoDanglingHeader(t, v)
	if !strings.Contains(v, "[p] pull") {
		t.Errorf("key hints pushed out of the pane: %q", v)
	}
}

func TestDetailView_rendersEverythingWhenItFits(t *testing.T) {
	ins := cachedInspect("id1", "sha256:digest")
	ins.Env = manyEnv(2)
	ins.Platforms = []backend.ImagePlatform{{OS: "linux", Architecture: "arm64", Size: 5242880}}
	m := New().SetItems([]backend.Image{imageWithID("id1")}).SetInspect(testRef, ins, nil)

	v := ansi.Strip(m.DetailView(60, 40))

	for _, want := range []string{"-- Env --", "VAR_NUMBER_0", "-- Platforms --", "linux/arm64"} {
		if !strings.Contains(v, want) {
			t.Errorf("detail missing %q at full height", want)
		}
	}
}

func assertFitsHeight(t *testing.T, v string, height int) {
	t.Helper()
	if got := strings.Count(v, "\n") + 1; got > height {
		t.Errorf("pane rendered %d lines into a %d-line budget", got, height)
	}
}

func assertNoDanglingHeader(t *testing.T, v string) {
	t.Helper()
	lines := strings.Split(v, "\n")
	for i, l := range lines {
		head := strings.TrimSpace(l)
		if !strings.HasPrefix(head, "--") {
			continue
		}
		next := ""
		if i+1 < len(lines) {
			next = strings.TrimSpace(lines[i+1])
		}
		if next == "" || strings.HasPrefix(next, "--") {
			t.Errorf("section header %q rendered with no rows under it", head)
		}
	}
}

func TestDetailView_fitsTheMinimumTerminalSize(t *testing.T) {
	ins := cachedInspect("id1", "sha256:e7a1a92a5bfeee40966aea60f0796b0e")
	ins.Cmd = []string{"/bin/sh"}
	ins.WorkingDir = "/"
	ins.LayerCount = 1
	ins.Env = manyEnv(4)
	ins.Platforms = []backend.ImagePlatform{{OS: "linux", Architecture: "arm64", Size: 5242880}}
	m := New().SetItems([]backend.Image{imageWithID("id1")}).SetInspect(testRef, ins, nil)

	// The app accepts a 60x12 terminal, which leaves the detail pane 18 columns
	// and 8 rows, so rows wrap as well as run long.
	for _, width := range []int{18, 38} {
		for _, height := range []int{4, 6, 8, 10} {
			v := ansi.Strip(m.DetailView(width, height))
			assertFitsHeight(t, v, height)
			assertNoDanglingHeader(t, v)
		}
	}
}

func TestDetailView_sectionsAndKeyHintsSurviveWrapping(t *testing.T) {
	// 40 columns is the pane width on any terminal wide enough to reach the
	// detailW cap, and both the key bar and the digest row wrap there.
	ins := cachedInspect("id1", "sha256:e7a1a92a5bfeee40966aea60f0796b0e")
	ins.Cmd = []string{"/bin/sh"}
	ins.WorkingDir = "/"
	ins.LayerCount = 1
	ins.Env = manyEnv(4)
	ins.Platforms = []backend.ImagePlatform{
		{OS: "linux", Architecture: "arm64", Size: 5242880},
		{OS: "linux", Architecture: "amd64", Size: 2097152},
	}
	m := New().SetItems([]backend.Image{imageWithID("id1")}).SetInspect(testRef, ins, nil)

	for _, height := range []int{12, 16, 20, 24} {
		v := ansi.Strip(m.DetailView(40, height))
		assertFitsHeight(t, v, height)
		assertNoDanglingHeader(t, v)
		// At this width the bar wraps but still fits alongside the image, so
		// it renders in full rather than being truncated.
		for _, want := range []string{"[p] pull", "[P]", "prune"} {
			if !strings.Contains(v, want) {
				t.Errorf("key hints clipped at height %d: %q in %q", height, want, v)
			}
		}
		for _, want := range []string{"docker.io/library/alpine:latest", "ID", "Size"} {
			if !strings.Contains(v, want) {
				t.Errorf("detail missing %q at height %d: %q", want, height, v)
			}
		}
	}
	// At least one of those sizes must actually render a section, or the
	// dangling-header assertion above proves nothing.
	if v := ansi.Strip(m.DetailView(40, 24)); !strings.Contains(v, "-- Env --") {
		t.Fatalf("no section rendered, the header assertions are vacuous: %q", v)
	}
}

func TestDetailView_narrowPaneStillIdentifiesTheImage(t *testing.T) {
	// 18x8 is the detail pane on the app's minimum 60x12 terminal, where the
	// key bar is wider than the pane and every row wraps.
	ins := cachedInspect("28bd5fe8b56d", "sha256:e7a1a92a5bfeee40966aea60f0796b0e")
	ins.Cmd = []string{"/bin/sh"}
	m := New().SetItems([]backend.Image{
		{ID: "28bd5fe8b56d", Repository: "docker.io/library/alpine", Tag: "latest", Size: 3848024},
	}).SetInspect(testRef, ins, nil)

	v := ansi.Strip(m.DetailView(18, 8))

	assertFitsHeight(t, v, 8)
	for _, want := range []string{"alpine", "ID", "28bd5fe8b56d"} {
		if !strings.Contains(v, want) {
			t.Errorf("narrow pane dropped %q for key hints: %q", want, v)
		}
	}
}

func TestDetailView_longValueRowsDoNotWrap(t *testing.T) {
	// A real digest is "sha256:" plus 64 hex characters, far wider than the
	// 40-column pane the app uses on any reasonably sized terminal.
	ins := cachedInspect("28bd5fe8b56d", "sha256:e7a1a92a5bfeee40966aea60f0796b0e6d4b2c1a9f8e7d6c5b4a39281706f5e4")
	ins.Cmd = []string{"/usr/local/bin/some-entrypoint --with --several --flags"}
	ins.WorkingDir = "/var/lib/some/deeply/nested/working/directory"
	m := New().SetItems([]backend.Image{
		{ID: "28bd5fe8b56d", Repository: "docker.io/library/alpine", Tag: "latest", Size: 3848024},
	}).SetInspect(testRef, ins, nil)

	for _, width := range []int{20, 40, 60} {
		v := ansi.Strip(m.DetailView(width, 20))
		for _, label := range []string{"Digest:", "Cmd:", "Workdir:"} {
			line := lineContaining(v, label)
			if line == "" {
				t.Fatalf("no %s row rendered at width %d: %q", label, width, v)
			}
			if strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), label)) == "" {
				t.Errorf("%s row wrapped its value onto the next row at width %d: %q", label, width, v)
			}
		}
	}
}

func lineContaining(v, want string) string {
	for _, l := range strings.Split(v, "\n") {
		if strings.Contains(l, want) {
			return l
		}
	}
	return ""
}

func TestDetailView_narrowestPaneStillIdentifiesTheImage(t *testing.T) {
	// The wide-list layout on a 60-column terminal leaves the detail pane 10
	// columns, narrower than the image reference itself.
	ins := cachedInspect("28bd5fe8b56d", "sha256:e7a1a92a5bfeee40966aea60f0796b0e")
	ins.Cmd = []string{"/bin/sh"}
	m := New().SetItems([]backend.Image{
		{ID: "28bd5fe8b56d", Repository: "docker.io/library/alpine", Tag: "latest", Size: 3848024},
	}).SetInspect(testRef, ins, nil)

	v := ansi.Strip(m.DetailView(10, 8))

	assertFitsHeight(t, v, 8)
	if !strings.Contains(v, "docker.i") {
		t.Errorf("the image reference must still render: %q", v)
	}
	if !strings.Contains(v, "ID") {
		t.Errorf("a wrapping title crowded the data rows out of the pane: %q", v)
	}
}
