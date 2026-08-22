package ui

import (
	"reflect"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/suite"

	"github.com/Laaaaksh/vessel/internal/config"
)

// helpWalkWidth is wide enough that every help key and description column
// renders untruncated, so the completeness walk reads full cells rather than
// clipped ones.
const helpWalkWidth = 100

// The help screen claims every reachable key is documented there. These tests
// make that claim provable instead of lucky: they walk the same sources
// dispatch reads — the KeyMap fields and the dispatched key literals — classify
// each binding by the views whose help must document it, and assert each one
// shows up as a rendered key cell in that view's help output.
//
// Scope note: the guarantee covers action bindings, i.e. keys dispatch matches.
// Free-text entry into a focused input (filter box, prompt, run form) is not a
// binding universe anyone can enumerate, so printable characters typed into
// those inputs sit outside this claim; their surfaces document themselves.
type helpCompletenessSuite struct {
	suite.Suite
}

func TestHelpCompletenessSuite(t *testing.T) {
	suite.Run(t, new(helpCompletenessSuite))
}

// dispatchedLiteralKeys lists the keys handleKey matches as bare literals in
// browse-mode action dispatch rather than a KeyMap field, spelled with the
// same keys.go constants dispatch reads. Bare literals matched inside other
// modes (confirm y/n, log-view esc/q) stay out of this list on purpose: their
// surfaces document themselves. The inventory itself is hand-synced: no test
// observes handleKey's switch literals, so adding one there without extending
// this list fails nothing today — review is what keeps the walk's input
// complete.
var dispatchedLiteralKeys = []string{
	keyForceQuit,
	keyViewContainers,
	keyViewImages,
	keyViewVolumes,
	keyViewSystem,
	keyViewNetworks,
	keyToggleCmdLog,
}

// helpHiddenBindings names classified bindings deliberately absent from the
// main help screen, with the reason each may stay hidden. The walk enforces
// only what its inventories require — every non-empty KeyMap field plus the
// dispatched literals above — so reachable keys outside that set (confirm
// y, say) are neither listed here nor checked anywhere. An entry here is a
// reviewed decision, not an oversight, and keeping the map itself honest for
// the enforced set is review work, not something a test observes.
var helpHiddenBindings = map[string]string{
	// Editing key inside an open input (filter box, prompt, run form); the
	// surface holding the input shows its own editing chrome.
	"backspace": "text-editing inside a focused input, documented by that surface",
	// Confirm-modal cancel; the modal itself renders "[y] confirm [n/esc] cancel".
	"n": "confirm-modal cancel, shown inline by the confirm dialog",
}

// viewOnlyKeys lists the KeyMap fields handleKey dispatches only inside one
// view's switch branch, mapped to the views whose help must document them.
// Everything else in KeyMap works in every view (or in a sub-state every view
// can reach, like the sidebar or the action menu), so those fields need no
// entry here. A newly added field therefore classifies as global on its own,
// so landing it here consciously is review work; a renamed one severs its
// lookup and surfaces as an unwanted global the per-view walk flags, while a
// removed field strands a stale entry here that fails nothing today.
var viewOnlyKeys = map[string][]View{
	"Logs":    {ViewContainers},
	"Stop":    {ViewContainers},
	"Start":   {ViewContainers},
	"Restart": {ViewContainers},
	"Exec":    {ViewContainers},
	"Pull":    {ViewImages},
	"Remove":  {ViewContainers, ViewImages, ViewVolumes},
	"Prune":   {ViewContainers, ViewImages, ViewVolumes},
	"Create":  {ViewContainers, ViewImages, ViewVolumes},
}

// allViews walks the View enum up to viewCount — the same constant Tab cycling
// mods against — so a view added to the enum joins this walk on its own
// instead of slipping past a hand-copied list unexamined.
func allViews() []View {
	views := make([]View, 0, viewCount)
	for i := 0; i < viewCount; i++ {
		views = append(views, View(i))
	}
	return views
}

// requiredKeysFor returns the keys whose bindings must appear in the given
// view's help: the globals every view shares plus the view-specific actions,
// minus the deliberately hidden ones.
func (s *helpCompletenessSuite) requiredKeysFor(view View, keys KeyMap) map[string]bool {
	required := map[string]bool{}
	for _, k := range dispatchedLiteralKeys {
		required[k] = true
	}
	required[navAliasDown] = true
	required[navAliasUp] = true

	rt := reflect.TypeOf(keys)
	rv := reflect.ValueOf(keys)
	for i := 0; i < rt.NumField(); i++ {
		val := rv.Field(i).String()
		if val == "" {
			continue
		}
		views, viewSpecific := viewOnlyKeys[rt.Field(i).Name]
		switch {
		case viewSpecific && containsView(views, view):
			required[val] = true
		case !viewSpecific:
			required[val] = true
		}
	}
	for k := range helpHiddenBindings {
		delete(required, k)
	}
	return required
}

