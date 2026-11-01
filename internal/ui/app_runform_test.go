package ui

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/Laaaaksh/vessel/internal/backend"
)

func containersModel(t *testing.T) Model {
	t.Helper()
	m := New()
	m.width, m.height = 120, 40
	m.activeView = ViewContainers
	m.client = backend.NewClientWithBinary(filepath.Join(t.TempDir(), "no-such-container-cli"))
	m.cntPanel = m.cntPanel.SetItems([]backend.Container{
		{ID: "1", Name: "web", Status: "running"},
		{ID: "2", Name: "db", Status: "exited"},
	})
	return m
}

func TestBeginRunForm_imagesView_prefillsSelectedImage(t *testing.T) {
	m := imagesModel(t)
	next, _ := m.handleKey(keyMsg("c"))
	m = next.(Model)
	if m.mode != modeRunForm {
		t.Fatalf("mode = %v, want modeRunForm", m.mode)
	}
	if got := m.runForm.text[runFieldImage]; got != "alpine:latest" {
		t.Fatalf("prefilled image = %q, want alpine:latest", got)
	}
}

func TestBeginRunForm_containersView_startsBlank(t *testing.T) {
	m := containersModel(t)
	next, _ := m.handleKey(keyMsg("c"))
	m = next.(Model)
	if m.mode != modeRunForm {
		t.Fatalf("mode = %v, want modeRunForm", m.mode)
	}
	if got := m.runForm.text[runFieldImage]; got != "" {
		t.Fatalf("image = %q, want blank (containers view has no image selection)", got)
	}
}

// esc from the form must cancel back to browse without touching the client.
func TestRunForm_escCancels(t *testing.T) {
	m := containersModel(t)
	next, _ := m.handleKey(keyMsg("c"))
	m = next.(Model)
	next, _ = m.handleKey(keyMsg("esc"))
	m = next.(Model)
	if m.mode != modeBrowse {
		t.Fatalf("mode = %v, want modeBrowse after esc", m.mode)
	}
	if m.status != "cancelled" {
		t.Fatalf("status = %q, want cancelled", m.status)
	}
}

// An invalid field (a port entry with no ':') must report a specific message
// and leave the form open, never reaching the CLI with bad input.
func TestRunForm_submitInvalidPort_staysOpenWithError(t *testing.T) {
	m := containersModel(t)
	next, _ := m.handleKey(keyMsg("c"))
	m = next.(Model)
	m.runForm.text[runFieldImage] = "alpine"
	m.runForm.text[runFieldPorts] = "bogus"

	next, cmd := m.handleKey(enterKey())
	m = next.(Model)
	if m.mode != modeRunForm {
		t.Fatalf("mode = %v, want modeRunForm (invalid submit must not close the form)", m.mode)
	}
	if m.runForm.err != `invalid port mapping "bogus" (want host:container)` {
		t.Fatalf("err = %q", m.runForm.err)
	}
	if cmd != nil {
		t.Fatal("an invalid submit must not issue a command")
	}
	if !strings.Contains(ansi.Strip(m.View().Content), "invalid port mapping") {
		t.Fatal("validation error must render in the form")
	}
}

// A valid submit closes the form and runs the container with exactly the
// flags entered - proved here by checking backend.Client.Run was actually
// invoked with the image, via the (expected) error from a nonexistent CLI
// binary naming the same image.
func TestRunForm_submitValid_closesFormAndInvokesRun(t *testing.T) {
	m := containersModel(t)
	next, _ := m.handleKey(keyMsg("c"))
	m = next.(Model)
	m.runForm.text[runFieldImage] = "alpine:latest"
	m.runForm.text[runFieldPorts] = "8080:80"
	m.runForm.bools[runFieldDetached] = true

	next, cmd := m.handleKey(enterKey())
	m = next.(Model)
	if m.mode != modeBrowse {
		t.Fatalf("mode = %v, want modeBrowse after a valid submit", m.mode)
	}
	if cmd == nil {
		t.Fatal("a valid submit must issue a command")
	}
	msg, ok := cmd().(actionDoneMsg)
	if !ok {
		t.Fatalf("cmd returned %T, want actionDoneMsg", cmd())
	}
	if msg.err == nil {
		t.Fatal("expected an error from the nonexistent fake CLI binary")
	}
	if !strings.Contains(msg.err.Error(), "alpine:latest") || !strings.Contains(msg.err.Error(), "8080:80") {
		t.Fatalf("error %q does not show Run was invoked with the form's flags", msg.err.Error())
	}
}

