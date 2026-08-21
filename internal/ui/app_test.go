package ui

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/Laaaaksh/vessel/internal/backend"
	"github.com/Laaaaksh/vessel/internal/config"
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

func fakeCLI(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	bin := filepath.Join(filepath.Dir(file), "..", "backend", "fakecli", "container")
	if _, err := os.Stat(bin); err != nil {
		t.Fatalf("fake cli missing: %v", err)
	}
	return bin
}

func imagesModel(t *testing.T, items []backend.Image) Model {
	t.Helper()
	m := New()
	m.width, m.height = 100, 30
	m.activeView = ViewImages
	m.imgPanel = m.imgPanel.SetItems(items)
	m.client = backend.NewClientWithBinary(fakeCLI(t))
	return m
}

func findAction(items []actionItem, label string) func(Model) (Model, tea.Cmd) {
	for _, it := range items {
		if it.label == label {
			return it.run
		}
	}
	return nil
}

func lastCLICommand(m Model) string {
	if m.client == nil {
		return ""
	}
	log := m.client.CommandLog()
	if len(log) == 0 {
		return ""
	}
	return log[len(log)-1]
}

func TestImagesActionsMenu_listsImageMobility(t *testing.T) {
	m := imagesModel(t, nil)
	for _, label := range []string{"Tag…", "Save…", "Load…", "Push"} {
		if findAction(m.buildActions(), label) == nil {
			t.Fatalf("images actions menu missing %q", label)
		}
	}
}

func TestImagesAction_Tag_flow(t *testing.T) {
	m := imagesModel(t, []backend.Image{{ID: "sha256:abc", Repository: "alpine", Tag: "latest"}})
	run := findAction(m.buildActions(), "Tag…")
	if run == nil {
		t.Fatal("Tag action missing")
	}
	x, _ := run(m)
	mm := x
	if mm.promptKind != "tag" || mm.promptRef != "alpine:latest" {
		t.Fatalf("tag prompt state: kind=%q ref=%q", mm.promptKind, mm.promptRef)
	}
	next, cmd := mm.handlePrompt(promptDoneMsg{kind: "tag", text: "alpine:probe"})
	msg := cmd()
	done := msg.(actionDoneMsg)
	if done.err != nil {
		t.Fatal(done.err)
	}
	res, _ := next.Update(done)
	out := res.(Model)
	if out.status != "tag alpine:probe ok" {
		t.Fatalf("status: got %q", out.status)
	}
	if got := lastCLICommand(out); got != "container image tag alpine:latest alpine:probe" {
		t.Fatalf("tag argument order: got %q", got)
	}
}

func TestImagesAction_Save_flow(t *testing.T) {
	m := imagesModel(t, []backend.Image{{ID: "sha256:abc", Repository: "alpine", Tag: "latest"}})
	run := findAction(m.buildActions(), "Save…")
	next, _ := run(m)
	mm := next
	if mm.promptKind != "save to" || mm.promptRef != "alpine:latest" {
		t.Fatalf("save prompt state: kind=%q ref=%q", mm.promptKind, mm.promptRef)
	}
	path := filepath.Join(t.TempDir(), "out.tar")
	res, cmd2 := mm.handlePrompt(promptDoneMsg{kind: "save to", text: path})
	done := cmd2().(actionDoneMsg)
	if done.err != nil {
		t.Fatal(done.err)
	}
	out, _ := res.Update(done)
	om := out.(Model)
	if got := lastCLICommand(om); got != "container image save --output "+path+" alpine:latest" {
		t.Fatalf("save argument order: got %q", got)
	}
}

