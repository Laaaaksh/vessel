package ui

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/Laaaaksh/vessel/internal/backend"
	"github.com/Laaaaksh/vessel/internal/config"
)

func TestView_shellModeEmpty(t *testing.T) {
	m := newTestModel()
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
	m := newTestModel()
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
	m := newTestModel()
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
	m := newTestModel()
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
	m := newTestModel()
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
	m := newTestModel()
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
	m := newTestModel()
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

	// The resolved timer and the completed inspect must not leave the selection
	// marked, or a later request for the same image would never be scheduled.
	next, _ = m.Update(imageInspectMsg{ref: "alpine:latest", err: errors.New("boom")})
	m = next.(Model)
	if _, again := m.Update(imagesLoadedMsg{items: items}); again == nil {
		t.Error("a settled selection can no longer schedule a new inspect")
	}
}

func TestInspect_doesNotReissueWhileOneIsInFlight(t *testing.T) {
	items := []backend.Image{{ID: "1", Repository: "alpine", Tag: "latest"}}
	m := imagesModel(t)

	next, cmd := m.Update(imagesLoadedMsg{items: items})
	m = next.(Model)
	settled, ok := cmd().(inspectSettledMsg)
	if !ok {
		t.Fatalf("expected inspectSettledMsg, got %T", cmd())
	}
	next, load := m.Update(settled)
	m = next.(Model)
	if load == nil {
		t.Fatal("the settled selection must be inspected")
	}

	// The inspect is now out at the CLI. Poll loads arriving before it returns
	// must not launch a second subprocess for the same selection.
	for range 3 {
		next, again := m.Update(imagesLoadedMsg{items: items})
		m = next.(Model)
		if again != nil {
			t.Fatalf("a second inspect was scheduled while one was in flight: %T", again())
		}
	}

	// A different selection is still inspected while the first is in flight.
	nginx := []backend.Image{{ID: "2", Repository: "nginx", Tag: "1.27"}}
	if _, other := m.Update(imagesLoadedMsg{items: nginx}); other == nil {
		t.Error("a new selection must be inspected even while another inspect is in flight")
	}

	// The result releases the selection, so a failed inspect can be retried.
	next, _ = m.Update(imageInspectMsg{ref: "alpine:latest", err: errors.New("boom")})
	m = next.(Model)
	if _, again := m.Update(imagesLoadedMsg{items: items}); again == nil {
		t.Error("a completed inspect must be retryable")
	}
}

func TestInspect_selectionJitterDoesNotDoubleUpTheSameInspect(t *testing.T) {
	// Cursor jitter A->B->A->B inside the debounce window arms one timer per
	// change; when they land in order, only the first may reach the CLI.
	m := imagesModel(t)
	m.imgPanel = m.imgPanel.SetItems([]backend.Image{
		{ID: "1", Repository: "alpine", Tag: "latest"},
		{ID: "2", Repository: "nginx", Tag: "1.27"},
	})

	var timers []tea.Cmd
	for _, delta := range []int{1, -1, 1} {
		next, cmd := m.handleMouseWheel(wheel(delta))
		m = next.(Model)
		if cmd == nil {
			t.Fatal("each selection change must schedule an inspect")
		}
		timers = append(timers, cmd)
	}
	if got := backend.FormatRef(*m.imgPanel.Selected()); got != "nginx:1.27" {
		t.Fatalf("selection settled on %q, want nginx:1.27", got)
	}

	launched := 0
	for i, timer := range timers {
		msg, ok := timer().(inspectSettledMsg)
		if !ok {
			t.Fatalf("timer %d: expected inspectSettledMsg, got %T", i, timer())
		}
		next, load := m.Update(msg)
		m = next.(Model)
		if load != nil {
			launched++
		}
	}
	if launched != 1 {
		t.Errorf("jitter launched %d inspects for one settled selection, want 1", launched)
	}
}

func wheel(delta int) tea.MouseWheelMsg {
	button := tea.MouseWheelDown
	if delta < 0 {
		button = tea.MouseWheelUp
	}
	return tea.MouseWheelMsg(tea.Mouse{Button: button})
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

func TestConfirmPruneModalMode(t *testing.T) {
	cases := []struct {
		name string
		view View
		want string
	}{
		{"containers", ViewContainers, "Prune stopped containers?"},
		{"images", ViewImages, "Prune unused images?"},
		{"volumes", ViewVolumes, "Prune unused volumes?"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := newTestModel()
			m.width, m.height = 100, 30
			m.activeView = tc.view
			m.focus = FocusList
			next, _ := m.handleKey(keyMsg("P"))
			m = next.(Model)
			if m.mode != modeConfirmDelete {
				t.Fatalf("mode=%v want confirm before prune", m.mode)
			}
			got := ansi.Strip(viewString(m.View()))
			if !strings.Contains(got, tc.want) {
				t.Fatalf("confirm modal must ask %q, got %q", tc.want, got)
			}
			next, _ = m.handleKey(keyMsg("n"))
			m = next.(Model)
			if m.mode != modeBrowse {
				t.Fatalf("cancel should return to browse, got %v", m.mode)
			}
		})
	}
}

func TestConfirmPrune_actionMenuPath(t *testing.T) {
	m := newTestModel()
	m.width, m.height = 100, 30
	m.focus = FocusList
	next, _ := m.handleKey(keyMsg("x"))
	m = next.(Model)
	if m.mode != modeActions {
		t.Fatalf("mode=%v want actions", m.mode)
	}
	// Containers menu: Start, Stop, Restart, Logs, Shell, Exec…, Prune stopped (idx 0..6).
	next, _ = m.handleKey(keyMsg("j"))
	m = next.(Model)
	for range 5 {
		next, _ = m.handleKey(keyMsg("j"))
		m = next.(Model)
	}
	next, _ = m.handleKey(keyMsg("enter"))
	m = next.(Model)
	if m.mode != modeConfirmDelete {
		t.Fatalf("prune from action menu must confirm, mode=%v", m.mode)
	}
	got := ansi.Strip(viewString(m.View()))
	if !strings.Contains(got, "Prune stopped containers?") {
		t.Fatalf("confirm modal must ask prune question, got %q", got)
	}
}

func TestConfirmStop_configOff(t *testing.T) {
	m := newTestModel()
	m.width, m.height = 100, 30
	m.cfg.ConfirmStop = false
	m.cntPanel = m.cntPanel.SetItems([]backend.Container{{ID: "abc", Name: "web", Status: "running"}})
	next, _ := m.handleKey(keyMsg("s"))
	m = next.(Model)
	if m.mode == modeConfirmDelete {
		t.Fatalf("confirm_stop off must stop immediately, mode=%v", m.mode)
	}
	if m.status == "nothing selected" {
		t.Fatal("stop should have selected the container")
	}
}