func containsView(views []View, want View) bool {
	for _, v := range views {
		if v == want {
			return true
		}
	}
	return false
}

// renderedRow is one binding line read back out of rendered help output.
type renderedRow struct{ key, desc string }

// renderFullHelp renders the help screen tall enough to show every binding
// without scrolling, then reads the rows back out of the rendered text — the
// assertion target is what the screen actually shows, not the row structs
// helpBindings builds. Rows are located positionally below the fixed header;
// horizontal centering pads each line variably on the left, which TrimLeft
// normalizes before the fixed-width key cell is sliced off.
func (s *helpCompletenessSuite) renderFullHelp(m Model) ([]renderedRow, string) {
	bindings := m.helpBindings()
	m.height = len(bindings) + helpChromeRows
	m.width = helpWalkWidth

	rendered := ansi.Strip(viewString(m.View()))
	lines := strings.Split(rendered, "\n")
	s.Len(lines, m.height, "chrome-sized render must not gain or lose rows")

	keyW := 0
	for _, b := range bindings {
		keyW = max(keyW, lipgloss.Width(b.key))
	}
	keyW = min(keyW, max(1, m.width/2))

	rows := make([]renderedRow, 0, len(bindings))
	for _, ln := range lines[helpHeaderRows : helpHeaderRows+len(bindings)] {
		body := strings.TrimLeft(ln, " ")
		cell := body[:min(keyW, len(body))]
		rows = append(rows, renderedRow{
			key:  cell,
			desc: strings.TrimSpace(body[min(keyW, len(body)):]),
		})
	}
	return rows, rendered
}

func rowTokens(rows []renderedRow) []string {
	var tokens []string
	for _, r := range rows {
		tokens = append(tokens, helpKeyTokens(r.key)...)
	}
	return tokens
}

// TestClassifiesEveryKeyMapField keeps the classification inventory coherent:
// walking every view must account for each non-empty KeyMap binding, and an
// exclusion must never overlap a classified key, or it would silently
// un-require a real one. Classification defaults new fields to global, so this
// test verifies the inventory's shape rather than forcing a decision per new
// field.
func (s *helpCompletenessSuite) TestClassifiesEveryKeyMapField() {
	keys := DefaultKeyMap()
	classified := map[string]bool{}
	for _, view := range allViews() {
		for k := range s.requiredKeysFor(view, keys) {
			classified[k] = true
		}
	}

	rt := reflect.TypeOf(keys)
	rv := reflect.ValueOf(keys)
	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)
		val := rv.Field(i).String()
		if val == "" || classified[val] {
			continue
		}
		s.Failf("unclassified binding",
			"KeyMap.%s (%q) is neither global, in viewOnlyKeys, nor excluded", field.Name, val)
	}

	for k, why := range helpHiddenBindings {
		s.NotContainsf(classified, k,
			"hidden key %q must not also be a required binding; resolve the conflict: %s", k, why)
	}
}

// TestEveryReachableBindingIsRenderedInItsViewsHelp is the completeness walk:
// for each view, every binding reachable there must surface as a rendered key
// cell in that view's own help, so the on-screen claim holds per view rather
// than merely across all views pooled together.
func (s *helpCompletenessSuite) TestEveryReachableBindingIsRenderedInItsViewsHelp() {
	keys := DefaultKeyMap()
	for _, view := range allViews() {
		m := newTestModel()
		m.activeView = view
		m.showHelp = true

		rows, rendered := s.renderFullHelp(m)

		for k := range s.requiredKeysFor(view, keys) {
			s.Containsf(rowTokens(rows), k,
				"%s help must render key %q; rows:\n%s", m.viewName(), k, rendered)
		}
		s.Contains(rendered, "press ? or esc to close",
			"walk size must fit every binding without scrolling, or the walk lies")
	}
}

// TestHiddenKeysStayOutOfRenderedHelp keeps the exclusion list honest in the
// other direction: a key marked deliberately hidden must not quietly reappear
// as a documented binding without the entry being removed first.
func (s *helpCompletenessSuite) TestHiddenKeysStayOutOfRenderedHelp() {
	for _, view := range allViews() {
		m := newTestModel()
		m.activeView = view
		m.showHelp = true

		rows, _ := s.renderFullHelp(m)
		for k, why := range helpHiddenBindings {
			s.NotContainsf(rowTokens(rows), k,
				"%s documents hidden key %q: %s", m.viewName(), k, why)
		}
	}
}