func TestImagesAction_Load_flow(t *testing.T) {
	path := filepath.Join(t.TempDir(), "in.tar")
	if err := os.WriteFile(path, []byte("oci-archive"), 0o600); err != nil {
		t.Fatal(err)
	}
	m := imagesModel(t, nil)
	run := findAction(m.buildActions(), "Load…")
	next, _ := run(m)
	mm := next
	res, cmd2 := mm.handlePrompt(promptDoneMsg{kind: "load from", text: path})
	done := cmd2().(actionDoneMsg)
	if done.err != nil {
		t.Fatal(done.err)
	}
	out, _ := res.Update(done)
	om := out.(Model)
	if got := lastCLICommand(om); got != "container image load --input "+path {
		t.Fatalf("load argument order: got %q", got)
	}
}

func TestImagesAction_Load_missingFile(t *testing.T) {
	m := imagesModel(t, nil)
	run := findAction(m.buildActions(), "Load…")
	next, _ := run(m)
	mm := next
	res, cmd := mm.handlePrompt(promptDoneMsg{kind: "load from", text: "/no/such/archive.tar"})
	done := cmd().(actionDoneMsg)
	if done.err == nil {
		t.Fatal("expected a missing-file error")
	}
	if !strings.Contains(done.err.Error(), "no such file") {
		t.Fatalf("want a clear no-such-file error, got: %v", done.err)
	}
	out, _ := res.Update(done)
	om := out.(Model)
	if om.lastErr == nil || !strings.Contains(om.lastErr.Error(), "no such file") {
		t.Fatalf("footer error must surface the missing file, got: %v", om.lastErr)
	}
	if got := lastCLICommand(om); got != "" {
		t.Fatalf("missing path must not shell out, but recorded %q", got)
	}
}

// beginPush selects Push from the images action menu and returns the model
// sitting in the confirmation it must open before anything is published.
func beginPush(t *testing.T, m Model) Model {
	t.Helper()
	run := findAction(m.buildActions(), "Push")
	if run == nil {
		t.Fatal("Push action missing")
	}
	next, cmd := run(m)
	if cmd != nil {
		t.Fatal("Push must not start work before the user confirms")
	}
	if next.mode != modeConfirmDelete {
		t.Fatalf("Push must open the confirm modal, mode=%v", next.mode)
	}
	if got := lastCLICommand(next); got != "" {
		t.Fatalf("Push must not shell out before confirmation, recorded %q", got)
	}
	return next
}

// modalText returns the rendered modal as one normalised line, so assertions
// read the label rather than wherever lipgloss happened to wrap it.
func modalText(t *testing.T, m Model) string {
	t.Helper()
	stripped := ansi.Strip(viewString(m.View()))
	noBorders := strings.Map(func(r rune) rune {
		if strings.ContainsRune("│─╭╮╰╯", r) {
			return -1
		}
		return r
	}, stripped)
	return strings.Join(strings.Fields(noBorders), " ")
}

// squash drops all whitespace so an assertion survives lipgloss hard-wrapping a
// long token such as a temp-directory path.
func squash(s string) string {
	return strings.Join(strings.Fields(s), "")
}

func TestImagesAction_Push_confirmOmitsAnUnknownDestination(t *testing.T) {
	// An unqualified ref resolves against the CLI's configured default registry,
	// which vessel does not read, so the label must name no destination at all
	// rather than assert a guess before an unrecoverable publish.
	m := beginPush(t, imagesModel(t, []backend.Image{
		{ID: "sha256:abc", Repository: "vessel/alpine", Tag: "probe"},
	}))
	view := modalText(t, m)
	if !strings.Contains(view, "Push vessel/alpine:probe?") {
		t.Fatalf("confirm modal must name the image, got: %q", view)
	}
	if strings.Contains(view, "docker.io") || strings.Contains(view, "→") {
		t.Fatalf("confirm modal must not guess a destination, got: %q", view)
	}
	if strings.Contains(view, "Delete vessel") {
		t.Fatalf("push confirmation must not read as a delete: %q", view)
	}
}

func TestImagesAction_Push_confirmNamesPrivateRegistry(t *testing.T) {
	m := beginPush(t, imagesModel(t, []backend.Image{
		{ID: "sha256:abc", Repository: "registry.local:5000/team/app", Tag: "v2"},
	}))
	view := modalText(t, m)
	if !strings.Contains(view, "Push registry.local:5000/team/app:v2 → registry.local:5000?") {
		t.Fatalf("confirm modal must name the private registry, got: %q", view)
	}
}