func TestConfirmStop_configOn(t *testing.T) {
	m := newTestModel()
	m.width, m.height = 100, 30
	m.cfg.ConfirmStop = true
	m.cntPanel = m.cntPanel.SetItems([]backend.Container{{ID: "abc", Name: "web", Status: "running"}})
	next, _ := m.handleKey(keyMsg("s"))
	m = next.(Model)
	if m.mode != modeConfirmDelete {
		t.Fatalf("confirm_stop on must open confirm, mode=%v", m.mode)
	}
	got := ansi.Strip(viewString(m.View()))
	if !strings.Contains(got, "Stop web?") {
		t.Fatalf("confirm modal must ask Stop web?, got %q", got)
	}
	next, _ = m.handleKey(keyMsg("n"))
	if next.(Model).mode != modeBrowse {
		t.Fatalf("cancel should return to browse, mode=%v", next.(Model).mode)
	}
}

func TestCustomCommandKeyDispatch(t *testing.T) {
	m := newTestModel()
	m.width, m.height = 100, 30
	m.cfg.CustomCommands = []config.CustomCommand{{Name: "inspect", Key: "z", Command: "container inspect {{.ID}}"}}
	m.cntPanel = m.cntPanel.SetItems([]backend.Container{{ID: "vessel-probe", Name: "web", Status: "running"}})

	next, cmd := m.handleKey(keyMsg("z"))
	m = next.(Model)
	if m.mode != modeBrowse {
		t.Fatalf("custom key must run in browse, mode=%v", m.mode)
	}
	if !strings.HasPrefix(m.status, "custom:") {
		t.Fatalf("custom command should have dispatched, status=%q", m.status)
	}
	if cmd == nil {
		t.Fatal("custom key should return a command")
	}
}

func TestCustomCommandConfiguredKeyOverridesDefault(t *testing.T) {
	m := newTestModel()
	m.width, m.height = 100, 30
	// User binds 'y' (normally yank) to a custom command; the configured key wins.
	m.cfg.CustomCommands = []config.CustomCommand{{Name: "redefine", Key: "y", Command: "echo redefined"}}
	m.cntPanel = m.cntPanel.SetItems([]backend.Container{{ID: "abc", Name: "web", Status: "running"}})
	next, _ := m.handleKey(keyMsg("y"))
	m = next.(Model)
	if !strings.HasPrefix(m.status, "custom:") {
		t.Fatalf("configured custom key must shadow builtin, status=%q", m.status)
	}
}

func TestFooterView_servicesDownHint(t *testing.T) {
	m := newTestModel()
	m.width, m.height = 100, 30
	m.lastErr = errors.New("container [image prune]: exit status 1 (stderr: Error: Plugins are unavailable. Start the container system services and retry:" +
		"\n\n    container system start\n)")
	out := ansi.Strip(viewString(m.View()))
	if !strings.Contains(out, "container system start") {
		t.Fatalf("services-down error must surface the hint, footer=%q", out)
	}
	if !strings.Contains(out, "…") {
		t.Fatalf("raw error must be truncated before the hint rather than the hint pushed off, footer=%q", out)
	}
	if !strings.HasSuffix(strings.TrimSpace(out), "to start services") {
		t.Fatalf("hint must be the last visible text in the footer, footer=%q", out)
	}
}

func TestFooterView_noHintForOtherErrors(t *testing.T) {
	m := newTestModel()
	m.width, m.height = 100, 30
	m.lastErr = errors.New("container [list]: exit status 1 (stderr: boom)")
	out := ansi.Strip(viewString(m.View()))
	if strings.Contains(out, "system start") {
		t.Fatalf("unrelated error must not get the service hint, footer=%q", out)
	}
}

func TestApplyContainersLoaded_keepsServicesDownHint(t *testing.T) {
	servicesDown := servicesDownErr()

	m := newTestModel()
	m.width, m.height = 100, 30
	m.lastErr = servicesDown
	// A successful `container list` poll is a top-level verb that works while
	// services are down; it must not wipe the services-down hint.
	next, _ := m.applyContainersLoaded(containersLoadedMsg{items: nil, err: nil})
	m = next.(Model)
	if m.lastErr == nil {
		t.Fatal("a successful top-level list must not clear a services-down hint")
	}
	out := ansi.Strip(viewString(m.View()))
	if !strings.Contains(out, "container system start") {
		t.Fatalf("services-down hint must survive the next successful poll, footer=%q", out)
	}
}

func servicesDownErr() error {
	return errors.New("container [image prune]: exit status 1 (stderr: Error: Plugins are unavailable. " +
		"Start the container system services and retry:\n\n    container system start\n)")
}

func TestFooterView_freshStatusNotMaskedBySickyServicesDownError(t *testing.T) {
	m := newTestModel()
	m.width, m.height = 100, 30
	m.setLastErr(servicesDownErr())
	m.setStatus("copied container id")
	out := ansi.Strip(viewString(m.View()))
	if !strings.Contains(out, "copied container id") {
		t.Fatalf("a fresh status must not be masked by a sticky services-down error, footer=%q", out)
	}
	if strings.Contains(out, "system start") {
		t.Fatalf("the sticky error must not render while a fresher status is set, footer=%q", out)
	}
	// The error itself must not have been discarded: once status is cleared,
	// the hint is still there to show.
	m.setStatus("")
	out = ansi.Strip(viewString(m.View()))
	if !strings.Contains(out, "container system start") {
		t.Fatalf("clearing status must reveal the still-sticky services-down hint, footer=%q", out)
	}
}

func TestFooterView_staleStatusDoesNotMaskLaterError(t *testing.T) {
	m := newTestModel()
	m.width, m.height = 100, 30
	// A layout keypress leaves a status behind that nothing ever clears; the
	// poll failure that follows it must still reach the footer.
	m.setStatus("layout wide")
	next, _ := m.applyContainersLoaded(containersLoadedMsg{err: servicesDownErr()})
	m = next.(Model)
	out := ansi.Strip(viewString(m.View()))
	if !strings.Contains(out, "container system start") {
		t.Fatalf("a failure after a status must surface its hint, footer=%q", out)
	}
	if strings.Contains(out, "layout wide") {
		t.Fatalf("the stale status must not hold the footer over a newer error, footer=%q", out)
	}
	// And a status set after that error takes the line back.
	m.setStatus("copied container id")
	out = ansi.Strip(viewString(m.View()))
	if !strings.Contains(out, "copied container id") {
		t.Fatalf("a status newer than the error must render, footer=%q", out)
	}
}

func TestFooterView_untouchedGenerationsPreferError(t *testing.T) {
	m := newTestModel()
	m.width, m.height = 100, 30
	// Neither field has been stamped (both generations zero): an error present
	// alongside a status still wins, as it did before recency was introduced.
	m.lastErr = errors.New("container [list]: exit status 1 (stderr: boom)")
	m.status = "copied container id"
	out := ansi.Strip(viewString(m.View()))
	if !strings.Contains(out, "boom") {
		t.Fatalf("an unstamped error must still render, footer=%q", out)
	}
	if strings.Contains(out, "copied container id") {
		t.Fatalf("an unstamped status must not win the tie, footer=%q", out)
	}
}

