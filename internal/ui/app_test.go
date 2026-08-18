package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/Laaaaksh/vessel/internal/backend"
)

func TestView_shellModeEmpty(t *testing.T) {
	m := New()
	m.width = 80
	m.height = 24
	m.mode = modeShell
	v := m.View()
	if strings.TrimSpace(viewString(v)) != "" {
		t.Fatalf("shell mode View must be empty, got %q", viewString(v))
	}
	if !v.AltScreen {
		t.Fatal("shell mode should keep AltScreen")
	}
}

func TestSelection_listContainsRows(t *testing.T) {
	m := New()
	m.width = 120
	m.height = 40
	m.cntPanel = m.cntPanel.SetItems([]backend.Container{
		{ID: "1", Name: "web", Status: "running"},
		{ID: "2", Name: "db", Status: "stopped"},
	})
	row := m.cntPanel.ListView(80, 20, nil)
	plain := ansi.Strip(row)
	if !strings.Contains(plain, "web") || !strings.Contains(plain, "db") {
		t.Fatalf("expected both rows, got %q", plain)
	}
}

func TestFocusKeys(t *testing.T) {
	m := New()
	m.width, m.height = 100, 30
	m.focus = FocusList
	next, _ := m.handleKey(keyMsg("l"))
	mm := next.(Model)
	if mm.focus != FocusDetail {
		t.Fatalf("focus after l: got %v want detail", mm.focus)
	}
	next, _ = mm.handleKey(keyMsg("h"))
	mm = next.(Model)
	if mm.focus != FocusList {
		t.Fatalf("focus after h: got %v want list", mm.focus)
	}
}

func TestConfirmModalMode(t *testing.T) {
	m := New()
	m.width, m.height = 100, 30
	m.cntPanel = m.cntPanel.SetItems([]backend.Container{{ID: "abc", Name: "web", Status: "running"}})
	next, _ := m.handleKey(keyMsg("d"))
	m = next.(Model)
	if m.mode != modeConfirmDelete {
		t.Fatalf("mode=%v want confirm", m.mode)
	}
	v := m.View()
	if !strings.Contains(ansi.Strip(viewString(v)), "Delete") {
		t.Fatalf("confirm modal missing from view: %q", ansi.Strip(viewString(v)))
	}
	next, _ = m.handleKey(keyMsg("n"))
	m = next.(Model)
	if m.mode != modeBrowse {
		t.Fatalf("cancel should return to browse, got %v", m.mode)
	}
}

func TestImageBulkDeleteConfirm(t *testing.T) {
	m := New()
	m.width, m.height = 100, 30
	m.activeView = ViewImages
	m.imgPanel = m.imgPanel.SetItems([]backend.Image{
		{ID: "a", Repository: "alpine", Tag: "latest"},
		{ID: "b", Repository: "busybox", Tag: "latest"},
	})
	var cmd tea.Cmd
	next, stepCmd := m.handleKey(spaceKey())
	cmd = stepCmd
	m = next.(Model)
	if cmd != nil {
		t.Fatalf("space should not issue a command, got %v", cmd)
	}
	next, _ = m.handleKey(keyMsg("j"))
	m = next.(Model)
	next, _ = m.handleKey(spaceKey())
	m = next.(Model)
	next, _ = m.handleKey(keyMsg("d"))
	m = next.(Model)
	if m.mode != modeConfirmDelete {
		t.Fatalf("mode=%v want confirm delete", m.mode)
	}
	assertPending(t, m, deleteImages, "a", "b")
	if m.pendingLbl != "2 images" {
		t.Fatalf("pendingLbl=%q want '2 images'", m.pendingLbl)
	}
	if !strings.Contains(ansi.Strip(viewString(m.View())), "Delete 2 images?") {
		t.Fatalf("confirm modal should name the count, got %q", ansi.Strip(viewString(m.View())))
	}
	next, _ = m.handleKey(keyMsg("n"))
	m = next.(Model)
	if m.mode != modeBrowse {
		t.Fatalf("cancel should return to browse, got %v", m.mode)
	}
}

