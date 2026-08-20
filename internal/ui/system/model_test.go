package system

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/charmbracelet/x/ansi"

	"github.com/Laaaaksh/vessel/internal/backend"
)

func keyMsg(s string) tea.KeyPressMsg {
	r := rune(s[0])
	return tea.KeyPressMsg(tea.Key{Code: r, Text: s})
}

// detailPaneW/detailPaneH are the actual detail-pane dimensions app.go's
// layoutDims computes at the smallest supported terminal, 60x12: sidebarW=18,
// detailW=min(42, 60/3)=20, bodyH=12-2=10, then the pane border costs 2 more
// each way. Testing at the full 60x12 terminal size instead of this hides
// wrapping bugs that only show up in the pane's real, much narrower width.
const (
	detailPaneW = 18
	detailPaneH = 8
)

func runningStatus() *backend.SystemStatus {
	return &backend.SystemStatus{
		Status:  "running",
		Version: "container-apiserver version 1.2.0 (build: release, commit: unspeci)",
	}
}

func downStatus() *backend.SystemStatus {
	return &backend.SystemStatus{Status: "unregistered"}
}

func fullUsage() *backend.DiskUsage {
	return &backend.DiskUsage{
		Containers: backend.DiskUsageCategory{Total: 3, Active: 0, SizeBytes: 4122652672, ReclaimableBytes: 4122652672},
		Images:     backend.DiskUsageCategory{Total: 2, Active: 1, SizeBytes: 9634656256, ReclaimableBytes: 257626112},
		Volumes:    backend.DiskUsageCategory{Total: 1, Active: 0, SizeBytes: 69390336, ReclaimableBytes: 69390336},
	}
}

func TestCursorMovesThroughAllFourRows(t *testing.T) {
	m := New()
	if m.Len() != 4 {
		t.Fatalf("Len() = %d, want 4", m.Len())
	}
	m, _ = m.Update(keyMsg("j"))
	m, _ = m.Update(keyMsg("j"))
	m, _ = m.Update(keyMsg("j"))
	if m.Cursor() != 3 {
		t.Fatalf("Cursor() = %d, want 3", m.Cursor())
	}
	// Clamps at the last row like every other panel's cursor; it does not wrap.
	m, _ = m.Update(keyMsg("j"))
	if m.Cursor() != 3 {
		t.Fatalf("Cursor() = %d, want 3 (clamped)", m.Cursor())
	}
	m, _ = m.Update(keyMsg("G"))
	if m.Cursor() != 3 {
		t.Fatalf("Cursor() after G = %d, want 3", m.Cursor())
	}
	m, _ = m.Update(keyMsg("g"))
	if m.Cursor() != 0 {
		t.Fatalf("Cursor() after g = %d, want 0", m.Cursor())
	}
}

func TestServicesDownIsFalseUntilAStatusArrives(t *testing.T) {
	m := New()
	if m.ServicesDown() {
		t.Fatal("ServicesDown() should be false before any poll result")
	}
}

func TestServicesDownReflectsTheLatestStatus(t *testing.T) {
	m := New().SetStatus(downStatus(), nil)
	if !m.ServicesDown() {
		t.Fatal("ServicesDown() should be true for an unregistered status")
	}
	m = m.SetStatus(runningStatus(), nil)
	if m.ServicesDown() {
		t.Fatal("ServicesDown() should be false once services report running")
	}
}

func TestListViewShowsRunningStatusAndUsage(t *testing.T) {
	m := New().SetStatus(runningStatus(), nil).SetDiskUsage(fullUsage(), nil)
	v := ansi.Strip(m.ListView(80, 10))
	for _, want := range []string{"Service", "running", "Containers", "3 total", "Images", "2 total", "Volumes", "1 total"} {
		if !strings.Contains(v, want) {
			t.Errorf("list missing %q: %q", want, v)
		}
	}
}

// The services-down state is this view's own subject, not a failure to
// report on it, so nothing here may read as an error.
func TestListViewRendersServicesDownAsANormalState(t *testing.T) {
	m := New().SetStatus(downStatus(), nil).
		SetDiskUsage(nil, errors.New(`Error: interrupted: "XPC connection error: Connection invalid"`))
	v := ansi.Strip(m.ListView(80, 10))
	if !strings.Contains(v, "not running") {
		t.Errorf("list missing the not-running status: %q", v)
	}
	if !strings.Contains(v, "services not running") {
		t.Errorf("disk rows should read as the known down state: %q", v)
	}
	if strings.Contains(v, "XPC") || strings.Contains(v, "Error:") {
		t.Errorf("raw df error leaked into the down state: %q", v)
	}
}

func TestDetailViewServiceRowShowsIdentity(t *testing.T) {
	m := New().SetStatus(&backend.SystemStatus{
		Status:      "running",
		Version:     "container-apiserver version 1.2.0",
		AppRoot:     "/Users/laksh/Library/Application Support/com.apple.container/",
		InstallRoot: "/opt/homebrew/Cellar/container/1.2.0/",
	}, nil)
	v := ansi.Strip(m.DetailView(60, 40))
	for _, want := range []string{"Service", "Status", "running", "Version", "1.2.0", "App root", "Install"} {
		if !strings.Contains(v, want) {
			t.Errorf("detail missing %q: %q", want, v)
		}
	}
}

