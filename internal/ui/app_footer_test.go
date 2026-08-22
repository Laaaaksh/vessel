package ui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/Laaaaksh/vessel/internal/config"
)

// restingContainersFooter is what the containers view has always advertised
// with default bindings; every shadowing scenario below starts from it.
const restingContainersFooter = "0/0  [enter] shell  [L] logs  [s/u/r] lifecycle  [c] run  [e] exec  [d] remove  [/] filter  [x] actions  [y] yank"

// assertRestingFooter pins one view's full footer line byte-for-byte: the
// no-custom rendering must stay identical to what the old hardcoded strings
// produced, so any drift here is a regression against the documented output.
func assertRestingFooter(t *testing.T, view View, want string) {
	t.Helper()
	m := newTestModel()
	m.width, m.height = 120, 24
	m.activeView = view
	if _, got := m.footerLine(); got != want {
		t.Fatalf("resting footer drifted from its documented string:\n got %q\nwant %q", got, want)
	}
}

func TestFooterLine_containersDefault_matchesDocumentedString(t *testing.T) {
	assertRestingFooter(t, ViewContainers, restingContainersFooter)
}

func TestFooterLine_imagesDefault_matchesDocumentedString(t *testing.T) {
	assertRestingFooter(t, ViewImages,
		"0/0  [p] pull  [c] run  [d] delete  [P] prune  [/] filter  [x] actions  [y] yank")
}

func TestFooterLine_volumesDefault_matchesDocumentedString(t *testing.T) {
	assertRestingFooter(t, ViewVolumes,
		"0/0  [c] create  [d] delete  [P] prune  [/] filter  [x] actions  [y] yank")
}

func TestFooterLine_networksDefault_matchesDocumentedString(t *testing.T) {
	assertRestingFooter(t, ViewNetworks, "0/0  [/] filter  [y] yank")
}

func TestFooterLine_systemDefault_matchesDocumentedString(t *testing.T) {
	// system.New() seeds four static rows, so the cursor reads 1/4.
	assertRestingFooter(t, ViewSystem, "1/4  [j/k] navigate  [y] yank  (read-only)")
}

func TestFooterLine_shadowedYank_showsCustomInsteadOfBuiltIn(t *testing.T) {
	m := newTestModel()
	m.width, m.height = 120, 24
	m.cfg.CustomCommands = []config.CustomCommand{
		{Name: "redeploy", Key: "y", Command: "echo redeploy"},
	}
	m.activeView = ViewContainers
	want := "0/0  [enter] shell  [L] logs  [s/u/r] lifecycle  [c] run  [e] exec  [d] remove  [/] filter  [x] actions  [y] custom: redeploy"
	if _, got := m.footerLine(); got != want {
		t.Fatalf("shadowed yank must hand its slot to the custom command:\n got %q\nwant %q", got, want)
	}
}

func TestFooterLine_shadowedLifecycleKey_splitsGroupAroundClaimedKey(t *testing.T) {
	m := newTestModel()
	m.width, m.height = 120, 24
	m.cfg.CustomCommands = []config.CustomCommand{
		{Name: "graceful stop", Key: "s", Command: "echo bye"},
	}
	m.activeView = ViewContainers
	want := "0/0  [enter] shell  [L] logs  [s] custom: graceful stop  [u/r] lifecycle  [c] run  [e] exec  [d] remove  [/] filter  [x] actions  [y] yank"
	if _, got := m.footerLine(); got != want {
		t.Fatalf("surviving lifecycle keys must stay grouped under their label:\n got %q\nwant %q", got, want)
	}
}

func TestFooterLine_wholeLifecycleGroupClaimed_customsTakeEverySlot(t *testing.T) {
	m := newTestModel()
	m.width, m.height = 120, 24
	m.cfg.CustomCommands = []config.CustomCommand{
		{Name: "stop-hard", Key: "s", Command: "echo s"},
		{Name: "start-fresh", Key: "u", Command: "echo u"},
		{Name: "recycle", Key: "r", Command: "echo r"},
	}
	m.activeView = ViewContainers
	want := "0/0  [enter] shell  [L] logs  [s] custom: stop-hard  [u] custom: start-fresh  [r] custom: recycle  [c] run  [e] exec  [d] remove  [/] filter  [x] actions  [y] yank"
	if _, got := m.footerLine(); got != want {
		t.Fatalf("a fully claimed group must leave no built-in fragment behind:\n got %q\nwant %q", got, want)
	}
}