func TestYankSelected_clipboardErrorNotMaskedByEarlierStatus(t *testing.T) {
	m := newTestModel()
	m.width, m.height = 100, 30
	m.setStatus("copied abc")
	m.setLastErr(errors.New("clipboard unavailable"))
	out := ansi.Strip(viewString(m.View()))
	if !strings.Contains(out, "clipboard unavailable") {
		t.Fatalf("a clipboard failure must not stay hidden behind the previous copy status, footer=%q", out)
	}
}

func TestActionDoneMsg_successClearsServicesDownHint(t *testing.T) {
	m := newTestModel()
	m.width, m.height = 100, 30
	m.setLastErr(servicesDownErr())
	// A plugin-gated verb (image prune, volume create/prune) actually succeeding
	// is real evidence the services came back - unlike an unrelated container
	// list poll, which TestApplyContainersLoaded_keepsServicesDownHint asserts
	// must NOT clear it.
	next, _ := m.Update(actionDoneMsg{msg: "pruned images"})
	m = next.(Model)
	if m.lastErr != nil {
		t.Fatalf("a successful action must clear a services-down hint, got %v", m.lastErr)
	}
}

func TestApplyContainersLoaded_clearsOtherErrors(t *testing.T) {
	m := newTestModel()
	m.width, m.height = 100, 30
	m.lastErr = errors.New("container [list]: exit status 1 (stderr: boom)")
	next, _ := m.applyContainersLoaded(containersLoadedMsg{items: nil, err: nil})
	m = next.(Model)
	if m.lastErr != nil {
		t.Fatalf("a successful poll must clear a non-services-down error, got %v", m.lastErr)
	}
}

func TestHelpBindingsCoverAllKeys(t *testing.T) {
	tokens := map[string]bool{}
	for _, v := range []View{ViewContainers, ViewImages, ViewVolumes} {
		for _, b := range helpBindings(v, FocusList, modeBrowse, DefaultKeyMap(), nil) {
			for _, tok := range helpKeyTokens(b.key) {
				tokens[tok] = true
			}
		}
	}
	km := DefaultKeyMap()
	rt := reflect.TypeOf(km)
	rv := reflect.ValueOf(km)
	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)
		val := rv.Field(i).String()
		if val == "" {
			continue
		}
		if !tokens[val] {
			t.Fatalf("KeyMap.%s (%q) has no help entry in any view", field.Name, val)
		}
	}
}

func TestHelpKeyTokens_separatorIsNotAKey(t *testing.T) {
	got := helpKeyTokens("g / G")
	want := []string{"g", "G"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("the / between keys is a separator, not a key: got %q want %q", got, want)
	}
	if got := helpKeyTokens("/"); !reflect.DeepEqual(got, []string{"/"}) {
		t.Fatalf("the filter row documents the / key itself, got %q", got)
	}
	if got := helpKeyTokens("space"); !reflect.DeepEqual(got, []string{"space"}) {
		t.Fatalf("the space key is spelled %q by a keypress, got %q", "space", got)
	}
}

func TestHelpBindingsIncludeReachableKeys(t *testing.T) {
	var sb strings.Builder
	for _, v := range []View{ViewContainers, ViewImages, ViewVolumes} {
		for _, b := range helpBindings(v, FocusList, modeBrowse, DefaultKeyMap(), nil) {
			sb.WriteString(b.key + " ")
			sb.WriteString(b.desc + "\n")
		}
	}
	all := sb.String()
	for _, k := range []string{"`", "ctrl+c", "esc", "enter", "1 2 3"} {
		if !strings.Contains(all, k) {
			t.Fatalf("reachable key %q missing from help", k)
		}
	}
}

func TestHelpBindings_shadowedRowNeverMislabelsSiblingKeys(t *testing.T) {
	custom := []config.CustomCommand{{Name: "up", Key: "u", Command: "echo up"}}
	for _, b := range helpBindings(ViewContainers, FocusList, modeBrowse, DefaultKeyMap(), custom) {
		keys := helpKeyTokens(b.key)
		for _, k := range keys {
			if k == "u" && !strings.HasPrefix(b.desc, "custom:") {
				t.Fatalf("shadowed key u still documented as %q", b.desc)
			}
		}
		if len(keys) == 1 && keys[0] == "r" && !strings.Contains(b.desc, "restart") {
			t.Fatalf("r documents restart, help says %q", b.desc)
		}
	}
	for _, want := range []helpRow{{"s", "stop"}, {"r", "restart"}} {
		found := false
		for _, b := range helpBindings(ViewContainers, FocusList, modeBrowse, DefaultKeyMap(), custom) {
			if b.key == want.key && strings.Contains(b.desc, want.desc) {
				found = true
			}
		}
		if !found {
			t.Fatalf("key %q must keep its own %q entry when a sibling key is shadowed", want.key, want.desc)
		}
	}
}

func TestCustomCommandWithoutCommandKeepsBuiltin(t *testing.T) {
	custom := []config.CustomCommand{{Name: "empty", Key: "y", Command: ""}}

	var listed []string
	for _, b := range helpBindings(ViewContainers, FocusList, modeBrowse, DefaultKeyMap(), custom) {
		listed = append(listed, b.key+"\t"+b.desc)
	}
	all := strings.Join(listed, "\n")
	if strings.Contains(all, "custom: empty") {
		t.Fatalf("a custom command with no command never fires, so help must omit it:\n%s", all)
	}
	if !strings.Contains(all, "yank id/name to clipboard") {
		t.Fatalf("the built-in y must keep its help entry:\n%s", all)
	}

	m := newTestModel()
	m.width, m.height = 100, 30
	m.focus = FocusList
	m.cfg.CustomCommands = custom
	m.cntPanel = m.cntPanel.SetItems([]backend.Container{{ID: "abc", Name: "web"}})
	next, _ := m.handleKey(keyMsg("y"))
	if status := next.(Model).status; strings.HasPrefix(status, "custom:") {
		t.Fatalf("empty custom command must fall through to the built-in, status=%q", status)
	}
}

func TestActionsKeyIsReservedFromCustomCommands(t *testing.T) {
	m := newTestModel()
	m.width, m.height = 100, 30
	m.focus = FocusList
	m.cfg.CustomCommands = []config.CustomCommand{{Name: "steal", Key: "x", Command: "echo nope"}}
	m.cntPanel = m.cntPanel.SetItems([]backend.Container{{ID: "abc", Name: "web", Status: "running"}})

	if cmd := m.customCommandForKey("x"); cmd != "" {
		t.Fatalf("the action-menu key must not dispatch a custom command, got %q", cmd)
	}
	next, _ := m.handleKey(keyMsg("x"))
	m = next.(Model)
	if m.mode != modeActions {
		t.Fatalf("x must still open the action menu, mode=%v", m.mode)
	}
	if strings.HasPrefix(m.status, "custom:") {
		t.Fatalf("a custom command bound to x must not have run, status=%q", m.status)
	}
	for _, row := range helpBindings(m.activeView, m.focus, m.mode, m.keys, m.cfg.CustomCommands) {
		if strings.Contains(row.desc, "steal") {
			t.Fatalf("help must not advertise a custom command that can never fire: %+v", row)
		}
	}
}