func TestDetailViewCategoryRowShowsBreakdown(t *testing.T) {
	m := New().SetStatus(runningStatus(), nil).SetDiskUsage(fullUsage(), nil).MoveBy(1)
	v := ansi.Strip(m.DetailView(60, 40))
	for _, want := range []string{"Containers", "Total", "3", "Active", "Size", "Reclaim"} {
		if !strings.Contains(v, want) {
			t.Errorf("detail missing %q: %q", want, v)
		}
	}
}

// Identity rows (Status, Version, App root, Install) must survive the
// smallest supported terminal size: detail panes here have a documented
// habit of dropping rows when short, and a long, unbounded Version value is
// exactly the kind of row that blows the pane's budget and drops everything
// after it if it is not truncated to fit.
func TestDetailViewIdentitySurvivesAtSmallestSupportedSize(t *testing.T) {
	m := New().SetStatus(runningStatus(), nil)
	v := ansi.Strip(m.DetailView(detailPaneW, detailPaneH))
	for _, want := range []string{"Status", "running", "Version", "App root", "Install"} {
		if !strings.Contains(v, want) {
			t.Errorf("identity row %q dropped at the smallest supported pane size: %q", want, v)
		}
	}
}

func TestDetailViewCategoryRowAtServicesDownExplainsWhyNotBlank(t *testing.T) {
	m := New().SetStatus(downStatus(), nil).
		SetDiskUsage(nil, errors.New("boom")).MoveBy(2)
	v := ansi.Strip(m.DetailView(60, 40))
	if !strings.Contains(v, "services not running") {
		t.Errorf("expected an explanation, got %q", v)
	}
	if strings.Contains(v, "boom") {
		t.Errorf("raw df error should not surface once the down state is known: %q", v)
	}
}

func TestDetailViewSurfacesAnUnexpectedUsageErrorWhileRunning(t *testing.T) {
	m := New().SetStatus(runningStatus(), nil).
		SetDiskUsage(nil, errors.New("permission denied")).MoveBy(1)
	v := ansi.Strip(m.DetailView(60, 40))
	if !strings.Contains(v, "permission denied") {
		t.Errorf("expected the real error surfaced, got %q", v)
	}
}

// KV's label column is a fixed 9 characters including the colon; a longer
// label wraps onto a second rendered row and drags its value down with it,
// which would corrupt every row below it in the pane's fixed row budget.
func TestDetailViewLabelsFitOneRenderedRow(t *testing.T) {
	m := New().SetStatus(&backend.SystemStatus{
		Status: "running", Version: "v1", AppRoot: "/a", InstallRoot: "/b",
	}, nil)
	v := ansi.Strip(m.DetailView(60, 40))
	for _, line := range strings.Split(v, "\n") {
		if strings.Contains(line, "Install") && !strings.Contains(line, "/b") {
			t.Errorf("Install label wrapped away from its value: %q", line)
		}
	}

	cat := New().SetStatus(runningStatus(), nil).SetDiskUsage(fullUsage(), nil).MoveBy(3)
	v = ansi.Strip(cat.DetailView(60, 40))
	for _, line := range strings.Split(v, "\n") {
		if strings.Contains(line, "Reclaim") && !strings.HasSuffix(strings.TrimRight(line, " "), "B") {
			t.Errorf("Reclaim label wrapped away from its value: %q", line)
		}
	}
}

// A KV row's value is unbounded by default: KVFit is required to keep a long
// one to a single rendered row. Getting that wrong at the pane's real narrow
// width drops the row (and everything after it) instead of merely looking
// ugly, so this is checked at detailPaneW/H rather than a roomy width.
func TestDetailViewLongVersionDoesNotWrapOrDropLaterRows(t *testing.T) {
	m := New().SetStatus(runningStatus(), nil)
	v := ansi.Strip(m.DetailView(detailPaneW, detailPaneH))
	lines := strings.Split(v, "\n")
	var versionLines int
	for _, line := range lines {
		if strings.Contains(line, "Version") {
			versionLines++
		}
	}
	if versionLines != 1 {
		t.Fatalf("expected the Version row on exactly one rendered line, got %d: %q", versionLines, v)
	}
	if !strings.Contains(v, "App root") {
		t.Errorf("App root dropped after a long Version row: %q", v)
	}
}

// The list row's value column is unbounded by default too: a long Service
// summary must not wrap and push the Containers/Images/Volumes rows below it
// out of the pane at a narrow width.
func TestListViewLongServiceSummaryDoesNotPushOtherRowsOut(t *testing.T) {
	m := New().SetStatus(runningStatus(), nil).SetDiskUsage(fullUsage(), nil)
	v := ansi.Strip(m.ListView(20, detailPaneH))
	for _, want := range []string{"Service", "Containers", "Images", "Volumes"} {
		if !strings.Contains(v, want) {
			t.Errorf("row %q pushed out of the list pane by a wrapped Service row: %q", want, v)
		}
	}
}

func TestYankTextReturnsVersionForServiceRow(t *testing.T) {
	m := New().SetStatus(runningStatus(), nil)
	if got := m.YankText(); got != runningStatus().Version {
		t.Errorf("YankText() = %q", got)
	}
}

func TestYankTextReturnsSizeForCategoryRow(t *testing.T) {
	m := New().SetDiskUsage(fullUsage(), nil).MoveBy(2)
	if got := m.YankText(); got == "" {
		t.Error("YankText() empty for images row")
	}
}
