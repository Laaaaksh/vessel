package volumes

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

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

func TestVolumeClearMarks(t *testing.T) {
	m := New().SetItems([]backend.Volume{vol("data"), vol("logs")})
	m, _ = m.Update(spaceKey())
	m, _ = m.Update(keyMsg("j"))
	m, _ = m.Update(spaceKey())
	m = m.ClearMarks()
	if got := m.MarkedIDs(); len(got) != 0 {
		t.Fatalf("ClearMarks left marks: %v", got)
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
