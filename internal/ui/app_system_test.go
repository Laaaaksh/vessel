package ui

import (
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/Laaaaksh/vessel/internal/backend"
)

func tabKey() tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Code: tea.KeyTab})
}

func systemModel(t *testing.T) Model {
	t.Helper()
	m := New()
	m.cfg.MouseEnabled = true
	m.width, m.height = 120, 40
	m.activeView = ViewSystem
	m.client = backend.NewClientWithBinary(filepath.Join(t.TempDir(), "no-such-container-cli"))
	return m
}

// Tab cycling is the exact spot the sibling network-view branch also edits,
// so this pins the whole View enum to visit each sidebar view exactly once
// before repeating.
func TestTabCycling_visitsEachViewExactlyOnce(t *testing.T) {
	m := New()
	m.width, m.height = 100, 30
	seen := []View{m.activeView}
	for range viewCount {
		next, _ := m.handleKey(tabKey())
		m = next.(Model)
		seen = append(seen, m.activeView)
	}
	want := []View{ViewContainers, ViewImages, ViewVolumes, ViewSystem, ViewContainers}
	if len(seen) != len(want) {
		t.Fatalf("seen=%v want %v", seen, want)
	}
	for i := range want {
		if seen[i] != want[i] {
			t.Fatalf("seen=%v want %v", seen, want)
		}
	}
}

func TestNumericShortcut_selectsSystemView(t *testing.T) {
	m := New()
	m.width, m.height = 100, 30
	next, _ := m.handleKey(keyMsg("4"))
	m = next.(Model)
	if m.activeView != ViewSystem {
		t.Fatalf("activeView=%v want ViewSystem", m.activeView)
	}
}

// The sidebar's own up/down navigation uses separate arithmetic from Tab; it
// must wrap through all four views too, in both directions.
func TestSidebarNav_wrapsThroughAllFourViewsBothDirections(t *testing.T) {
	m := New()
	m.width, m.height = 100, 30
	m.focus = FocusSidebar

	seen := []View{m.activeView}
	for range viewCount {
		next, _ := m.handleKey(keyMsg("j"))
		m = next.(Model)
		seen = append(seen, m.activeView)
	}
	want := []View{ViewContainers, ViewImages, ViewVolumes, ViewSystem, ViewContainers}
	for i := range want {
		if seen[i] != want[i] {
			t.Fatalf("down: seen=%v want %v", seen, want)
		}
	}

	next, _ := m.handleKey(keyMsg("k"))
	m = next.(Model)
	if m.activeView != ViewSystem {
		t.Fatalf("up from Containers=%v want ViewSystem (wrap)", m.activeView)
	}
}

func TestActiveViewLoadCmd_systemReturnsASystemLoadedMsg(t *testing.T) {
	m := systemModel(t)
	cmd := m.activeViewLoadCmd()
	if cmd == nil {
		t.Fatal("expected a load command for ViewSystem")
	}
	if _, ok := cmd().(systemLoadedMsg); !ok {
		t.Fatalf("cmd() = %T, want systemLoadedMsg", cmd())
	}
}

func TestSystemView_mouseWheelMovesSystemPanelNotContainerPanel(t *testing.T) {
	m := systemModel(t)
	m.cntPanel = m.cntPanel.SetItems([]backend.Container{
		{ID: "1", Name: "web", Status: "running"},
		{ID: "2", Name: "db", Status: "running"},
	})
	cntCursorBefore := m.cntPanel.Cursor()

	next, _ := m.handleMouseWheel(tea.MouseWheelMsg(tea.Mouse{Button: tea.MouseWheelDown}))
	m = next.(Model)

	if m.sysPanel.Cursor() != 1 {
		t.Fatalf("sysPanel cursor=%d want 1", m.sysPanel.Cursor())
	}
	if m.cntPanel.Cursor() != cntCursorBefore {
		t.Fatalf("container panel cursor moved while System view was active: %d -> %d", cntCursorBefore, m.cntPanel.Cursor())
	}
}

func TestSystemView_mouseClickMovesSystemPanelNotContainerPanel(t *testing.T) {
	m := systemModel(t)
	m.cntPanel = m.cntPanel.SetItems([]backend.Container{{ID: "1", Name: "web", Status: "running"}})

	next, _ := m.handleMouseClick(tea.MouseClickMsg(tea.Mouse{Y: 4, Button: tea.MouseLeft}))
	m = next.(Model)

	if m.sysPanel.Cursor() != 2 {
		t.Fatalf("sysPanel cursor=%d want 2 (row=Y-2)", m.sysPanel.Cursor())
	}
}

// A filter left active on another panel before the user tabbed to System
// must not lock all of System's keys into that panel's filter-input routing.
func TestSystemView_panelFilteringIsAlwaysFalse(t *testing.T) {
	m := systemModel(t)
	m.cntPanel = m.cntPanel.SetItems([]backend.Container{{ID: "1", Name: "web", Status: "running"}})
	m.cntPanel, _ = m.cntPanel.Update(keyMsg("/"))
	if !m.cntPanel.Filtering() {
		t.Fatal("test setup: container panel should be filtering")
	}
	if m.panelFiltering() {
		t.Fatal("panelFiltering() must be false for ViewSystem even if another panel is mid-filter")
	}
}

func TestSystemView_yankCopiesSelectedRowText(t *testing.T) {
	m := systemModel(t)
	m.sysPanel = m.sysPanel.SetStatus(&backend.SystemStatus{Status: "running", Version: "v1.2.0"}, nil)
	next, _ := m.yankSelected()
	m = next.(Model)
	if !strings.Contains(m.status, "v1.2.0") {
		t.Fatalf("status=%q, want it to mention the copied version", m.status)
	}
}

func TestSystemView_viewNameAndCursorInfoUseTheSystemPanel(t *testing.T) {
	m := systemModel(t)
	if got := m.viewName(); got != "system" {
		t.Fatalf("viewName()=%q want system", got)
	}
	m.sysPanel = m.sysPanel.MoveBy(1)
	cur, n := m.cursorInfo()
	if cur != 1 || n != 4 {
		t.Fatalf("cursorInfo()=(%d,%d) want (1,4)", cur, n)
	}
}

// A custom command run from System view must not silently borrow whatever
// container the container panel happens to have selected in the background.
func TestSystemView_runCustomDoesNotLeakContainerSelection(t *testing.T) {
	m := systemModel(t)
	m.cntPanel = m.cntPanel.SetItems([]backend.Container{{ID: "abc123", Name: "web", Image: "nginx", Status: "running"}})
	m, _ = m.runCustom("echo {{.ID}}/{{.Name}}/{{.Image}}")
	if strings.Contains(m.status, "abc123") || strings.Contains(m.status, "web") || strings.Contains(m.status, "nginx") {
		t.Fatalf("custom command leaked the container panel's selection: %q", m.status)
	}
}