func TestImagesActions_refuseUntaggedImage(t *testing.T) {
	// A digest-pinned row lists with an empty tag; formatting it yields a bare
	// repository that a registry resolves as :latest — a different artifact.
	m := imagesModel(t, []backend.Image{{ID: "sha256:abc", Repository: "alpine", Tag: ""}})
	for _, label := range []string{"Tag…", "Save…", "Push"} {
		run := findAction(m.buildActions(), label)
		if run == nil {
			t.Fatalf("missing action %q", label)
		}
		next, cmd := run(m)
		if cmd != nil {
			t.Fatalf("%s must not act on an untagged image", label)
		}
		if next.mode != modeBrowse {
			t.Fatalf("%s on an untagged image opened mode %v", label, next.mode)
		}
		if !strings.Contains(next.status, "no named reference") {
			t.Fatalf("%s status must explain why the row is unaddressable, got %q", label, next.status)
		}
		if strings.Contains(next.status, "tag it first") {
			t.Fatalf("%s must not prescribe an action it also refuses, got %q", label, next.status)
		}
		if got := lastCLICommand(next); got != "" {
			t.Fatalf("%s on an untagged image shelled out: %q", label, got)
		}
	}
}

func TestImagesAction_Save_confirmsOverwrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "existing.tar")
	if err := os.WriteFile(path, []byte("precious"), 0o600); err != nil {
		t.Fatal(err)
	}
	m := imagesModel(t, []backend.Image{{ID: "sha256:abc", Repository: "alpine", Tag: "latest"}})
	run := findAction(m.buildActions(), "Save…")
	next, _ := run(m)
	res, cmd := next.handlePrompt(promptDoneMsg{kind: "save to", text: path})
	mm := res.(Model)
	if cmd != nil {
		t.Fatal("saving over an existing file must ask first")
	}
	if mm.mode != modeConfirmDelete {
		t.Fatalf("expected a confirmation, mode=%v", mm.mode)
	}
	view := modalText(t, mm)
	if !strings.Contains(squash(view), squash("Overwrite "+path+" with alpine:latest?")) {
		t.Fatalf("overwrite confirmation must name file and image, got: %q", view)
	}
	if got := lastCLICommand(mm); got != "" {
		t.Fatalf("must not shell out before confirmation, recorded %q", got)
	}

	cancelled, _ := mm.handleKey(keyMsg("n"))
	if got := lastCLICommand(cancelled.(Model)); got != "" {
		t.Fatalf("cancelled save shelled out: %q", got)
	}
	if body, err := os.ReadFile(path); err != nil || string(body) != "precious" {
		t.Fatalf("cancelled save must leave the file alone, got %q %v", body, err)
	}

	confirmed, cmd := mm.handleKey(keyMsg("y"))
	if cmd == nil {
		t.Fatal("confirming must start the save")
	}
	done := cmd().(actionDoneMsg)
	if done.err != nil {
		t.Fatal(done.err)
	}
	out, _ := confirmed.(Model).Update(done)
	if got := lastCLICommand(out.(Model)); got != "container image save --output "+path+" alpine:latest" {
		t.Fatalf("save argument order: got %q", got)
	}
}

func TestImagesAction_Save_newPathNeedsNoConfirmation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fresh.tar")
	m := imagesModel(t, []backend.Image{{ID: "sha256:abc", Repository: "alpine", Tag: "latest"}})
	run := findAction(m.buildActions(), "Save…")
	next, _ := run(m)
	res, cmd := next.handlePrompt(promptDoneMsg{kind: "save to", text: path})
	if cmd == nil {
		t.Fatal("saving to a fresh path should run without a confirmation")
	}
	if mm := res.(Model); mm.mode == modeConfirmDelete {
		t.Fatal("a fresh path must not raise an overwrite confirmation")
	}
	if done := cmd().(actionDoneMsg); done.err != nil {
		t.Fatal(done.err)
	}
}