func TestFooterLine_imagesPullClaimed_customTakesSlotInPlace(t *testing.T) {
	m := newTestModel()
	m.width, m.height = 120, 24
	m.cfg.CustomCommands = []config.CustomCommand{
		{Name: "fetch-base", Key: "p", Command: "echo pull"},
	}
	m.activeView = ViewImages
	want := "0/0  [p] custom: fetch-base  [c] run  [d] delete  [P] prune  [/] filter  [x] actions  [y] yank"
	if _, got := m.footerLine(); got != want {
		t.Fatalf("images footer must give the pull slot to the claiming command:\n got %q\nwant %q", got, want)
	}
}

func TestFooterLine_unnamedShadowingCustom_usesGenericLabel(t *testing.T) {
	m := newTestModel()
	m.width, m.height = 120, 24
	m.cfg.CustomCommands = []config.CustomCommand{
		{Key: "d", Command: "echo unnamed"},
	}
	m.activeView = ViewContainers
	want := "0/0  [enter] shell  [L] logs  [s/u/r] lifecycle  [c] run  [e] exec  [d] custom command  [/] filter  [x] actions  [y] yank"
	if _, got := m.footerLine(); got != want {
		t.Fatalf("an unnamed command must fall back to the shared generic label:\n got %q\nwant %q", got, want)
	}
}

func TestFooterLine_unreachableCustomKeys_leaveFooterUntouched(t *testing.T) {
	m := newTestModel()
	m.width, m.height = 120, 24
	m.cfg.CustomCommands = []config.CustomCommand{
		{Name: "nav", Key: "j", Command: "echo reserved"},
		{Name: "shiftless", Key: "shift+z", Command: "echo unproducible"},
		{Name: "menu-only", Key: "", Command: "echo menu"},
		{Name: "mute", Key: "w", Command: ""},
		{Name: "first-z", Key: "z", Command: "echo first"},
		{Name: "second-z", Key: "z", Command: "echo second"},
	}
	m.activeView = ViewContainers
	if _, got := m.footerLine(); got != restingContainersFooter {
		t.Fatalf("keys dispatch never grants cannot change the footer:\n got %q\nwant %q", got, restingContainersFooter)
	}
}

func TestFooterAndHelp_agreeOnShadowedKeys(t *testing.T) {
	m := newTestModel()
	m.width, m.height = 120, 24
	m.cfg.CustomCommands = []config.CustomCommand{
		{Name: "redefine", Key: "y", Command: "echo redefined"},
	}
	_, footer := m.footerLine()
	if strings.Contains(footer, "[y] yank") {
		t.Fatalf("footer still advertises yank under a key a custom command owns: %q", footer)
	}
	if !strings.Contains(footer, "[y] custom: redefine") {
		t.Fatalf("footer must advertise the owning command where yank sat: %q", footer)
	}
	var help strings.Builder
	for _, b := range m.helpBindings() {
		help.WriteString(b.key + "\t" + b.desc + "\n")
	}
	if strings.Contains(help.String(), "yank id/name to clipboard") {
		t.Fatalf("help still advertises yank the footer has dropped:\n%s", help.String())
	}
	if !strings.Contains(help.String(), "custom: redefine") {
		t.Fatalf("help must carry the same custom command the footer shows:\n%s", help.String())
	}
}

func TestFooterView_withCustomCommands_staysOneReadableRowAt60x12(t *testing.T) {
	m := newTestModel()
	m.width, m.height = 60, 12
	m.cfg.CustomCommands = []config.CustomCommand{
		{Name: "redeploy", Key: "y", Command: "echo 1"},
		{Name: strings.Repeat("long-name-", 8), Key: "d", Command: "echo 2"},
		{Name: "graceful stop", Key: "s", Command: "echo 3"},
		{Name: "fresh-key", Key: "z", Command: "echo 4"},
	}
	m.activeView = ViewContainers
	footer := m.footerView()
	if lines := lipgloss.Height(footer); lines != 1 {
		t.Fatalf("footer must render on exactly one row at 60x12, got %d: %q", lines, footer)
	}
	stripped := ansi.Strip(footer)
	if got := lipgloss.Width(stripped); got > m.width {
		t.Fatalf("footer must not overflow %d columns, got %d: %q", m.width, got, stripped)
	}
	if !strings.HasPrefix(stripped, "0/0") {
		t.Fatalf("truncation must keep the cursor-position identity prefix, got %q", stripped)
	}
}