func TestImageSingleMarkStillUsesSinglePath(t *testing.T) {
	m := New()
	m.activeView = ViewImages
	m.imgPanel = m.imgPanel.SetItems([]backend.Image{{ID: "a", Repository: "alpine", Tag: "latest"}})
	next, _ := m.handleKey(spaceKey())
	m = next.(Model)
	next, _ = m.handleKey(keyMsg("d"))
	m = next.(Model)
	if m.mode != modeConfirmDelete {
		t.Fatalf("mode=%v want confirm delete", m.mode)
	}
	assertPending(t, m, deleteImages, "a")
}

func TestVolumeBulkDeleteConfirm(t *testing.T) {
	m := New()
	m.width, m.height = 100, 30
	m.activeView = ViewVolumes
	m.volPanel = m.volPanel.SetItems([]backend.Volume{{Name: "data", Driver: "local"}, {Name: "logs", Driver: "local"}})
	var next tea.Model
	next, _ = m.handleKey(spaceKey())
	m = next.(Model)
	next, _ = m.handleKey(keyMsg("j"))
	m = next.(Model)
	next, _ = m.handleKey(spaceKey())
	m = next.(Model)
	next, _ = m.handleKey(keyMsg("d"))
	m = next.(Model)
	if m.mode != modeConfirmDelete {
		t.Fatalf("mode=%v want confirm delete", m.mode)
	}
	assertPending(t, m, deleteVolumes, "data", "logs")
	if m.pendingLbl != "2 volumes" {
		t.Fatalf("pendingLbl=%q want '2 volumes'", m.pendingLbl)
	}
	if !strings.Contains(ansi.Strip(viewString(m.View())), "Delete 2 volumes?") {
		t.Fatalf("confirm modal should name the count, got %q", ansi.Strip(viewString(m.View())))
	}
}

// assertPending checks the staged delete targets the given panel with exactly
// these ids, in list order.
func assertPending(t *testing.T, m Model, kind deleteKind, ids ...string) {
	t.Helper()
	if m.pendingKind != kind {
		t.Fatalf("pendingKind=%v want %v", m.pendingKind, kind)
	}
	if len(m.pendingIDs) != len(ids) {
		t.Fatalf("pendingIDs=%v want %v", m.pendingIDs, ids)
	}
	for i := range ids {
		if m.pendingIDs[i] != ids[i] {
			t.Fatalf("pendingIDs=%v want %v", m.pendingIDs, ids)
		}
	}
}

func viewString(v tea.View) string {
	return v.Content
}

func keyMsg(s string) tea.KeyPressMsg {
	if s == "esc" {
		return tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape})
	}
	if len(s) == 1 {
		r := rune(s[0])
		return tea.KeyPressMsg(tea.Key{Code: r, Text: s})
	}
	return tea.KeyPressMsg(tea.Key{Text: s})
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

// Marks key off identities that outlive a removal (an image digest, a volume
// name). The refresh that follows a delete is what drops them, so recreating
// the same image or volume cannot resurface a mark and re-arm a bulk delete
// nobody asked for.
func TestImageDeleteDropsMarksOnceRefreshLands(t *testing.T) {
	m := New()
	m.width, m.height = 100, 30
	m.activeView = ViewImages
	items := []backend.Image{
		{ID: "a", Repository: "alpine", Tag: "latest"},
		{ID: "b", Repository: "busybox", Tag: "latest"},
	}
	m.imgPanel = m.imgPanel.SetItems(items)
	m = markRows(t, m, 2)
	next, _ := m.handleKey(keyMsg("d"))
	m = next.(Model)
	assertPending(t, m, deleteImages, "a", "b")
	next, _ = m.handleKey(keyMsg("y"))
	m = next.(Model)
	if m.mode != modeBrowse {
		t.Fatalf("mode=%v want browse after confirm", m.mode)
	}
	// The delete lands and the refresh reports both images gone.
	m.imgPanel = m.imgPanel.SetItems(nil)
	if got := m.imgPanel.MarkedIDs(); len(got) != 0 {
		t.Fatalf("marks survived the delete: %v", got)
	}
	// Both images are pulled again under the same digests.
	m.imgPanel = m.imgPanel.SetItems(items)
	if got := m.imgPanel.MarkedIDs(); len(got) != 0 {
		t.Fatalf("marks resurfaced after recreate: %v", got)
	}
	if strings.Contains(ansi.Strip(m.imgPanel.ListView(100, 10)), "*") {
		t.Fatal("recreated images still render a mark")
	}
}