func TestFooterView_clampsCLIErrorToOneRow(t *testing.T) {
	t.Setenv("FAKE_CONTAINER_FAIL_PUSH", "auth")
	m := beginPush(t, imagesModel(t, []backend.Image{
		{ID: "sha256:abc", Repository: "alpine", Tag: "latest"},
	}))
	next, cmd := m.handleKey(keyMsg("y"))
	done := cmd().(actionDoneMsg)
	if done.err == nil {
		t.Fatal("expected an auth failure")
	}
	if !strings.Contains(done.err.Error(), "\n") {
		t.Fatal("precondition: a CLI error should carry stderr newlines")
	}
	out, _ := next.(Model).Update(done)
	om := out.(Model)

	assertOneRow(t, om, "push auth failure")
	if !strings.Contains(ansi.Strip(om.footerView()), "error:") {
		t.Fatalf("footer should still report the error, got %q", ansi.Strip(om.footerView()))
	}
}

// assertOneRow checks the invariant layoutDims depends on: the footer occupies
// exactly one row of at most m.width display cells, whatever it is rendering.
func assertOneRow(t *testing.T, m Model, what string) {
	t.Helper()
	footer := ansi.Strip(m.footerView())
	if n := strings.Count(footer, "\n"); n != 0 {
		t.Fatalf("%s: footer must occupy one row, got %d extra:\n%s", what, n, footer)
	}
	if w := lipgloss.Width(footer); w > m.width {
		t.Fatalf("%s: footer is %d cells wide, frame is %d — it will wrap", what, w, m.width)
	}
}

func TestFooterView_keyHintsKeepTheirGrouping(t *testing.T) {
	m := New()
	m.width, m.height = 120, 24
	m.cntPanel = m.cntPanel.SetItems([]backend.Container{{ID: "1", Name: "web", Status: "running"}})
	if m.status != "" || m.lastErr != nil {
		t.Fatal("precondition: the key-hint branch needs no status and no error")
	}
	footer := ansi.Strip(m.footerView())
	if !strings.Contains(footer, "[d] remove  [/] filter") {
		t.Fatalf("key hints must keep their authored double-space grouping, got %q", footer)
	}
	if strings.Contains(footer, "…") {
		t.Fatalf("key hints must not be truncated when they fit, got %q", footer)
	}
}

func TestHelpView_imagesFitsAnEightyByTwentyFourTerminal(t *testing.T) {
	for _, view := range []View{ViewContainers, ViewImages, ViewVolumes} {
		m := New()
		m.width, m.height = 80, 24
		m.activeView = view
		rendered := ansi.Strip(m.helpView())
		if rows := len(strings.Split(strings.TrimRight(rendered, "\n"), "\n")); rows > m.height {
			t.Errorf("%s help renders %d rows into a %d-row screen", m.viewName(), rows, m.height)
		}
	}
}

func TestImagesDetail_showsRegistryLoginAfterAuthFailure(t *testing.T) {
	t.Setenv("FAKE_CONTAINER_FAIL_PUSH", "auth")
	m := beginPush(t, imagesModel(t, []backend.Image{
		{ID: "sha256:abc", Repository: "alpine", Tag: "latest"},
	}))
	next, cmd := m.handleKey(keyMsg("y"))
	done := cmd().(actionDoneMsg)
	if done.err == nil {
		t.Fatal("expected an auth failure")
	}
	out, _ := next.(Model).Update(done)
	om := out.(Model)

	// The footer is clamped to one row, so it cannot be the only surface.
	detail := squash(ansi.Strip(om.imgPanel.DetailView(40, 20)))
	if !strings.Contains(detail, squash("container registry login")) {
		t.Fatalf("detail pane must name the login command, got: %q", detail)
	}

	// A later success clears the standing notice.
	cleared, _ := om.Update(actionDoneMsg{msg: "pull ok"})
	after := squash(ansi.Strip(cleared.(Model).imgPanel.DetailView(40, 20)))
	if strings.Contains(after, squash("container registry login")) {
		t.Fatalf("notice should clear once an action succeeds, got: %q", after)
	}
}

