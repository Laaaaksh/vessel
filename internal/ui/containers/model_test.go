package containers

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/Laaaaksh/vessel/internal/backend"
)

// spaceKey mirrors a real space bar press: KeyPressMsg.String() returns
// "space", never a literal space.
func spaceKey() tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Code: ' ', Text: " "})
}

func cnt(id, name string) backend.Container {
	return backend.Container{ID: id, Name: name, Status: "running"}
}

// A container removed by any path - confirmed delete, prune, or a docker rm in
// another terminal - reaches the panel as a refresh without it. Its mark must
// not survive that, or recreating the id re-arms a delete nobody asked for.
func TestContainerMarksDropWhenRefreshRemovesThem(t *testing.T) {
	items := []backend.Container{cnt("1", "web"), cnt("2", "db")}
	m := New().SetItems(items)
	m, _ = m.Update(spaceKey())
	m = m.SetItems(nil)
	m = m.SetItems(items)
	if got := m.MarkedIDs(); len(got) != 0 {
		t.Fatalf("mark resurfaced after the container was recreated: %v", got)
	}
	if m.marked["1"] {
		t.Fatal("stale mark still held in the map")
	}
}

func TestContainerMarksSurviveARefreshThatKeepsThem(t *testing.T) {
	items := []backend.Container{cnt("1", "web"), cnt("2", "db")}
	m := New().SetItems(items)
	m, _ = m.Update(spaceKey())
	m = m.SetItems(items)
	got := m.MarkedIDs()
	if len(got) != 1 || got[0] != "1" {
		t.Fatalf("MarkedIDs = %v, want [1]", got)
	}
}