func TestCustomCommandKeyDoesNotShadowNavigation(t *testing.T) {
	m := newTestModel()
	m.width, m.height = 100, 30
	m.focus = FocusList
	m.cfg.CustomCommands = []config.CustomCommand{{Name: "nav", Key: "j", Command: "echo nope"}}
	m.cntPanel = m.cntPanel.SetItems([]backend.Container{
		{ID: "a", Name: "one"}, {ID: "b", Name: "two"}, {ID: "c", Name: "three"},
	})
	next, _ := m.handleKey(keyMsg("j"))
	m = next.(Model)
	if strings.HasPrefix(m.status, "custom:") {
		t.Fatalf("navigation key must not be shadowed by a custom command, status=%q", m.status)
	}
	if m.cntPanel.Cursor() != 1 {
		t.Fatalf("list must still scroll with j, cursor=%d", m.cntPanel.Cursor())
	}
}

func TestHelpBindings_customCommands(t *testing.T) {
	custom := []config.CustomCommand{
		{Name: "redefine", Key: "y", Command: "echo redefined"},
		{Name: "nav", Key: "j", Command: "echo nope"},
	}
	var rows []string
	for _, b := range helpBindings(ViewContainers, FocusList, modeBrowse, DefaultKeyMap(), custom) {
		rows = append(rows, b.key+"\t"+b.desc)
	}
	all := strings.Join(rows, "\n")
	if !strings.Contains(all, "custom: redefine") {
		t.Fatalf("configured custom key must appear in help, got:\n%s", all)
	}
	if strings.Contains(all, "yank id/name to clipboard") {
		t.Fatalf("shadowed builtin must not keep its old help entry, got:\n%s", all)
	}
	if strings.Contains(all, "custom: nav") {
		t.Fatalf("custom command on a reserved key never fires, so help must omit it, got:\n%s", all)
	}
	if !strings.Contains(all, "move up / down (in list)") {
		t.Fatalf("navigation help entry must survive a custom command bound to j, got:\n%s", all)
	}
}

func TestFooterView_servicesDownStaysOneLine(t *testing.T) {
	for _, w := range []int{60, 100, 183, 200} {
		m := newTestModel()
		m.width, m.height = w, 30
		m.lastErr = errors.New("container [image prune]: exit status 1 (stderr: Error: Plugins are unavailable. " +
			"Start the container system services and retry:\n\n    container system start\n)")
		footer := m.footerView()
		if got := strings.Count(footer, "\n") + 1; got != 1 {
			t.Fatalf("width=%d: footer must render on one line, got %d lines: %q", w, got, footer)
		}
		if w >= 100 && !strings.Contains(ansi.Strip(footer), "container system start") {
			t.Fatalf("width=%d: hint must survive truncation, footer=%q", w, footer)
		}
	}
}

func TestConfirmPrune_keepsGlobalBudgetAndReportsProgress(t *testing.T) {
	if _, _, timeout := pendingAction(pruneImages); timeout != globalTimeout {
		t.Fatalf("prune must keep the global budget, got %v want %v", timeout, globalTimeout)
	}
	if _, _, timeout := pendingAction(deleteContainers); timeout != confirmTimeout {
		t.Fatalf("single-resource delete budget = %v, want %v", timeout, confirmTimeout)
	}

	m := newTestModel()
	m.width, m.height = 100, 30
	m.activeView = ViewImages
	m.focus = FocusList
	next, _ := m.handleKey(keyMsg("P"))
	m = next.(Model)
	next, cmd := m.handleKey(keyMsg("y"))
	m = next.(Model)
	if cmd == nil {
		t.Fatal("confirming a prune should return a command")
	}
	if !strings.Contains(m.status, "prune") {
		t.Fatalf("running prune must show progress in the footer, status=%q", m.status)
	}
}

func TestHelpView_fitsTerminalHeight(t *testing.T) {
	custom := []config.CustomCommand{
		{Name: "one", Key: "z", Command: "echo 1"},
		{Name: "two", Key: "Z", Command: "echo 2"},
	}
	for _, size := range []struct{ w, h int }{{80, 24}, {80, 12}, {120, 40}, {200, 60}} {
		for _, v := range []View{ViewContainers, ViewImages, ViewVolumes} {
			m := newTestModel()
			m.width, m.height = size.w, size.h
			m.activeView = v
			m.cfg.CustomCommands = custom
			m.showHelp = true
			got := lipgloss.Height(viewString(m.View()))
			if got > size.h {
				t.Fatalf("%dx%d view=%d: help renders %d lines, alt screen only shows %d",
					size.w, size.h, v, got, size.h)
			}
		}
	}
}

func TestHelpView_scrollsToTheLastBinding(t *testing.T) {
	m := newTestModel()
	m.width, m.height = 80, 24
	m.showHelp = true
	bindings := m.helpBindings()
	last := bindings[len(bindings)-1]

	first := ansi.Strip(viewString(m.View()))
	if strings.Contains(first, last.desc) {
		t.Skip("help already fits, nothing to scroll")
	}
	if !strings.Contains(first, bindings[0].desc) {
		t.Fatalf("help must start at the first binding, got %q", first)
	}

	next, _ := m.handleKey(keyMsg("G"))
	m = next.(Model)
	bottom := ansi.Strip(viewString(m.View()))
	if !strings.Contains(bottom, last.desc) {
		t.Fatalf("every binding must be reachable in help, %q missing after scrolling:\n%s", last.desc, bottom)
	}
	if lines := lipgloss.Height(viewString(m.View())); lines > m.height {
		t.Fatalf("scrolled help renders %d lines for a %d-row screen", lines, m.height)
	}

	next, _ = m.handleKey(keyMsg("?"))
	next, _ = next.(Model).handleKey(keyMsg("?"))
	if got := next.(Model).helpScroll; got != 0 {
		t.Fatalf("reopening help must start at the top, scroll=%d", got)
	}
}

func TestCustomCommandKeySpellings(t *testing.T) {
	cases := []struct {
		name      string
		configKey string
		press     string
	}{
		{"literal space", " ", "space"},
		{"named space", "space", "space"},
		{"dash modifier", "ctrl-z", "ctrl+z"},
		{"uppercase name", "Enter", "enter"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			custom := []config.CustomCommand{{Name: "probe", Key: tc.configKey, Command: "echo probe"}}
			if got := customCommandFor(custom, DefaultKeyMap(), tc.press); got != "echo probe" {
				t.Fatalf("config key %q must fire on %q, got %q", tc.configKey, tc.press, got)
			}
			rows := map[string]int{}
			for _, b := range helpBindings(ViewContainers, FocusList, modeBrowse, DefaultKeyMap(), custom) {
				for _, k := range helpKeyTokens(b.key) {
					rows[k]++
				}
			}
			if rows[tc.press] != 1 {
				t.Fatalf("key %q must have exactly one help row, got %d", tc.press, rows[tc.press])
			}
		})
	}
}