func TestImagesDetail_noticeNeverGrowsThePaneBeyondItsBudget(t *testing.T) {
	panel := New().imgPanel.
		SetItems([]backend.Image{{ID: "sha256:abc", Repository: "alpine", Tag: "latest"}}).
		SetNotice(backend.PushAuthNotice)
	// The geometry mainPanels hands the detail pane on the smallest terminal
	// View() still renders (60x12), and on 80x24 with the command log open.
	for _, size := range []struct{ w, h int }{{18, 8}, {14, 16}, {40, 20}} {
		rendered := ansi.Strip(panel.DetailView(size.w, size.h))
		rows := strings.Split(strings.TrimRight(rendered, "\n"), "\n")
		if len(rows) > size.h {
			t.Errorf("detail pane %dx%d rendered %d rows: %q", size.w, size.h, len(rows), rendered)
		}
		for _, row := range rows {
			if w := lipgloss.Width(row); w > size.w {
				t.Errorf("detail pane %dx%d row is %d cells wide: %q", size.w, size.h, w, row)
			}
		}
	}
}

func TestImagesDetail_noticeSurvivesTheSmallestSupportedPane(t *testing.T) {
	items := []backend.Image{{
		ID:         "sha256:abc",
		Repository: "ghcr.io/a-deliberately-long-org/a-deliberately-long-image",
		Tag:        "v1",
	}}
	// Both refusals are held to the same envelope: 18x4 is what mainPanels hands
	// the detail pane at 60x12 — the smallest frame View() still renders — with
	// the command log toggled on.
	for _, notice := range []string{backend.PushAuthNotice, backend.PushPermissionNotice} {
		panel := New().imgPanel.SetItems(items).SetNotice(notice)
		for _, size := range []struct{ w, h int }{{18, 4}, {18, 8}, {14, 16}, {40, 20}} {
			detail := squash(ansi.Strip(panel.DetailView(size.w, size.h)))
			if !strings.Contains(detail, squash(notice)) {
				t.Errorf("detail pane %dx%d truncates %q: %q", size.w, size.h, notice, detail)
			}
		}
	}
}

func TestImagesDetail_noticeNotShownForANonPushFailure(t *testing.T) {
	// Docker Hub answers 401 for a repository that does not exist, so a typo'd
	// pull produces auth-shaped stderr. It is not a credentials problem.
	m := imagesModel(t, []backend.Image{{ID: "sha256:abc", Repository: "alpine", Tag: "latest"}})
	pullErr := &backend.CLIError{
		Args:   []string{"image", "pull", "vessel-no-such-xyz123:latest"},
		Stderr: "Error: ... 401 Unauthorized. Reason: Unknown, no credentials found for host registry-1.docker.io\n",
		Err:    errors.New("exit status 1"),
	}
	if backend.PushDenialNotice(pullErr) == "" {
		t.Fatal("precondition: this stderr should classify as auth-shaped")
	}
	out, _ := m.Update(actionDoneMsg{err: pullErr})
	om := out.(Model)
	detail := squash(ansi.Strip(om.imgPanel.DetailView(40, 20)))
	if strings.Contains(detail, squash("container registry login")) {
		t.Fatalf("a failed pull must not offer credential advice, got: %q", detail)
	}
	if om.lastErr == nil {
		t.Fatal("the error itself should still reach the footer")
	}
}

