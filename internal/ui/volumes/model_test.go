package volumes

import (
	"fmt"
	"strings"
	"testing"
	"time"
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

func vol(name string) backend.Volume {
	return backend.Volume{Name: name, Driver: "local"}
}

func TestVolumeSpaceTogglesMarkOnSelected(t *testing.T) {
	m := New().SetItems([]backend.Volume{vol("data"), vol("logs")})
	m, _ = m.Update(spaceKey())
	if got := m.marked["data"]; !got {
		t.Fatal("space should mark the selected volume")
	}
	m, _ = m.Update(spaceKey())
	if m.marked["data"] {
		t.Fatal("space on a marked volume should unmark it")
	}
}

func TestVolumeMarkedIDsFollowsSelection(t *testing.T) {
	m := New().SetItems([]backend.Volume{vol("data"), vol("logs"), vol("cache")})
	m, _ = m.Update(spaceKey())
	m, _ = m.Update(keyMsg("j"))
	m, _ = m.Update(spaceKey())
	want := []string{"data", "logs"}
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
func TestVolumeToggleMarkKeyIsConfigurable(t *testing.T) {
	m := New().SetItems([]backend.Volume{vol("data"), vol("logs")}).SetToggleMarkKey("m")
	m, _ = m.Update(spaceKey())
	if got := m.MarkedIDs(); len(got) != 0 {
		t.Fatalf("space still marks after rebinding: %v", got)
	}
	m, _ = m.Update(keyMsg("m"))
	if got := m.MarkedIDs(); len(got) != 1 || got[0] != "data" {
		t.Fatalf("MarkedIDs = %v, want [data] from the rebound key", got)
	}
}

func TestVolumeEmptyToggleMarkKeyKeepsTheDefault(t *testing.T) {
	m := New().SetItems([]backend.Volume{vol("data")}).SetToggleMarkKey("")
	m, _ = m.Update(spaceKey())
	if got := m.MarkedIDs(); len(got) != 1 || got[0] != "data" {
		t.Fatalf("MarkedIDs = %v, want [data]: an empty binding must not disable marking", got)
	}
}

func TestVolumeMarksDropWhenRefreshRemovesThem(t *testing.T) {
	m := New().SetItems([]backend.Volume{vol("data"), vol("logs")})
	m, _ = m.Update(spaceKey())
	// data is deleted underneath us; the next refresh no longer contains it.
	m = m.SetItems([]backend.Volume{vol("logs")})
	if got := m.MarkedIDs(); len(got) != 0 {
		t.Fatalf("stale mark survived refresh: %v", got)
	}
	// data is recreated under the same name: the old mark must not resurface,
	// whichever path removed it (delete, prune, another terminal).
	m = m.SetItems([]backend.Volume{vol("data"), vol("logs")})
	if got := m.MarkedIDs(); len(got) != 0 {
		t.Fatalf("mark resurfaced after the volume was recreated: %v", got)
	}
	if strings.Contains(m.ListView(80, 10), "*") {
		t.Fatal("recreated volume still renders a mark")
	}
}

func TestVolumeMarksSurviveARefreshThatKeepsThem(t *testing.T) {
	items := []backend.Volume{vol("data"), vol("logs")}
	m := New().SetItems(items)
	m, _ = m.Update(spaceKey())
	m = m.SetItems(items)
	got := m.MarkedIDs()
	if len(got) != 1 || got[0] != "data" {
		t.Fatalf("MarkedIDs = %v, want [data]", got)
	}
}

// A mark hidden by an active filter still tracks its own volume: it survives a
// refresh that keeps the volume, and dies with one that drops it.
func TestVolumeMarksHiddenByFilterStillTrackTheirVolume(t *testing.T) {
	m := New().SetItems([]backend.Volume{vol("data"), vol("logs")})
	m, _ = m.Update(spaceKey())
	m, _ = m.Update(keyMsg("j"))
	m, _ = m.Update(spaceKey())
	m = typeFilter(m, "data")
	m = m.SetItems([]backend.Volume{vol("data"), vol("logs")})
	m, _ = m.Update(keyMsg("esc"))
	if got := m.MarkedIDs(); len(got) != 2 {
		t.Fatalf("MarkedIDs = %v, want both marks kept across the refresh", got)
	}
	m = typeFilter(m, "data")
	m = m.SetItems([]backend.Volume{vol("data")})
	m, _ = m.Update(keyMsg("esc"))
	got := m.MarkedIDs()
	if len(got) != 1 || got[0] != "data" {
		t.Fatalf("MarkedIDs = %v, want [data]: the hidden logs mark outlived its volume", got)
	}
}

func TestVolumeMarksOnlySurfaceItemsInFilteredView(t *testing.T) {
	m := New().SetItems([]backend.Volume{vol("db-data"), vol("db-logs"), vol("cache")})
	// Mark all three, then filter down to "db": the cache mark must not surface.
	for range m.filtered {
		m, _ = m.Update(spaceKey())
		m, _ = m.Update(keyMsg("j"))
	}
	m = typeFilter(m, "db")
	got := m.MarkedIDs()
	if len(got) != 2 || got[0] != "db-data" || got[1] != "db-logs" {
		t.Fatalf("MarkedIDs under filter = %v, want [db-data db-logs]", got)
	}
}

func TestVolumeSpaceMarksFilteredSelection(t *testing.T) {
	m := New().SetItems([]backend.Volume{vol("db-data"), vol("cache")})
	m = typeFilter(m, "db")
	m.marked = nil // exercise lazy init
	m, _ = m.Update(spaceKey())
	if got := m.MarkedIDs(); len(got) != 1 || got[0] != "db-data" {
		t.Fatalf("space should mark the filtered selection, got %v", got)
	}
}

func TestVolumeListViewShowsMark(t *testing.T) {
	m := New().SetItems([]backend.Volume{vol("data"), vol("logs")})
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
// unmarked one both keep DRIVER under its header instead of sliding right.
func TestRenderRowAlignsWithHeader(t *testing.T) {
	m := New().SetItems([]backend.Volume{
		vol("data"),
		vol("a-very-long-volume-name-that-overflows-its-column"),
	})
	m, _ = m.Update(spaceKey())

	var header string
	var rows []string
	for _, ln := range strings.Split(ansi.Strip(m.ListView(100, 10)), "\n") {
		switch {
		case strings.Contains(ln, "DRIVER"):
			header = ln
		case strings.Contains(ln, "local"):
			rows = append(rows, ln)
		}
	}
	if header == "" || len(rows) != 2 {
		t.Fatalf("expected a header and 2 rows, got header=%q rows=%v", header, rows)
	}
	want := column(t, header, "DRIVER")
	for i, r := range rows {
		if got := column(t, r, "local"); got != want {
			t.Errorf("row %d: driver at column %d, header DRIVER at %d", i, got, want)
		}
	}
}

var created = time.Date(2026, 8, 5, 16, 8, 41, 0, time.UTC)

func volumeRow(size uint64, at time.Time) backend.Volume {
	return backend.Volume{Name: "data", Driver: "local", SizeBytes: size, Created: at}
}

func volumeInspect(size uint64, at time.Time) *backend.VolumeInspect {
	return &backend.VolumeInspect{
		Name:      "data",
		Driver:    "local",
		SizeBytes: size,
		Created:   at,
		Format:    "ext4",
	}
}

func TestSetItems_dropsInspectWhenVolumeChanged(t *testing.T) {
	m := New().SetItems([]backend.Volume{volumeRow(1<<30, created)})
	m = m.SetInspect("data", volumeInspect(1<<30, created), nil)
	if m.InspectedName() != "data" {
		t.Fatalf("inspect not cached: %q", m.InspectedName())
	}

	m = m.SetItems([]backend.Volume{volumeRow(2<<30, created)})

	if got := m.InspectedName(); got != "" {
		t.Errorf("inspect kept after the volume was resized: %q", got)
	}
	if v := ansi.Strip(m.DetailView(60, 40)); strings.Contains(v, "ext4") {
		t.Errorf("stale inspect still rendered after resize: %q", v)
	}
}

func TestSetItems_dropsInspectWhenVolumeRecreated(t *testing.T) {
	m := New().SetItems([]backend.Volume{volumeRow(1<<30, created)})
	m = m.SetInspect("data", volumeInspect(1<<30, created), nil)

	m = m.SetItems([]backend.Volume{volumeRow(1<<30, created.Add(time.Hour))})

	if got := m.InspectedName(); got != "" {
		t.Errorf("inspect kept after the volume was recreated: %q", got)
	}
}

func TestSetItems_keepsInspectForUnchangedVolume(t *testing.T) {
	m := New().SetItems([]backend.Volume{volumeRow(1<<30, created)})
	m = m.SetInspect("data", volumeInspect(1<<30, created), nil)

	m = m.SetItems([]backend.Volume{volumeRow(1<<30, created)})

	if got := m.InspectedName(); got != "data" {
		t.Errorf("inspect dropped for unchanged volume: %q", got)
	}
	if v := ansi.Strip(m.DetailView(60, 40)); !strings.Contains(v, "ext4") {
		t.Errorf("format missing after unchanged refresh: %q", v)
	}
}

func TestSetInspect_lateResultForOtherVolumeKeepsCurrentCache(t *testing.T) {
	m := New().SetItems([]backend.Volume{
		volumeRow(1<<30, created),
		{Name: "cache", Driver: "local"},
	})
	m = m.SetInspect("data", volumeInspect(1<<30, created), nil)

	m = m.SetInspect("cache", &backend.VolumeInspect{Name: "cache", Format: "xfs"}, nil)

	if got := m.InspectedName(); got != "data" {
		t.Errorf("late foreign result evicted the cache: %q", got)
	}
	v := ansi.Strip(m.DetailView(60, 40))
	if !strings.Contains(v, "ext4") {
		t.Errorf("current volume's format lost: %q", v)
	}
	if strings.Contains(v, "xfs") {
		t.Errorf("foreign format rendered: %q", v)
	}
}

func manyPairs(n int, prefix string) map[string]string {
	out := make(map[string]string, n)
	for i := range n {
		out[fmt.Sprintf("%s_%02d", prefix, i)] = "some-value"
	}
	return out
}

func TestDetailView_staysWithinHeightBudget(t *testing.T) {
	ins := volumeInspect(1<<30, created)
	ins.Labels = manyPairs(12, "label")
	ins.Options = manyPairs(4, "option")
	m := New().SetItems([]backend.Volume{volumeRow(1<<30, created)}).SetInspect("data", ins, nil)

	const height = 20
	v := ansi.Strip(m.DetailView(60, height))

	if got := strings.Count(v, "\n") + 1; got > height {
		t.Errorf("pane rendered %d lines into a %d-line budget", got, height)
	}
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
	if !strings.Contains(v, "[c] create") {
		t.Errorf("key hints pushed out of the pane: %q", v)
	}
}

func TestDetailView_sectionRowsAreStablyOrdered(t *testing.T) {
	ins := volumeInspect(1<<30, created)
	ins.Labels = manyPairs(6, "label")
	m := New().SetItems([]backend.Volume{volumeRow(1<<30, created)}).SetInspect("data", ins, nil)

	first := ansi.Strip(m.DetailView(60, 40))
	for range 5 {
		if got := ansi.Strip(m.DetailView(60, 40)); got != first {
			t.Fatal("label rows render in a different order between calls")
		}
	}
}

func TestDetailView_fitsTheMinimumTerminalSize(t *testing.T) {
	ins := volumeInspect(1<<30, created)
	ins.Labels = manyPairs(3, "label")
	ins.Options = manyPairs(2, "option")
	m := New().SetItems([]backend.Volume{volumeRow(1<<30, created)}).SetInspect("data", ins, nil)

	for _, width := range []int{18, 38} {
		for _, height := range []int{4, 6, 8, 10} {
			v := ansi.Strip(m.DetailView(width, height))
			if got := strings.Count(v, "\n") + 1; got > height {
				t.Errorf("pane rendered %d lines into %dx%d", got, width, height)
			}
		}
	}
}

func TestDetailView_sectionsAndKeyHintsSurviveWrapping(t *testing.T) {
	ins := volumeInspect(1<<30, created)
	ins.Labels = manyPairs(4, "label")
	ins.Options = manyPairs(2, "option")
	m := New().SetItems([]backend.Volume{volumeRow(1<<30, created)}).SetInspect("data", ins, nil)

	for _, height := range []int{12, 16, 20, 24} {
		v := ansi.Strip(m.DetailView(40, height))
		if got := strings.Count(v, "\n") + 1; got > height {
			t.Errorf("pane rendered %d lines into 40x%d", got, height)
		}
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
				t.Errorf("section header %q rendered with no rows under it at height %d", head, height)
			}
		}
		// At this width the bar wraps but still fits alongside the volume, so
		// it renders in full rather than being truncated.
		for _, want := range []string{"[c] create", "[y]", "yank path"} {
			if !strings.Contains(v, want) {
				t.Errorf("key hints clipped at height %d: %q in %q", height, want, v)
			}
		}
		for _, want := range []string{"data", "Driver"} {
			if !strings.Contains(v, want) {
				t.Errorf("detail missing %q at height %d: %q", want, height, v)
			}
		}
	}
	if v := ansi.Strip(m.DetailView(40, 24)); !strings.Contains(v, "-- Labels --") {
		t.Fatalf("no section rendered, the header assertions are vacuous: %q", v)
	}
}

func TestDetailView_narrowPaneStillIdentifiesTheVolume(t *testing.T) {
	ins := volumeInspect(1<<30, created)
	m := New().SetItems([]backend.Volume{volumeRow(1<<30, created)}).SetInspect("data", ins, nil)

	const width, height = 18, 6
	v := ansi.Strip(m.DetailView(width, height))

	if got := strings.Count(v, "\n") + 1; got > height {
		t.Errorf("pane rendered %d lines into %dx%d", got, width, height)
	}
	for _, want := range []string{"data", "Driver", "local"} {
		if !strings.Contains(v, want) {
			t.Errorf("narrow pane dropped %q for key hints: %q", want, v)
		}
	}
}

func TestDetailView_narrowestPaneStillIdentifiesTheVolume(t *testing.T) {
	ins := volumeInspect(1<<30, created)
	m := New().SetItems([]backend.Volume{
		{Name: "a-rather-long-volume-name", Driver: "local", SizeBytes: 1 << 30, Created: created},
	}).SetInspect("a-rather-long-volume-name", ins, nil)

	v := ansi.Strip(m.DetailView(10, 8))

	if got := strings.Count(v, "\n") + 1; got > 8 {
		t.Errorf("pane rendered %d lines into 10x8", got)
	}
	if !strings.Contains(v, "a-rather") {
		t.Errorf("the volume name must still render: %q", v)
	}
	if !strings.Contains(v, "Driver") {
		t.Errorf("a wrapping title crowded the data rows out of the pane: %q", v)
	}
}