func TestVolumeDeleteDropsMarksOnceRefreshLands(t *testing.T) {
	m := New()
	m.width, m.height = 100, 30
	m.activeView = ViewVolumes
	items := []backend.Volume{{Name: "data", Driver: "local"}, {Name: "logs", Driver: "local"}}
	m.volPanel = m.volPanel.SetItems(items)
	m = markRows(t, m, 2)
	next, _ := m.handleKey(keyMsg("d"))
	m = next.(Model)
	assertPending(t, m, deleteVolumes, "data", "logs")
	next, _ = m.handleKey(keyMsg("y"))
	m = next.(Model)
	m.volPanel = m.volPanel.SetItems(nil)
	if got := m.volPanel.MarkedIDs(); len(got) != 0 {
		t.Fatalf("marks survived the delete: %v", got)
	}
	m.volPanel = m.volPanel.SetItems(items)
	if got := m.volPanel.MarkedIDs(); len(got) != 0 {
		t.Fatalf("marks resurfaced after recreate: %v", got)
	}
}

func TestContainerDeleteDropsMarksOnceRefreshLands(t *testing.T) {
	m := New()
	m.width, m.height = 100, 30
	items := []backend.Container{
		{ID: "1", Name: "web", Status: "running"},
		{ID: "2", Name: "db", Status: "running"},
	}
	m.cntPanel = m.cntPanel.SetItems(items)
	m = markRows(t, m, 2)
	next, _ := m.handleKey(keyMsg("d"))
	m = next.(Model)
	assertPending(t, m, deleteContainers, "1", "2")
	next, _ = m.handleKey(keyMsg("y"))
	m = next.(Model)
	m.cntPanel = m.cntPanel.SetItems(nil)
	if got := m.cntPanel.MarkedIDs(); len(got) != 0 {
		t.Fatalf("marks survived the delete: %v", got)
	}
	m.cntPanel = m.cntPanel.SetItems(items)
	if got := m.cntPanel.MarkedIDs(); len(got) != 0 {
		t.Fatalf("marks resurfaced after recreate: %v", got)
	}
}

// Prune removes the same objects without passing through the confirmation, so
// the mark drop has to hold there too.
func TestVolumePruneThenRecreateDoesNotResurfaceMarks(t *testing.T) {
	m := New()
	m.activeView = ViewVolumes
	items := []backend.Volume{{Name: "data", Driver: "local"}, {Name: "logs", Driver: "local"}}
	m.volPanel = m.volPanel.SetItems(items)
	m = markRows(t, m, 2)
	// `p` prunes both volumes; the refresh it triggers reports an empty list.
	m.volPanel = m.volPanel.SetItems(nil)
	// Both are recreated under the same names.
	m.volPanel = m.volPanel.SetItems(items)
	if got := m.volPanel.MarkedIDs(); len(got) != 0 {
		t.Fatalf("marks resurfaced after prune + recreate: %v", got)
	}
	next, _ := m.handleKey(keyMsg("d"))
	m = next.(Model)
	assertPending(t, m, deleteVolumes, "data")
	if m.pendingLbl != "data" {
		t.Fatalf("pendingLbl=%q: a resurfaced mark turned d into a bulk delete", m.pendingLbl)
	}
}

// A delete that fails leaves its objects in place, so the marks must still be
// there afterwards and the action must stay retryable without re-marking.
func TestFailedDeleteKeepsMarks(t *testing.T) {
	m := New()
	m.width, m.height = 100, 30
	m.activeView = ViewImages
	items := []backend.Image{
		{ID: "a", Repository: "alpine", Tag: "latest"},
		{ID: "b", Repository: "busybox", Tag: "latest"},
	}
	m.imgPanel = m.imgPanel.SetItems(items)
	m = markRows(t, m, 2)
	next, _ := m.handleKey(keyMsg("d"))
	m = next.(Model)
	next, _ = m.handleKey(keyMsg("y"))
	m = next.(Model)
	// The delete failed (image still referenced), so the refresh returns both.
	m.imgPanel = m.imgPanel.SetItems(items)
	got := m.imgPanel.MarkedIDs()
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("MarkedIDs after a failed delete = %v, want [a b]", got)
	}
}