func TestImagesDetail_noticeClearedByALaterNonAuthFailure(t *testing.T) {
	t.Setenv("FAKE_CONTAINER_FAIL_PUSH", "auth")
	m := beginPush(t, imagesModel(t, []backend.Image{
		{ID: "sha256:abc", Repository: "alpine", Tag: "latest"},
	}))
	next, cmd := m.handleKey(keyMsg("y"))
	out, _ := next.(Model).Update(cmd().(actionDoneMsg))
	om := out.(Model)
	if !strings.Contains(squash(ansi.Strip(om.imgPanel.DetailView(40, 20))), squash("container registry login")) {
		t.Fatal("precondition: the auth failure should have set the notice")
	}

	after, _ := om.Update(actionDoneMsg{err: errors.New("unexpected network failure")})
	am := after.(Model)
	detail := squash(ansi.Strip(am.imgPanel.DetailView(40, 20)))
	if strings.Contains(detail, squash("container registry login")) {
		t.Fatalf("a non-auth failure must not leave stale credential advice, got: %q", detail)
	}
}

func TestHelpView_keyColumnNeverRunsIntoItsDescription(t *testing.T) {
	for _, view := range []View{ViewContainers, ViewImages, ViewVolumes} {
		m := New()
		m.width, m.height = 80, 24
		m.activeView = view
		rendered := ansi.Strip(m.helpView())
		for _, b := range helpBindings(m.activeView, m.focus, m.mode) {
			if b.key == "" || b.desc == "" {
				continue
			}
			if strings.Contains(rendered, b.key+b.desc) {
				t.Errorf("%s help: key %q runs straight into its description", m.viewName(), b.key)
			}
		}
	}
}

func TestActionsModal_fitsTheSmallestSupportedFrame(t *testing.T) {
	for _, view := range []View{ViewContainers, ViewImages, ViewVolumes} {
		m := New()
		m.width, m.height = 60, 12
		m.activeView = view
		m.imgPanel = m.imgPanel.SetItems([]backend.Image{{ID: "sha256:abc", Repository: "alpine", Tag: "latest"}})
		m.cntPanel = m.cntPanel.SetItems([]backend.Container{{ID: "1", Name: "web", Status: "running"}})
		next, _ := m.handleKey(keyMsg("x"))
		mm := next.(Model)
		if mm.mode != modeActions {
			t.Fatalf("%s: x should open the actions menu, got mode %v", m.viewName(), mm.mode)
		}
		frame := ansi.Strip(viewString(mm.View()))
		if rows := len(strings.Split(strings.TrimRight(frame, "\n"), "\n")); rows > mm.height {
			t.Errorf("%s actions menu renders %d rows into a %d-row frame", m.viewName(), rows, mm.height)
		}
	}
}

func TestActionsModal_longLabelCannotOutgrowTheFrame(t *testing.T) {
	m := New()
	m.width, m.height = 60, 12
	m.cfg.CustomCommands = []config.CustomCommand{
		{Name: "tail all container logs together", Command: "echo hi"},
		{Name: "another deliberately long custom command name", Command: "echo hi"},
	}
	next, _ := m.handleKey(keyMsg("x"))
	mm := next.(Model)

	// Walk the whole menu so every label, selected and not, renders in the window.
	for i := 0; i < len(mm.actionItems); i++ {
		frame := ansi.Strip(viewString(mm.View()))
		if rows := len(strings.Split(strings.TrimRight(frame, "\n"), "\n")); rows > mm.height {
			t.Fatalf("item %d (%q) grows the frame to %d rows in a %d-row terminal",
				i, mm.actionItems[i].label, rows, mm.height)
		}
		for _, row := range strings.Split(ansi.Strip(mm.actionsModal()), "\n") {
			if w := lipgloss.Width(row); w > mm.actionsModalWidth() {
				t.Fatalf("item %d: modal row is %d cells wide, modal is %d: %q",
					i, w, mm.actionsModalWidth(), row)
			}
		}
		n, _ := mm.handleKey(keyMsg("j"))
		mm = n.(Model)
	}
}