func TestCustomCommandUnusableKeyIsNotAdvertised(t *testing.T) {
	custom := []config.CustomCommand{{Name: "phantom", Key: "not-a-key", Command: "echo nope"}}
	for _, b := range helpBindings(ViewContainers, FocusList, modeBrowse, DefaultKeyMap(), custom) {
		if strings.Contains(b.desc, "phantom") {
			t.Fatalf("a key no keypress produces must not appear in help: %q -> %q", b.key, b.desc)
		}
	}
}

func TestStopTimeoutMatchesUnconfirmedStop(t *testing.T) {
	if _, _, timeout := pendingAction(stopContainer); timeout != lifecycleTimeout {
		t.Fatalf("confirming a stop must not change its budget: got %v want %v", timeout, lifecycleTimeout)
	}
}

// newTestModel is New() with the developer's ~/.config/vessel/config.toml
// dropped, so assertions never depend on the host's dotfiles.
func newTestModel() Model {
	m := New()
	m.cfg = config.Config{}
	return m
}

func TestFooterView_alwaysOneLine(t *testing.T) {
	servicesDown := errors.New("container [image prune]: exit status 1 (stderr: Error: Plugins are unavailable. " +
		"Start the container system services and retry:\n\n    container system start\n)")
	states := []struct {
		name  string
		apply func(Model) Model
	}{
		{"resting key hints", func(m Model) Model { return m }},
		{"status", func(m Model) Model {
			m.status = "custom: " + strings.Repeat("echo hello ", 30)
			return m
		}},
		{"multi-line status", func(m Model) Model {
			m.status = "custom ok\nsecond line\tthird"
			return m
		}},
		{"services down", func(m Model) Model {
			m.lastErr = servicesDown
			return m
		}},
		{"plain error", func(m Model) Model {
			m.lastErr = errors.New("container [list]: exit status 1 (stderr: " + strings.Repeat("boom ", 40) + ")")
			return m
		}},
		{"wide-rune status", func(m Model) Model {
			m.status = "custom: " + strings.Repeat("世界", 60)
			return m
		}},
		{"wide-rune error", func(m Model) Model {
			m.lastErr = errors.New("container [volume create " + strings.Repeat("世界", 60) + "]: exit status 1")
			return m
		}},
		{"wide-rune services-down error", func(m Model) Model {
			m.lastErr = errors.New("container [volume create " + strings.Repeat("世界", 60) +
				"]: exit status 1 (stderr: Error: Plugins are unavailable.\n\n    container system start\n)")
			return m
		}},
	}
	for _, st := range states {
		for _, w := range []int{60, 72, 80, 100, 183, 200} {
			for _, v := range []View{ViewContainers, ViewImages, ViewVolumes} {
				m := newTestModel()
				m.width, m.height = w, 24
				m.activeView = v
				m = st.apply(m)
				footer := m.footerView()
				if lines := lipgloss.Height(footer); lines != 1 {
					t.Fatalf("%s at width %d view %d: footer must be one row, got %d: %q",
						st.name, w, v, lines, footer)
				}
			}
		}
	}
}

func TestHelpView_fitsWithWideRuneCustomCommands(t *testing.T) {
	custom := []config.CustomCommand{{Name: strings.Repeat("世界", 40), Key: "z", Command: "echo wide"}}
	for _, size := range []struct{ w, h int }{{60, 24}, {80, 24}, {120, 24}} {
		m := newTestModel()
		m.width, m.height = size.w, size.h
		m.cfg.CustomCommands = custom
		m.showHelp = true
		// The custom command is the last row, so scroll to it.
		next, _ := m.handleKey(keyMsg("G"))
		m = next.(Model)
		out := ansi.Strip(viewString(m.View()))
		if !strings.Contains(out, "custom:") {
			t.Fatalf("%dx%d: the custom command row must be on screen, got:\n%s", size.w, size.h, out)
		}
		if got := lipgloss.Height(viewString(m.View())); got > size.h {
			t.Fatalf("%dx%d: help renders %d lines for a %d-row screen", size.w, size.h, got, size.h)
		}
	}
}

func TestHelpBindings_keyStillLiveInLogViewKeepsARow(t *testing.T) {
	km := DefaultKeyMap()
	for _, key := range []string{km.Follow, km.Yank} {
		custom := []config.CustomCommand{{Name: "taken", Key: key, Command: "echo taken"}}
		var got []string
		for _, b := range helpBindings(ViewContainers, FocusList, modeBrowse, km, custom) {
			for _, t := range helpKeyTokens(b.key) {
				if t == key {
					got = append(got, b.desc)
				}
			}
		}
		if len(got) != 2 {
			t.Fatalf("key %q: want a custom row plus its surviving log-view row, got %q", key, got)
		}
		var custRow, logRow bool
		for _, d := range got {
			if strings.HasPrefix(d, "custom:") {
				custRow = true
			}
			if strings.Contains(d, "log view") {
				logRow = true
			}
		}
		if !custRow || !logRow {
			t.Fatalf("key %q: help must show both the custom command and what it still does in the log view, got %q", key, got)
		}
	}
}