// The cursor row wins over a single mark, so this delete never touches "b" -
// and a mark on an object the delete left alone must survive it.
func TestDeleteKeepsMarksItDidNotTouch(t *testing.T) {
	m := New()
	m.width, m.height = 100, 30
	m.activeView = ViewImages
	items := []backend.Image{
		{ID: "a", Repository: "alpine", Tag: "latest"},
		{ID: "b", Repository: "busybox", Tag: "latest"},
	}
	m.imgPanel = m.imgPanel.SetItems(items)
	next, _ := m.handleKey(keyMsg("j"))
	m = next.(Model)
	next, _ = m.handleKey(spaceKey())
	m = next.(Model)
	next, _ = m.handleKey(keyMsg("k"))
	m = next.(Model)
	next, _ = m.handleKey(keyMsg("d"))
	m = next.(Model)
	assertPending(t, m, deleteImages, "a")
	next, _ = m.handleKey(keyMsg("y"))
	m = next.(Model)
	m.imgPanel = m.imgPanel.SetItems(items[1:])
	got := m.imgPanel.MarkedIDs()
	if len(got) != 1 || got[0] != "b" {
		t.Fatalf("MarkedIDs = %v, want [b]: deleting alpine must not drop busybox's mark", got)
	}
}

// A mark hidden behind an active filter is not a delete target: it survives a
// delete of something else, and disappears only when its own volume does.
func TestVolumeMarkHiddenByFilterTracksItsOwnVolume(t *testing.T) {
	m := New()
	m.width, m.height = 100, 30
	m.activeView = ViewVolumes
	items := []backend.Volume{{Name: "data", Driver: "local"}, {Name: "logs", Driver: "local"}}
	m.volPanel = m.volPanel.SetItems(items)
	m.volPanel, _ = m.volPanel.Update(spaceKey())
	m.volPanel, _ = m.volPanel.Update(keyMsg("j"))
	m.volPanel, _ = m.volPanel.Update(spaceKey())
	m.volPanel, _ = m.volPanel.Update(keyMsg("/"))
	for _, r := range "data" {
		m.volPanel, _ = m.volPanel.Update(keyMsg(string(r)))
	}
	m.volPanel, _ = m.volPanel.Update(enterKey())
	next, _ := m.handleKey(keyMsg("d"))
	m = next.(Model)
	assertPending(t, m, deleteVolumes, "data")
	next, _ = m.handleKey(keyMsg("y"))
	m = next.(Model)
	// "data" is gone, "logs" is untouched behind the filter.
	m.volPanel = m.volPanel.SetItems(items[1:])
	m.volPanel, _ = m.volPanel.Update(keyMsg("esc"))
	got := m.volPanel.MarkedIDs()
	if len(got) != 1 || got[0] != "logs" {
		t.Fatalf("MarkedIDs = %v, want [logs]", got)
	}
	// Now "logs" goes away too, then both come back.
	m.volPanel = m.volPanel.SetItems(nil)
	m.volPanel = m.volPanel.SetItems(items)
	if got := m.volPanel.MarkedIDs(); len(got) != 0 {
		t.Fatalf("marks resurfaced after recreate: %v", got)
	}
}

// markRows marks the first n rows of the active panel with space, moving the
// cursor down between presses the way a user would.
func markRows(t *testing.T, m Model, n int) Model {
	t.Helper()
	for i := 0; i < n; i++ {
		if i > 0 {
			next, _ := m.handleKey(keyMsg("j"))
			m = next.(Model)
		}
		next, _ := m.handleKey(spaceKey())
		m = next.(Model)
	}
	return m
}

func TestCancelledDeleteKeepsMarks(t *testing.T) {
	m := New()
	m.width, m.height = 100, 30
	m.activeView = ViewImages
	m.imgPanel = m.imgPanel.SetItems([]backend.Image{
		{ID: "a", Repository: "alpine", Tag: "latest"},
		{ID: "b", Repository: "busybox", Tag: "latest"},
	})
	next, _ := m.handleKey(spaceKey())
	m = next.(Model)
	next, _ = m.handleKey(keyMsg("j"))
	m = next.(Model)
	next, _ = m.handleKey(spaceKey())
	m = next.(Model)
	next, _ = m.handleKey(keyMsg("d"))
	m = next.(Model)
	next, _ = m.handleKey(keyMsg("n"))
	m = next.(Model)
	if got := m.imgPanel.MarkedIDs(); len(got) != 2 {
		t.Fatalf("cancelling a delete must keep marks, got %v", got)
	}
}