func TestActionsModal_windowFollowsTheSelection(t *testing.T) {
	m := New()
	m.width, m.height = 60, 12
	m.activeView = ViewImages
	m.imgPanel = m.imgPanel.SetItems([]backend.Image{{ID: "sha256:abc", Repository: "alpine", Tag: "latest"}})
	next, _ := m.handleKey(keyMsg("x"))
	mm := next.(Model)
	if len(mm.actionItems) < 5 {
		t.Fatalf("precondition: this test needs a menu taller than the window, got %d items", len(mm.actionItems))
	}
	last := mm.actionItems[len(mm.actionItems)-1].label

	if strings.Contains(modalText(t, mm), last) {
		t.Fatalf("precondition: the last item should start outside the window, got %q", modalText(t, mm))
	}
	for i := 0; i < len(mm.actionItems)-1; i++ {
		n, _ := mm.handleKey(keyMsg("j"))
		mm = n.(Model)
	}
	view := modalText(t, mm)
	if !strings.Contains(view, last) {
		t.Fatalf("the window must scroll to the selection, got %q", view)
	}
	if !strings.Contains(view, fmt.Sprintf("%d/%d", len(mm.actionItems), len(mm.actionItems))) {
		t.Fatalf("a windowed menu should say where the selection sits, got %q", view)
	}
	frame := ansi.Strip(viewString(mm.View()))
	if rows := len(strings.Split(strings.TrimRight(frame, "\n"), "\n")); rows > mm.height {
		t.Errorf("scrolled menu renders %d rows into a %d-row frame", rows, mm.height)
	}
}

func TestImagesDetail_forbiddenPushDoesNotBlameCredentials(t *testing.T) {
	t.Setenv("FAKE_CONTAINER_FAIL_PUSH", "forbidden")
	m := beginPush(t, imagesModel(t, []backend.Image{
		{ID: "sha256:abc", Repository: "myorg/app", Tag: "v1"},
	}))
	next, cmd := m.handleKey(keyMsg("y"))
	done := cmd().(actionDoneMsg)
	if done.err == nil {
		t.Fatal("expected the forbidden push to fail")
	}
	out, _ := next.(Model).Update(done)
	om := out.(Model)

	detail := squash(ansi.Strip(om.imgPanel.DetailView(40, 20)))
	if !strings.Contains(detail, squash("no write access")) {
		t.Fatalf("a 403 must be explained as a permission problem, got: %q", detail)
	}
	if strings.Contains(detail, squash("push rejected — run")) {
		t.Fatalf("a 403 must not tell the user to re-run a login they already hold, got: %q", detail)
	}
}

func TestImagesAction_Tag_promptNamesSourceAndWantsNewRef(t *testing.T) {
	m := imagesModel(t, []backend.Image{{ID: "sha256:abc", Repository: "alpine", Tag: "latest"}})
	run := findAction(m.buildActions(), "Tag…")
	next, _ := run(m)
	if next.mode != modePrompt {
		t.Fatalf("Tag… should open a prompt, got mode %v", next.mode)
	}
	view := modalText(t, next)
	if !strings.Contains(view, "tag alpine:latest as (new reference)") {
		t.Fatalf("tag prompt must name the source image and ask for the new reference, got: %q", view)
	}
}

func TestFooterView_clampsWideRunesByDisplayWidth(t *testing.T) {
	m := New()
	m.width, m.height = 80, 24
	m.lastErr = errors.New(strings.Repeat("容器", 120))
	assertOneRow(t, m, "wide-rune error")

	m.lastErr = nil
	m.status = strings.Repeat("容器", 120)
	assertOneRow(t, m, "wide-rune status")
}

