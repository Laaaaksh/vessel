package ui

import (
	"errors"
	"path/filepath"
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

func imagesModel(t *testing.T) Model {
	t.Helper()
	m := New()
	m.cfg.MouseEnabled = true
	m.width, m.height = 120, 40
	m.activeView = ViewImages
	m.client = backend.NewClientWithBinary(filepath.Join(t.TempDir(), "no-such-container-cli"))
	m.imgPanel = m.imgPanel.SetItems([]backend.Image{
		{ID: "1", Repository: "alpine", Tag: "latest"},
		{ID: "2", Repository: "nginx", Tag: "1.27"},
	})
	return m
}

func volumesModel(t *testing.T) Model {
	t.Helper()
	m := New()
	m.cfg.MouseEnabled = true
	m.width, m.height = 120, 40
	m.activeView = ViewVolumes
	m.client = backend.NewClientWithBinary(filepath.Join(t.TempDir(), "no-such-container-cli"))
	m.volPanel = m.volPanel.SetItems([]backend.Volume{
		{Name: "data"},
		{Name: "cache"},
	})
	return m
}

// settle runs the debounce timer the selection change scheduled and delivers
// the resulting message, returning whatever inspect command it triggers.
func settle(t *testing.T, m Model, cmd tea.Cmd) (Model, tea.Cmd) {
	t.Helper()
	if cmd == nil {
		t.Fatal("expected a debounced inspect to be scheduled, got nil")
	}
	msg, ok := cmd().(inspectSettledMsg)
	if !ok {
		t.Fatalf("expected inspectSettledMsg, got %T", cmd())
	}
	next, out := m.Update(msg)
	return next.(Model), out
}

func inspectRefOf(t *testing.T, m Model, cmd tea.Cmd) string {
	t.Helper()
	_, load := settle(t, m, cmd)
	if load == nil {
		t.Fatal("expected an inspect command after the selection settled, got nil")
	}
	msg, ok := load().(imageInspectMsg)
	if !ok {
		t.Fatalf("expected imageInspectMsg, got %T", load())
	}
	return msg.ref
}

func TestMouseWheel_imagesInspectsNewSelection(t *testing.T) {
	m := imagesModel(t)
	next, cmd := m.handleMouseWheel(tea.MouseWheelMsg(tea.Mouse{Button: tea.MouseWheelDown}))
	m = next.(Model)
	if got := backend.FormatRef(*m.imgPanel.Selected()); got != "nginx:1.27" {
		t.Fatalf("selection after wheel = %q, want nginx:1.27", got)
	}
	if got := inspectRefOf(t, m, cmd); got != "nginx:1.27" {
		t.Errorf("inspect ref = %q, want nginx:1.27", got)
	}
}

func TestMouseClick_imagesInspectsNewSelection(t *testing.T) {
	m := imagesModel(t)
	next, cmd := m.handleMouseClick(tea.MouseClickMsg(tea.Mouse{Y: 3, Button: tea.MouseLeft}))
	m = next.(Model)
	if got := backend.FormatRef(*m.imgPanel.Selected()); got != "nginx:1.27" {
		t.Fatalf("selection after click = %q, want nginx:1.27", got)
	}
	if got := inspectRefOf(t, m, cmd); got != "nginx:1.27" {
		t.Errorf("inspect ref = %q, want nginx:1.27", got)
	}
}

func TestMouseWheel_imagesUnchangedSelectionIssuesNoInspect(t *testing.T) {
	m := imagesModel(t)
	m.imgPanel = m.imgPanel.SetInspect("alpine:latest", &backend.ImageInspect{ID: "1"}, nil)
	// Wheel up at the top row cannot move the cursor.
	_, cmd := m.handleMouseWheel(tea.MouseWheelMsg(tea.Mouse{Button: tea.MouseWheelUp}))
	if cmd != nil {
		t.Fatalf("unchanged selection must not inspect, got %T", cmd())
	}
}

func TestMouseWheel_volumesInspectsNewSelection(t *testing.T) {
	m := volumesModel(t)
	next, cmd := m.handleMouseWheel(tea.MouseWheelMsg(tea.Mouse{Button: tea.MouseWheelDown}))
	m = next.(Model)
	if got := m.volPanel.Selected().Name; got != "cache" {
		t.Fatalf("selection after wheel = %q, want cache", got)
	}
	_, load := settle(t, m, cmd)
	if load == nil {
		t.Fatal("expected an inspect command after the selection settled, got nil")
	}
	msg, ok := load().(volumeInspectMsg)
	if !ok {
		t.Fatalf("expected volumeInspectMsg, got %T", load())
	}
	if msg.name != "cache" {
		t.Errorf("inspect name = %q, want cache", msg.name)
	}
}

func TestInspect_rapidSelectionChangesCoalesceIntoOne(t *testing.T) {
	m := imagesModel(t)
	m.imgPanel = m.imgPanel.SetItems([]backend.Image{
		{ID: "1", Repository: "alpine", Tag: "latest"},
		{ID: "2", Repository: "nginx", Tag: "1.27"},
		{ID: "3", Repository: "redis", Tag: "7"},
	})

	// Two cursor steps in quick succession, as a held key produces.
	var scheduled []tea.Cmd
	for range 2 {
		next, cmd := m.handleMouseWheel(tea.MouseWheelMsg(tea.Mouse{Button: tea.MouseWheelDown}))
		m = next.(Model)
		if cmd == nil {
			t.Fatal("selection change must schedule an inspect")
		}
		scheduled = append(scheduled, cmd)
	}
	if got := backend.FormatRef(*m.imgPanel.Selected()); got != "redis:7" {
		t.Fatalf("selection after two steps = %q, want redis:7", got)
	}

	// Every superseded timer must be a no-op...
	for i, cmd := range scheduled[:len(scheduled)-1] {
		if _, load := settle(t, m, cmd); load != nil {
			t.Errorf("superseded timer %d inspected anyway: %T", i, load())
		}
	}
	// ...and only the last one inspects, for the settled selection.
	if got := inspectRefOf(t, m, scheduled[len(scheduled)-1]); got != "redis:7" {
		t.Errorf("settled inspect ref = %q, want redis:7", got)
	}
}

func TestInspect_repeatedListLoadsDoNotStarveThePendingTimer(t *testing.T) {
	items := []backend.Image{{ID: "1", Repository: "alpine", Tag: "latest"}}
	m := imagesModel(t)

	next, first := m.Update(imagesLoadedMsg{items: items})
	m = next.(Model)
	if first == nil {
		t.Fatal("the first list load must schedule an inspect")
	}

	// Poll loads keep arriving faster than the debounce; the selection has not
	// moved, so they must not supersede the timer already in flight.
	for range 3 {
		next, again := m.Update(imagesLoadedMsg{items: items})
		m = next.(Model)
		if again != nil {
			t.Fatalf("an unchanged selection re-armed the debounce: %T", again())
		}
	}

	settled, ok := first().(inspectSettledMsg)
	if !ok {
		t.Fatalf("expected inspectSettledMsg, got %T", first())
	}
	next, load := m.Update(settled)
	m = next.(Model)
	if load == nil {
		t.Fatal("the pending inspect was starved by the intervening list loads")
	}
	if msg, ok := load().(imageInspectMsg); !ok || msg.ref != "alpine:latest" {
		t.Errorf("expected an inspect of alpine:latest, got %T %+v", load(), msg)
	}

	// The resolved timer must not leave the selection marked as pending, or a
	// later request for the same image would never be scheduled again.
	if _, again := m.Update(imagesLoadedMsg{items: items}); again == nil {
		t.Error("a settled selection can no longer schedule a new inspect")
	}
}

func TestLoadImageInspectCmd_skipsWhenAlreadyInspected(t *testing.T) {
	m := imagesModel(t)
	if cmd := m.loadImageInspectCmd(); cmd == nil {
		t.Fatal("first inspect of a selection must be issued")
	}
	m.imgPanel = m.imgPanel.SetInspect("alpine:latest", &backend.ImageInspect{ID: "1"}, nil)
	if cmd := m.loadImageInspectCmd(); cmd != nil {
		t.Error("inspect must be skipped while the same image is already inspected")
	}
	// A failed inspect is not cached, so the next poll retries.
	m.imgPanel = m.imgPanel.SetInspect("alpine:latest", nil, errors.New("boom"))
	if cmd := m.loadImageInspectCmd(); cmd == nil {
		t.Error("failed inspect must be retried")
	}
}

func TestLoadVolumeInspectCmd_skipsWhenAlreadyInspected(t *testing.T) {
	m := volumesModel(t)
	if cmd := m.loadVolumeInspectCmd(); cmd == nil {
		t.Fatal("first inspect of a selection must be issued")
	}
	m.volPanel = m.volPanel.SetInspect("data", &backend.VolumeInspect{Name: "data"}, nil)
	if cmd := m.loadVolumeInspectCmd(); cmd != nil {
		t.Error("inspect must be skipped while the same volume is already inspected")
	}
	m.volPanel = m.volPanel.SetInspect("data", nil, errors.New("boom"))
	if cmd := m.loadVolumeInspectCmd(); cmd == nil {
		t.Error("failed inspect must be retried")
	}
}

func TestSelectionChanged_nilHandling(t *testing.T) {
	img := &backend.Image{Repository: "alpine", Tag: "latest"}
	if selectionRefChanged(nil, nil) {
		t.Error("nil to nil image selection did not change")
	}
	if !selectionRefChanged(nil, img) {
		t.Error("nil to non-nil image selection changed")
	}
	if !selectionRefChanged(img, nil) {
		t.Error("non-nil to nil image selection changed")
	}
	vol := &backend.Volume{Name: "data"}
	if selectionNameChanged(nil, nil) {
		t.Error("nil to nil volume selection did not change")
	}
	if !selectionNameChanged(nil, vol) {
		t.Error("nil to non-nil volume selection changed")
	}
	if !selectionNameChanged(vol, nil) {
		t.Error("non-nil to nil volume selection changed")
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

// The mark key is a KeyMap binding, not a literal in each panel: rebinding it
// has to reach every pane and the bulk delete that acts on the marks.
func TestToggleMarkBindingReachesEveryPanel(t *testing.T) {
	m := New().withKeys(rebound("m"))
	m.width, m.height = 100, 30
	m.activeView = ViewImages
	m.imgPanel = m.imgPanel.SetItems([]backend.Image{
		{ID: "a", Repository: "alpine", Tag: "latest"},
		{ID: "b", Repository: "busybox", Tag: "latest"},
	})
	m.volPanel = m.volPanel.SetItems([]backend.Volume{{Name: "data", Driver: "local"}, {Name: "logs", Driver: "local"}})
	m.cntPanel = m.cntPanel.SetItems([]backend.Container{
		{ID: "1", Name: "web", Status: "running"},
		{ID: "2", Name: "db", Status: "running"},
	})

	next, _ := m.handleKey(spaceKey())
	m = next.(Model)
	if got := m.imgPanel.MarkedIDs(); len(got) != 0 {
		t.Fatalf("space still marks after rebinding: %v", got)
	}
	for _, v := range []View{ViewImages, ViewVolumes, ViewContainers} {
		m.activeView = v
		next, _ = m.handleKey(keyMsg("m"))
		m = next.(Model)
		next, _ = m.handleKey(keyMsg("j"))
		m = next.(Model)
		next, _ = m.handleKey(keyMsg("m"))
		m = next.(Model)
	}
	assertMarked(t, "images", m.imgPanel.MarkedIDs(), "a", "b")
	assertMarked(t, "volumes", m.volPanel.MarkedIDs(), "data", "logs")
	assertMarked(t, "containers", m.cntPanel.MarkedIDs(), "1", "2")

	// The bulk delete acts on what the rebound key marked.
	m.activeView = ViewVolumes
	next, _ = m.handleKey(keyMsg("d"))
	m = next.(Model)
	assertPending(t, m, deleteVolumes, "data", "logs")
}

// The default binding is what a real space bar press produces, so marking and
// bulk-deleting work end to end out of the box.
func TestDefaultToggleMarkBindingMarksAndBulkDeletes(t *testing.T) {
	m := New()
	m.width, m.height = 100, 30
	m.activeView = ViewImages
	m.imgPanel = m.imgPanel.SetItems([]backend.Image{
		{ID: "a", Repository: "alpine", Tag: "latest"},
		{ID: "b", Repository: "busybox", Tag: "latest"},
	})
	m = markRows(t, m, 2)
	assertMarked(t, "images", m.imgPanel.MarkedIDs(), "a", "b")
	next, _ := m.handleKey(keyMsg("d"))
	m = next.(Model)
	assertPending(t, m, deleteImages, "a", "b")
	if m.pendingLbl != "2 images" {
		t.Fatalf("pendingLbl=%q want '2 images'", m.pendingLbl)
	}
}

// The delete command is the one path that destroys user state, so an unhandled
// kind must fail loudly instead of falling through to a container delete.
func TestUnhandledDeleteKindFailsInsteadOfDeletingContainers(t *testing.T) {
	m := New()
	m.cntPanel = m.cntPanel.SetItems([]backend.Container{{ID: "1", Name: "web", Status: "running"}})
	m.mode = modeConfirmDelete
	m.pendingKind = deleteKind(99)
	m.pendingIDs = []string{"1"}
	next, cmd := m.handleKey(keyMsg("y"))
	m = next.(Model)
	if cmd == nil {
		t.Fatal("confirm produced no command")
	}
	done, ok := cmd().(actionDoneMsg)
	if !ok {
		t.Fatalf("cmd returned %T, want actionDoneMsg", cmd())
	}
	if done.err == nil {
		t.Fatal("an unhandled delete kind must report an error, not delete containers")
	}
}

func rebound(toggleMark string) KeyMap {
	k := DefaultKeyMap()
	k.ToggleMark = toggleMark
	return k
}

func assertMarked(t *testing.T, pane string, got []string, want ...string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s MarkedIDs = %v, want %v", pane, got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%s MarkedIDs = %v, want %v", pane, got, want)
		}
	}
}