func TestCustomCommandKeyModifierSpellings(t *testing.T) {
	t.Run("modifier order is canonical", func(t *testing.T) {
		custom := []config.CustomCommand{{Name: "probe", Key: "shift-ctrl-a", Command: "echo probe"}}
		if got := customCommandFor(custom, DefaultKeyMap(), "ctrl+shift+a"); got != "echo probe" {
			t.Fatalf("a keypress spells modifiers ctrl+shift+…, so that config key must fire on it, got %q", got)
		}
	})
	t.Run("shifted character is unmatchable", func(t *testing.T) {
		custom := []config.CustomCommand{{Name: "phantom", Key: "shift+z", Command: "echo nope"}}
		if got := customCommandFor(custom, DefaultKeyMap(), "Z"); got != "" {
			t.Fatalf("shift+z is not what a keypress reports; it must not claim to fire, got %q", got)
		}
		for _, b := range helpBindings(ViewContainers, FocusList, modeBrowse, DefaultKeyMap(), custom) {
			if strings.Contains(b.desc, "phantom") {
				t.Fatalf("a binding that can never fire must not appear in help: %q -> %q", b.key, b.desc)
			}
		}
	})
	t.Run("modified character is lowercased", func(t *testing.T) {
		custom := []config.CustomCommand{{Name: "probe", Key: "ctrl+Z", Command: "echo probe"}}
		if got := customCommandFor(custom, DefaultKeyMap(), "ctrl+z"); got != "echo probe" {
			t.Fatalf("a modifier suppresses the typed text, so ctrl+Z must fire on ctrl+z, got %q", got)
		}
		rows := 0
		for _, b := range helpBindings(ViewContainers, FocusList, modeBrowse, DefaultKeyMap(), custom) {
			for _, k := range helpKeyTokens(b.key) {
				if k == "ctrl+Z" {
					t.Fatalf("help must advertise the key a keypress reports, not %q", k)
				}
				if k == "ctrl+z" {
					rows++
				}
			}
		}
		if rows != 1 {
			t.Fatalf("want one ctrl+z help row, got %d", rows)
		}
	})
	t.Run("shift plus a named key stays usable", func(t *testing.T) {
		custom := []config.CustomCommand{{Name: "probe", Key: "shift+tab", Command: "echo probe"}}
		if got := customCommandFor(custom, DefaultKeyMap(), "shift+tab"); got != "echo probe" {
			t.Fatalf("shift+tab is a real keypress spelling, got %q", got)
		}
	})
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
	m := newTestModel()
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
	m := newTestModel()
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
	m := newTestModel()
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
	m := newTestModel()
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
	m := newTestModel()
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
	m := newTestModel()
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
	m := newTestModel()
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
	m := newTestModel()
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
	m := newTestModel().withKeys(rebound("m"))
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
	m := newTestModel()
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
	m := newTestModel()
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

// A prune kind that has no spec of its own would ask about, and then sweep,
// whichever store the lookup fell back to. Every kind isPrune reports true for
// must therefore own a distinct question.
func TestEveryPruneKindAsksAboutItsOwnStore(t *testing.T) {
	asked := map[string]deleteKind{}
	prunes := 0
	for k := deleteKind(0); k < deleteKind(64); k++ {
		if !k.isPrune() {
			continue
		}
		prunes++
		m := newTestModel()
		m.pendingKind = k
		q := m.confirmQuestion()
		if q == "" {
			t.Fatalf("prune kind %d asks nothing before sweeping a store", k)
		}
		if prev, dup := asked[q]; dup {
			t.Fatalf("prune kinds %d and %d both ask %q: one of them has no spec and would sweep the other's store", k, prev, q)
		}
		asked[q] = k
		if label, done, timeout := pendingAction(k); label == "" || done != "pruned" || timeout != globalTimeout {
			t.Fatalf("prune kind %d reports (%q, %q, %v) instead of its own prune action", k, label, done, timeout)
		}
	}
	if prunes != 3 {
		t.Fatalf("expected the three prune kinds, found %d", prunes)
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

func imagesModelWithItems(t *testing.T, items []backend.Image) Model {
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
	m := imagesModelWithItems(t, nil)
	for _, label := range []string{"Tag…", "Save…", "Load…", "Push"} {
		if findAction(m.buildActions(), label) == nil {
			t.Fatalf("images actions menu missing %q", label)
		}
	}
}

func TestImagesAction_Tag_flow(t *testing.T) {
	m := imagesModelWithItems(t, []backend.Image{{ID: "sha256:abc", Repository: "alpine", Tag: "latest"}})
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
	m := imagesModelWithItems(t, []backend.Image{{ID: "sha256:abc", Repository: "alpine", Tag: "latest"}})
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
	m := imagesModelWithItems(t, nil)
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
	m := imagesModelWithItems(t, nil)
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

// beginLoadFailure drives the images Load… action to its missing-file
// failure and returns the resulting model, mirroring
// TestImagesAction_Load_missingFile up to the actionDoneMsg Update.
func beginLoadFailure(t *testing.T, m Model) Model {
	t.Helper()
	run := findAction(m.buildActions(), "Load…")
	if run == nil {
		t.Fatal("Load action missing")
	}
	next, _ := run(m)
	res, cmd := next.handlePrompt(promptDoneMsg{kind: "load from", text: "/no/such/archive.tar"})
	done := cmd().(actionDoneMsg)
	if done.err == nil {
		t.Fatal("expected a missing-file error")
	}
	out, _ := res.Update(done)
	return out.(Model)
}

// TestActionDoneMsg_errorSurvivesFollowingRefresh reproduces the reported
// bug: images view -> Load… -> a nonexistent path used to show the failure
// for one frame, then the containers refresh actionDoneMsg triggers wiped
// it via applyContainersLoaded's unconditional clear-on-success.
func TestActionDoneMsg_errorSurvivesFollowingRefresh(t *testing.T) {
	m := beginLoadFailure(t, imagesModelWithItems(t, nil))
	if m.lastErr == nil || !strings.Contains(m.lastErr.Error(), "no such file") {
		t.Fatalf("precondition: action failure must set the footer error, got: %v", m.lastErr)
	}

	// The refreshCmd the failed action itself triggers must not clear it.
	next, _ := m.applyContainersLoaded(containersLoadedMsg{items: nil, err: nil})
	m = next.(Model)
	if m.lastErr == nil || !strings.Contains(m.lastErr.Error(), "no such file") {
		t.Fatalf("action error must survive the refresh that follows it, got: %v", m.lastErr)
	}
}

// TestActionDoneMsg_errorSurvivesPeriodicTick extends the above across a
// second, later refresh - the kind a periodic tick issues once the app is
// idle - to prove the error is not merely delayed by one cycle.
func TestActionDoneMsg_errorSurvivesPeriodicTick(t *testing.T) {
	m := beginLoadFailure(t, imagesModelWithItems(t, nil))
	next, _ := m.applyContainersLoaded(containersLoadedMsg{items: nil, err: nil})
	m = next.(Model)

	next, _ = m.applyContainersLoaded(containersLoadedMsg{items: nil, err: nil})
	m = next.(Model)
	if m.lastErr == nil || !strings.Contains(m.lastErr.Error(), "no such file") {
		t.Fatalf("action error must survive a further periodic-tick refresh, got: %v", m.lastErr)
	}
}

// TestActionDoneMsg_successReplacesDurableError checks the first of the two
// legitimate ways to supersede a durable action error: a later action
// actually succeeding.
func TestActionDoneMsg_successReplacesDurableError(t *testing.T) {
	m := beginLoadFailure(t, imagesModelWithItems(t, nil))
	next, _ := m.Update(actionDoneMsg{msg: "load ok"})
	m = next.(Model)
	if m.lastErr != nil {
		t.Fatalf("a successful subsequent action must replace a durable error, got: %v", m.lastErr)
	}
	if m.status != "load ok" {
		t.Fatalf("successful action status: got %q", m.status)
	}
}

// TestFooterView_newerStatusReplacesDurableError checks the second
// legitimate supersession: a newer deliberate status message takes the
// footer even while the durable error is still present in the model (it is
// not discarded, only outranked - matching the sticky services-down case).
func TestFooterView_newerStatusReplacesDurableError(t *testing.T) {
	m := beginLoadFailure(t, imagesModelWithItems(t, nil))
	m.setStatus("copied alpine:latest")
	out := ansi.Strip(viewString(m.View()))
	if !strings.Contains(out, "copied alpine:latest") {
		t.Fatalf("a fresher status must win the footer over a durable error, got: %q", out)
	}
	if m.lastErr == nil {
		t.Fatal("the durable error must still be held, only outranked, not discarded")
	}
}

// TestFooterView_durableErrorAt60x12KeepsIdentityRows covers the smallest
// supported terminal: the persisted error must render in the footer without
// pushing the detail pane's identity rows (repository/tag) out of its
// row budget - the sharp edge AGENTS.md documents for uiutil.Pane.Add.
func TestFooterView_durableErrorAt60x12KeepsIdentityRows(t *testing.T) {
	m := beginLoadFailure(t, imagesModelWithItems(t, []backend.Image{
		{ID: "sha256:abc", Repository: "alpine", Tag: "latest"},
	}))
	m.width, m.height = 60, 12
	out := ansi.Strip(viewString(m.View()))
	if !strings.Contains(out, "no such file") {
		t.Fatalf("footer must render the durable error at 60x12, got: %q", out)
	}
	if !strings.Contains(out, "alpine") {
		t.Fatalf("the selected image's reference must still render at 60x12, got: %q", out)
	}
	// The id is rendered only by the detail pane - the list has no ID column -
	// so it is the token that actually distinguishes a surviving identity row
	// from a pane whose rows uiutil.Pane.Add dropped for want of budget.
	if !strings.Contains(out, "sha256:abc") {
		t.Fatalf("detail pane must keep the selected image's ID row at 60x12, got: %q", out)
	}
}

// TestNetworksLoadedMsg_loadErrorIsNotDurable checks that a networks load
// failure behaves like every other load-originated error: it replaces a
// durable action error rather than inheriting its durability, so a later
// successful poll can still self-heal the footer.
func TestNetworksLoadedMsg_loadErrorIsNotDurable(t *testing.T) {
	m := beginLoadFailure(t, imagesModelWithItems(t, nil))
	if !m.errDurable {
		t.Fatal("precondition: the failed action must have set a durable error")
	}

	next, _ := m.Update(networksLoadedMsg{err: errors.New("networks unavailable")})
	m = next.(Model)
	if m.lastErr == nil || !strings.Contains(m.lastErr.Error(), "networks unavailable") {
		t.Fatalf("networks load failure must take the footer, got: %v", m.lastErr)
	}
	if m.errDurable {
		t.Fatal("a load-originated error must not stay durable")
	}

	after, _ := m.applyContainersLoaded(containersLoadedMsg{items: nil, err: nil})
	m = after.(Model)
	if m.lastErr != nil {
		t.Fatalf("a successful poll must clear a load-originated error, got: %v", m.lastErr)
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
	m := beginPush(t, imagesModelWithItems(t, []backend.Image{
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
	m := beginPush(t, imagesModelWithItems(t, []backend.Image{
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
	m := imagesModelWithItems(t, []backend.Image{{ID: "sha256:abc", Repository: "alpine", Tag: ""}})
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
	m := imagesModelWithItems(t, []backend.Image{{ID: "sha256:abc", Repository: "alpine", Tag: "latest"}})
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
	m := imagesModelWithItems(t, []backend.Image{{ID: "sha256:abc", Repository: "alpine", Tag: "latest"}})
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
	m := beginPush(t, imagesModelWithItems(t, []backend.Image{
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
	m := beginPush(t, imagesModelWithItems(t, []backend.Image{
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
		SetNotice("alpine:latest", backend.PushAuthNotice)
	// The geometry mainPanels hands the detail pane on the smallest terminal
	// View() still renders (60x12) — 18x4 once the command log is open, 18x8
	// without it — and on 80x24 with the command log open.
	for _, size := range []struct{ w, h int }{{18, 4}, {18, 8}, {14, 16}, {40, 20}} {
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
		panel := New().imgPanel.SetItems(items).
			SetNotice(backend.FormatRef(items[0]), notice)
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
	m := imagesModelWithItems(t, []backend.Image{{ID: "sha256:abc", Repository: "alpine", Tag: "latest"}})
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
	m := beginPush(t, imagesModelWithItems(t, []backend.Image{
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

// twoImages gives the panel a second row to move the cursor onto, which is the
// only way to tell an image-scoped notice from a panel-wide one.
func twoImages() []backend.Image {
	return []backend.Image{
		{ID: "sha256:aaa", Repository: "alpine", Tag: "latest"},
		{ID: "sha256:bbb", Repository: "busybox", Tag: "v1"},
	}
}

// pushRefusedOnFirstImage pushes the highlighted (first) image and lets the
// registry refuse it, leaving the panel holding that image's notice.
func pushRefusedOnFirstImage(t *testing.T, items []backend.Image) Model {
	t.Helper()
	t.Setenv("FAKE_CONTAINER_FAIL_PUSH", "auth")
	m := beginPush(t, imagesModelWithItems(t, items))
	next, cmd := m.handleKey(keyMsg("y"))
	done := cmd().(actionDoneMsg)
	if done.err == nil {
		t.Fatal("expected the push to be refused")
	}
	out, _ := next.(Model).Update(done)
	om := out.(Model)
	if !strings.Contains(squash(ansi.Strip(om.imgPanel.DetailView(40, 20))), squash("container registry login")) {
		t.Fatal("precondition: the refusal should have set a notice on the pushed image")
	}
	return om
}

func TestImagesDetail_noticeDoesNotFollowTheCursorToAnotherImage(t *testing.T) {
	om := pushRefusedOnFirstImage(t, twoImages())
	other := om.imgPanel.MoveBy(1)
	if got := backend.FormatRef(*other.Selected()); got != "busybox:v1" {
		t.Fatalf("precondition: the cursor should be on the second image, got %q", got)
	}
	detail := squash(ansi.Strip(other.DetailView(40, 20)))
	if strings.Contains(detail, squash("container registry login")) {
		t.Fatalf("a refusal reported for alpine:latest must not show on busybox:v1, got: %q", detail)
	}
}

func TestImagesDetail_noticeReturnsWithTheCursorToItsOwnImage(t *testing.T) {
	om := pushRefusedOnFirstImage(t, twoImages())
	back := om.imgPanel.MoveBy(1).MoveBy(-1)
	if got := backend.FormatRef(*back.Selected()); got != "alpine:latest" {
		t.Fatalf("precondition: the cursor should be back on the pushed image, got %q", got)
	}
	detail := squash(ansi.Strip(back.DetailView(40, 20)))
	if !strings.Contains(detail, squash("container registry login")) {
		t.Fatalf("the refusal must still be visible on the image it was about, got: %q", detail)
	}
}

func TestImagesDetail_noticeStaysOffAnotherImageInTheSmallestPane(t *testing.T) {
	om := pushRefusedOnFirstImage(t, twoImages())
	other := om.imgPanel.MoveBy(1)
	// 18x4 is what mainPanels hands the detail pane at 60x12 — the smallest
	// frame View() still renders — with the command log toggled on; 18x8 is the
	// same frame without it. The notice leads the pane, so if it were still
	// panel-wide it would be the part that survives the cap.
	for _, size := range []struct{ w, h int }{{18, 4}, {18, 8}} {
		detail := squash(ansi.Strip(other.DetailView(size.w, size.h)))
		if strings.Contains(detail, squash("container registry login")) {
			t.Errorf("detail pane %dx%d shows another image's refusal: %q", size.w, size.h, detail)
		}
		if !strings.Contains(detail, "busybox") {
			t.Errorf("detail pane %dx%d should lead with the selected image, got: %q", size.w, size.h, detail)
		}
	}
}

func TestHelpView_keyColumnNeverRunsIntoItsDescription(t *testing.T) {
	for _, view := range []View{ViewContainers, ViewImages, ViewVolumes} {
		m := New()
		m.width, m.height = 80, 24
		m.activeView = view
		rendered := ansi.Strip(m.helpView())
		for _, b := range m.helpBindings() {
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

// modalRows returns a modal's inner rows: ANSI stripped, border column and the
// modal's two cells of left padding removed, right padding trimmed. That is the
// shape a reader sees.
func modalRows(t *testing.T, modal string) []string {
	t.Helper()
	var out []string
	for _, line := range strings.Split(ansi.Strip(modal), "\n") {
		if !strings.HasPrefix(line, "│") {
			continue
		}
		inner := strings.TrimSuffix(strings.TrimPrefix(line, "│"), "│")
		out = append(out, strings.TrimRight(strings.TrimPrefix(inner, "  "), " "))
	}
	return out
}

// A menu that already fits must render as it did before windowing existed: every
// item on its own row in order, the two blank spacers, and the plain hint with
// no position marker.
func TestActionsModal_shortMenuRendersWithoutWindowing(t *testing.T) {
	for _, view := range []View{ViewContainers, ViewImages, ViewVolumes} {
		m := New()
		m.width, m.height = 100, 30
		m.activeView = view
		m.imgPanel = m.imgPanel.SetItems([]backend.Image{{ID: "sha256:abc", Repository: "alpine", Tag: "latest"}})
		m.cntPanel = m.cntPanel.SetItems([]backend.Container{{ID: "1", Name: "web", Status: "running"}})
		next, _ := m.handleKey(keyMsg("x"))
		mm := next.(Model)
		if len(mm.actionItems) > mm.height-actionsModalChrome {
			t.Fatalf("precondition: the %s menu must already fit, got %d items in a %d-row window",
				m.viewName(), len(mm.actionItems), mm.height-actionsModalChrome)
		}

		want := []string{"", "actions", ""}
		for i, item := range mm.actionItems {
			if i == mm.actionIdx {
				want = append(want, " > "+item.label)
				continue
			}
			want = append(want, "  "+item.label)
		}
		want = append(want, "", "[enter] run  [esc] close", "")

		if got := modalRows(t, mm.actionsModal()); !slices.Equal(got, want) {
			t.Errorf("%s actions modal shape:\n got %q\nwant %q", m.viewName(), got, want)
		}
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
	m := beginPush(t, imagesModelWithItems(t, []backend.Image{
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
	m := imagesModelWithItems(t, []backend.Image{{ID: "sha256:abc", Repository: "alpine", Tag: "latest"}})
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
	m := imagesModelWithItems(t, []backend.Image{
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
	m := beginPush(t, imagesModelWithItems(t, []backend.Image{
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
	m := beginPush(t, imagesModelWithItems(t, []backend.Image{
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
	m := beginPush(t, imagesModelWithItems(t, []backend.Image{
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
	m := imagesModelWithItems(t, nil)
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

// A space bar press serialises as the literal string "space", never " "
// (see AGENTS.md, UI key handling). A prompt that only accepted len==1 byte
// strings silently dropped it, so "my images" typed into a prompt came out
// as "myimages" with no error. This proves the fix round-trips it intact.
func TestHandlePromptKey_spaceSurvivesRoundTrip(t *testing.T) {
	m := New()
	next, _ := m.beginPrompt("pull", "image to pull")
	m = next.(Model)
	for _, k := range []tea.KeyPressMsg{keyMsg("m"), keyMsg("y"), spaceKey(), keyMsg("i")} {
		next, _ = m.handleKey(k)
		m = next.(Model)
	}
	if m.promptBuf != "my i" {
		t.Fatalf("promptBuf = %q, want %q", m.promptBuf, "my i")
	}
	next, _ = m.handleKey(enterKey())
	m = next.(Model)
	if m.status != "pull my i…" {
		t.Fatalf("status = %q, want the space preserved in %q", m.status, "pull my i…")
	}
}

// A multi-byte rune (accents, CJK, emoji) is still exactly one printable
// character, but its Key.String() is more than one byte. A prompt keyed on
// byte length silently dropped it the same way it dropped space.
func TestHandlePromptKey_multibyteRuneSurvivesRoundTrip(t *testing.T) {
	m := New()
	next, _ := m.beginPrompt("pull", "image to pull")
	m = next.(Model)
	for _, r := range "café" {
		next, _ = m.handleKey(keyMsg(string(r)))
		m = next.(Model)
	}
	if m.promptBuf != "café" {
		t.Fatalf("promptBuf = %q, want %q", m.promptBuf, "café")
	}
	// Backspace must remove the whole rune, not just its last byte, or a
	// trailing multi-byte character would corrupt into invalid UTF-8.
	next, _ = m.handleKey(keyMsg("backspace"))
	m = next.(Model)
	if m.promptBuf != "caf" {
		t.Fatalf("promptBuf after backspace = %q, want %q", m.promptBuf, "caf")
	}
}

// A dangling image has no reference, and the reference is the only thing the
// CLI accepts, so settling on one must not run `container image inspect ""`.
func TestScheduleInspect_danglingImageRunsNoSubprocess(t *testing.T) {
	m := imagesModel(t)
	m.imgPanel = m.imgPanel.SetItems([]backend.Image{
		{ID: "1", Repository: "alpine", Tag: "latest"},
		{ID: "sha256:dangling"},
	})

	next, cmd := m.handleMouseWheel(tea.MouseWheelMsg(tea.Mouse{Button: tea.MouseWheelDown}))
	m = next.(Model)
	if backend.FormatRef(*m.imgPanel.Selected()) != "" {
		t.Fatalf("expected the dangling row to be selected, got %+v", m.imgPanel.Selected())
	}
	if cmd != nil {
		t.Errorf("a dangling selection scheduled an inspect: %T", cmd())
	}
	if load := m.loadImageInspectCmd(); load != nil {
		t.Errorf("a dangling selection produced an inspect command: %T", load())
	}
}