func TestConfirm_pendingActionDoesNotOutliveItsModal(t *testing.T) {
	m := imagesModel(t, []backend.Image{
		{ID: "sha256:aaa", Repository: "vessel/alpine", Tag: "probe"},
		{ID: "sha256:bbb", Repository: "vessel/busybox", Tag: "probe"},
	})
	m = beginPush(t, m)

	// An unrelated in-flight command lands and forces the frame back to browse,
	// silently dismissing the push confirmation.
	dismissed, _ := m.Update(actionDoneMsg{msg: "pull ok"})
	mm := dismissed.(Model)
	if mm.mode != modeBrowse {
		t.Fatalf("actionDoneMsg should return to browse, got %v", mm.mode)
	}

	// The user now confirms a delete of a different row.
	mm.imgPanel = mm.imgPanel.SetCursor(1)
	next, _ := mm.handleKey(keyMsg("d"))
	dm := next.(Model)
	if dm.mode != modeConfirmDelete {
		t.Fatalf("d should open the delete confirmation, got %v", dm.mode)
	}
	if !strings.Contains(modalText(t, dm), "Delete vessel/busybox:probe?") {
		t.Fatalf("modal must describe the delete, got: %q", modalText(t, dm))
	}

	confirmed, cmd := dm.handleKey(keyMsg("y"))
	if cmd == nil {
		t.Fatal("confirming should start the delete")
	}
	done := cmd().(actionDoneMsg)
	if done.err != nil {
		t.Fatal(done.err)
	}
	out, _ := confirmed.(Model).Update(done)
	if got := lastCLICommand(out.(Model)); got != "container image delete sha256:bbb" {
		t.Fatalf("confirming a delete ran %q — a stale push closure hijacked it", got)
	}
}

func TestImagesAction_Push_confirmedRunsPush(t *testing.T) {
	m := beginPush(t, imagesModel(t, []backend.Image{
		{ID: "sha256:abc", Repository: "vessel/alpine", Tag: "probe"},
	}))
	next, cmd := m.handleKey(keyMsg("y"))
	mm := next.(Model)
	if mm.mode != modeBrowse {
		t.Fatalf("confirming should return to browse, got %v", mm.mode)
	}
	done := cmd().(actionDoneMsg)
	if done.err != nil {
		t.Fatal(done.err)
	}
	out, _ := mm.Update(done)
	om := out.(Model)
	if om.status != "push vessel/alpine:probe ok" {
		t.Fatalf("status: got %q", om.status)
	}
	if got := lastCLICommand(om); got != "container image push vessel/alpine:probe" {
		t.Fatalf("push argument order: got %q", got)
	}
}

func TestImagesAction_Push_cancelledDoesNotPush(t *testing.T) {
	m := beginPush(t, imagesModel(t, []backend.Image{
		{ID: "sha256:abc", Repository: "vessel/alpine", Tag: "probe"},
	}))
	next, cmd := m.handleKey(keyMsg("n"))
	mm := next.(Model)
	if mm.mode != modeBrowse {
		t.Fatalf("cancel should return to browse, got %v", mm.mode)
	}
	if cmd != nil {
		t.Fatal("cancelling must not publish anything")
	}
	if got := lastCLICommand(mm); got != "" {
		t.Fatalf("cancelled push must not shell out, recorded %q", got)
	}
}

func TestImagesAction_Push_authFailureNamesLogin(t *testing.T) {
	t.Setenv("FAKE_CONTAINER_FAIL_PUSH", "auth")
	m := beginPush(t, imagesModel(t, []backend.Image{
		{ID: "sha256:abc", Repository: "alpine", Tag: "latest"},
	}))
	next, cmd := m.handleKey(keyMsg("y"))
	done := cmd().(actionDoneMsg)
	if done.err == nil {
		t.Fatal("expected an auth failure")
	}
	out, _ := next.Update(done)
	om := out.(Model)
	if om.lastErr == nil || !strings.Contains(om.lastErr.Error(), "container registry login") {
		t.Fatalf("push auth error must name the login command, got: %v", om.lastErr)
	}
}

func TestImagesAction_nilSelectionGuards(t *testing.T) {
	m := imagesModel(t, nil)
	for _, label := range []string{"Tag…", "Save…", "Push"} {
		run := findAction(m.buildActions(), label)
		if run == nil {
			t.Fatalf("missing action %q", label)
		}
		next, _ := run(m)
		mm := next
		if mm.status != "nothing selected" {
			t.Fatalf("%s with no selection: status=%q want nothing selected", label, mm.status)
		}
	}
}