// Navigation (tab / j / k) must move focus within bounds, and space must
// toggle a bool field it lands on rather than typing into it.
func TestRunForm_navigationAndToggle(t *testing.T) {
	m := containersModel(t)
	next, _ := m.handleKey(keyMsg("c"))
	m = next.(Model)
	if m.runForm.focus != runFieldImage {
		t.Fatalf("initial focus = %d, want runFieldImage", m.runForm.focus)
	}
	for i := 0; i < runFieldDetached; i++ {
		next, _ = m.handleKey(keyMsg("tab"))
		m = next.(Model)
	}
	if m.runForm.focus != runFieldDetached {
		t.Fatalf("focus after %d tabs = %d, want runFieldDetached (%d)", runFieldDetached, m.runForm.focus, runFieldDetached)
	}
	next, _ = m.handleKey(spaceKey())
	m = next.(Model)
	if !m.runForm.bools[runFieldDetached] {
		t.Fatal("space on the Detached field must toggle it on")
	}
	next, _ = m.handleKey(keyMsg("k"))
	m = next.(Model)
	if m.runForm.focus != runFieldDetached-1 {
		t.Fatalf("focus after k = %d, want %d", m.runForm.focus, runFieldDetached-1)
	}
}

func TestBeginExec_requiresRunningContainer(t *testing.T) {
	m := containersModel(t)
	next, _ := m.handleKey(keyMsg("j")) // move to the stopped "db" row
	m = next.(Model)
	next, _ = m.handleKey(keyMsg("e"))
	m = next.(Model)
	if m.mode != modeBrowse {
		t.Fatalf("mode = %v, want modeBrowse (exec must refuse a stopped container)", m.mode)
	}
	if m.status != "exec disabled: container is not running" {
		t.Fatalf("status = %q", m.status)
	}
}

func TestBeginExec_opensPromptForRunningContainer(t *testing.T) {
	m := containersModel(t)
	next, _ := m.handleKey(keyMsg("e"))
	m = next.(Model)
	if m.mode != modePrompt {
		t.Fatalf("mode = %v, want modePrompt", m.mode)
	}
	if m.promptKind != "exec" {
		t.Fatalf("promptKind = %q, want exec", m.promptKind)
	}
}

// The exec prompt is the same fixed handlePromptKey plumbing the space bug
// was fixed in, so a multi-word command must survive intact into the command
// actually sent to the CLI.
func TestSubmitExec_commandWithSpaceReachesClient(t *testing.T) {
	m := containersModel(t)
	next, _ := m.handleKey(keyMsg("e"))
	m = next.(Model)
	for _, k := range []string{"l", "s", keySpace, "-", "l", "a"} {
		next, _ = m.handleKey(keyMsg(k))
		m = next.(Model)
	}
	if m.promptBuf != "ls -la" {
		t.Fatalf("promptBuf = %q, want %q", m.promptBuf, "ls -la")
	}
	next, cmd := m.handleKey(enterKey())
	m = next.(Model)
	if cmd == nil {
		t.Fatal("submitting the exec prompt must issue a command")
	}
	msg, ok := cmd().(actionDoneMsg)
	if !ok {
		t.Fatalf("cmd returned %T, want actionDoneMsg", cmd())
	}
	if msg.err == nil || !strings.Contains(msg.err.Error(), "ls -la") {
		t.Fatalf("error %v does not show the full command reached Client.Exec", msg.err)
	}
}

// The form must render legibly at the smallest supported terminal size
// (60x12) rather than overflowing: fewer fields are shown at once, with a
// counter, instead of the modal spilling past the screen.
func TestRunFormModal_rendersAtSmallestSupportedSize(t *testing.T) {
	m := containersModel(t)
	m.width, m.height = 60, 12
	next, _ := m.handleKey(keyMsg("c"))
	m = next.(Model)
	view := ansi.Strip(m.View().Content)
	if !strings.Contains(view, "run container") {
		t.Fatal("form title missing from the rendered view")
	}
	if !strings.Contains(view, "1/") {
		t.Fatalf("expected a field counter when the form does not fit, got:\n%s", view)
	}
	lines := strings.Split(view, "\n")
	if len(lines) > 12 {
		t.Fatalf("rendered %d lines, want at most the 12-row terminal height", len(lines))
	}
}