// TestDispatchedLiteralsAreAllReserved couples the two literal inventories:
// every key dispatch handles as a bare literal must sit behind the reserved
// guard, or a custom command could claim it in config while dispatch keeps
// winning — help would advertise a binding that never fires. This exact gap
// once left the view digits 4 and 5 unreserved.
func (s *helpCompletenessSuite) TestDispatchedLiteralsAreAllReserved() {
	keys := DefaultKeyMap()
	for _, k := range append(dispatchedLiteralKeys, navAliasDown, navAliasUp) {
		s.Truef(keys.Reserved(k), "dispatched literal %q must be reserved from custom commands", k)
	}
}

// TestCustomShadowKeepsRemainingBindingsDocumented runs the same completeness
// walk with a custom command shadowing a built-in: the shadowed key flips to
// its custom row and every other reachable binding stays documented, so help
// completeness survives configuration, not just defaults.
func (s *helpCompletenessSuite) TestCustomShadowKeepsRemainingBindingsDocumented() {
	keys := DefaultKeyMap()
	custom := []config.CustomCommand{{Name: "up", Key: keys.Start, Command: "echo up"}}
	m := newTestModel()
	m.activeView = ViewContainers
	m.cfg.CustomCommands = custom
	m.showHelp = true

	rows, rendered := s.renderFullHelp(m)
	tokens := rowTokens(rows)

	for k := range s.requiredKeysFor(ViewContainers, keys) {
		if k == keys.Start {
			continue
		}
		s.Containsf(tokens, k, "shadowed keymap must keep %q documented; rows:\n%s", k, rendered)
	}
	s.Contains(tokens, keys.Start, "the shadowing key takes over the start row")
	var shadowFound bool
	for _, r := range rows {
		if helpKeyTokens(r.key)[0] == keys.Start && strings.HasPrefix(r.desc, "custom:") {
			shadowFound = true
		}
	}
	s.True(shadowFound, "start row must say custom after shadowing:\n%s", rendered)
}

// TestHelpAt60x12KeepsIdentityRowsWhileScrolling guards the smallest supported
// terminal: the help chrome (title, view/focus line, close hint) must survive
// the entire scroll range, the range must sweep every window of the binding
// list exactly, and shrinking the window hides rows behind scrolling without
// ever dropping them.
func (s *helpCompletenessSuite) TestHelpAt60x12KeepsIdentityRowsWhileScrolling() {
	m := newTestModel()
	m.width, m.height = 60, 12
	m.activeView = ViewContainers
	m.showHelp = true

	bindings := m.helpBindings()
	windows := len(bindings) - helpVisibleRows(m.height) + 1
	s.Greater(windows, 1, "precondition: 60x12 must actually scroll, or nothing here is exercised")

	first := ansi.Strip(viewString(m.View()))
	s.Contains(first, "vessel — keybindings")
	s.Contains(first, "view=")
	s.Contains(first, descPrefix(bindings[0].desc))

	keys := DefaultKeyMap()
	next, _ := m.handleKey(keyMsg(keys.GotoBottom))
	m = next.(Model)
	last := ansi.Strip(viewString(m.View()))
	s.Contains(last, "vessel — keybindings", "title must survive scrolling to the bottom")
	s.Contains(last, descPrefix(bindings[len(bindings)-1].desc))
	s.LessOrEqual(lipgloss.Height(viewString(m.View())), m.height)

	next, _ = m.handleKey(keyMsg(keys.GotoTop))
	m = next.(Model)
	frames := map[string]bool{}
	for step := 0; step <= len(bindings); step++ {
		frames[ansi.Strip(viewString(m.View()))] = true
		next, _ = m.handleKey(keyMsg(navAliasDown))
		m = next.(Model)
	}
	s.Len(frames, windows, "scrolling must sweep every window of the binding list exactly")

	seen := flattenFrames(frames)
	for _, b := range bindings {
		s.Containsf(seen, descPrefix(b.desc),
			"binding %q must be visible at some scroll position on a 60x12 screen", b.key)
	}
	s.Contains(ansi.Strip(viewString(m.View())), "press ? or esc to close",
		"close hint must survive the whole scroll range")
}

// descPrefixLen bounds the description fragment scroll assertions match on.
// It must stay below the description width guaranteed visible at 60 columns —
// helpView caps the key column at width/2, leaving at least 29 description
// columns there — or truncated prose would break substring matching.
const descPrefixLen = 10

// descPrefix shortens a description to a fragment narrow enough to survive the
// 60-column description truncation, so scroll-position assertions match prose
// that may be clipped on screen.
func descPrefix(desc string) string {
	desc = strings.TrimSpace(desc)
	return desc[:min(descPrefixLen, len(desc))]
}

// flattenFrames joins every collected frame so substring assertions can look
// across scroll positions in one call.
func flattenFrames(frames map[string]bool) string {
	var sb strings.Builder
	for frame := range frames {
		sb.WriteString(frame)
		sb.WriteString("\n")
	}
	return sb.String()
}
