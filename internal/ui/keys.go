package ui

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/Laaaaksh/vessel/internal/config"
)

// KeyMap documents all keybindings for vessel.
// Matching is done via tea.KeyPressMsg.String() in Update().
type KeyMap struct {
	Up         string
	Down       string
	Left       string
	Right      string
	PageUp     string
	PageDown   string
	HalfUp     string
	HalfDown   string
	GotoTop    string
	GotoBottom string
	Enter      string
	Escape     string
	Tab        string
	Logs       string
	Stop       string
	Start      string
	Restart    string
	Remove     string
	Filter     string
	Help       string
	Quit       string
	Yank       string
	Actions    string
	Pull       string
	Prune      string
	Create     string
	Exec       string
	Follow     string
	LayoutNext string
	LayoutPrev string
	ToggleMark string
	FocusNext  string
	FocusPrev  string
}

// DefaultKeyMap returns the default keybindings.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up:         "up",
		Down:       "down",
		Left:       "left",
		Right:      "right",
		PageUp:     "pgup",
		PageDown:   "pgdown",
		HalfUp:     "ctrl+u",
		HalfDown:   "ctrl+d",
		GotoTop:    "g",
		GotoBottom: "G",
		Enter:      "enter",
		Escape:     "esc",
		Tab:        "tab",
		Logs:       "L",
		Stop:       "s",
		Start:      "u",
		Restart:    "r",
		Remove:     "d",
		Filter:     "/",
		Help:       "?",
		Quit:       "q",
		Yank:       "y",
		Actions:    "x",
		Pull:       "p",
		Prune:      "P",
		Create:     "c",
		Exec:       "e",
		Follow:     "f",
		LayoutNext: "+",
		LayoutPrev: "_",
		ToggleMark: "space",
		FocusNext:  "l",
		FocusPrev:  "h",
	}
}

// Dispatched key literals handleKey matches outside the KeyMap fields. They
// live beside KeyMap as constants so dispatch, the reserved-key guard below,
// and the help-completeness test all read one definition instead of three
// copies that can drift apart.
const (
	keyForceQuit      = "ctrl+c"
	keyViewContainers = "1"
	keyViewImages     = "2"
	keyViewVolumes    = "3"
	keyViewSystem     = "4"
	keyViewNetworks   = "5"
	keyToggleCmdLog   = "`"
	navAliasDown      = "j"
	navAliasUp        = "k"
)

// Match reports whether k equals any of the candidates.
func Match(k string, candidates ...string) bool {
	for _, c := range candidates {
		if c != "" && k == c {
			return true
		}
	}
	return false
}

// navAliasDown/navAliasUp are the vim-style aliases list navigation accepts
// beside the arrow keys. They are reserved, so no hint that cites them can
// ever be shadowed by a custom command.
const (
	navAliasDown = "j"
	navAliasUp   = "k"
)

// NavDown matches down / j.
func (k KeyMap) NavDown(s string) bool { return Match(s, k.Down, navAliasDown) }

// NavUp matches up / k.
func (k KeyMap) NavUp(s string) bool { return Match(s, k.Up, navAliasUp) }

// Reserved reports whether s is a navigation, filtering, layout or global
// binding. Update() dispatches those before user-configured custom command
// keys, so a custom command can never shadow them: binding "j" to a command
// must not make the list unscrollable. The action menu is reserved too: it is
// where every custom command that could not take its key still runs from, so
// shadowing it would strand the rest of them.
func (k KeyMap) Reserved(s string) bool {
	if k.NavUp(s) || k.NavDown(s) {
		return true
	}
	if Match(s, k.Left, k.Right, k.FocusNext, k.FocusPrev,
		k.PageUp, k.PageDown, k.HalfUp, k.HalfDown,
		k.GotoTop, k.GotoBottom, k.Filter, k.Escape, k.Tab,
		k.Quit, k.Help, k.LayoutNext, k.LayoutPrev, k.Actions) {
		return true
	}
	// Every view digit is reserved, not just the first three: handleKey's
	// numeric switch runs before custom-command dispatch, so a custom command
	// bound to any digit can never fire — reserving all five is what stops
	// help from advertising a binding that does nothing.
	return Match(s, keyViewContainers, keyViewImages, keyViewVolumes,
		keyViewSystem, keyViewNetworks, keyToggleCmdLog, keyForceQuit)
}

// helpRow is one line of the in-app help: the key column and what it does.
type helpRow struct {
	key, desc string
}

// helpBindings returns contextual (key, description) pairs, including the
// custom commands reachable by their configured key.
func helpBindings(view View, focus Focus, mode Mode, keys KeyMap, custom []config.CustomCommand) []helpRow {
	base := []helpRow{
		{"h / l / left / right", "move focus (sidebar / list / detail)"},
		{"j / k / up / down", "move up / down (in list)"},
		{"g / G", "top / bottom"},
		{"pgup / pgdown / ctrl+u / ctrl+d", "page / half-page scroll"},
		{"tab / 1 2 3 4 5", "switch Containers / Images / Volumes / System / Networks"},
		{"+ / _", "cycle layout"},
		{"space", "toggle multi-select mark"},
		{"y", "yank id/name to clipboard"},
		{"x", "action menu"},
		{"/", "filter"},
		{"f", "follow / freeze logs (log view)"},
		{"`", "toggle command log"},
		{"?", "toggle help"},
		{"esc", "cancel / close"},
		{"enter", "confirm dialogs / enter list from sidebar"},
		{"q / ctrl+c", "quit"},
	}
	switch view {
	case ViewImages:
		base = append([]helpRow{
			{"p", "pull image (prompt)"},
			{"P", "prune unused images (confirm)"},
			{"d", "delete marked, else cursor row (confirm)"},
			{"c", "run container from image (form)"},
			{"x → image mobility", "tag, save, load, push (push confirms)"},
		}, base...)
	case ViewVolumes:
		base = append([]helpRow{
			{"c", "create volume (prompt)"},
			{"P", "prune unused volumes (confirm)"},
			{"d", "delete marked, else cursor row (confirm)"},
		}, base...)
	case ViewNetworks, ViewSystem:
		// Read-only views: list and inspect only. They get no view-specific
		// rows — dispatch handles no per-view verbs here, and inheriting the
		// containers block's would advertise actions that do nothing.
	default:
		base = append([]helpRow{
			{"enter", "open shell in running container"},
			{"L", "view logs"},
			{"s", "stop container"},
			{"u", "start container"},
			{"r", "restart container"},
			{"d", "delete marked, else cursor row (confirm)"},
			{"P", "prune stopped containers (confirm)"},
			{"c", "run new container (form)"},
			{"e", "one-shot exec in running container"},
		}, base...)
	}
	_ = focus
	_ = mode
	return withCustomBindings(base, keys, custom)
}

// runtimeKeyNames are the multi-rune key names tea.KeyPressMsg.String() emits.
// Anything else it emits is a single rune (the key's own text), so a configured
// key outside this set that is longer than one rune can never match a keypress.
var runtimeKeyNames = map[string]bool{
	"enter": true, "tab": true, "backspace": true, "esc": true, "space": true,
	"up": true, "down": true, "left": true, "right": true, "begin": true,
	"find": true, "insert": true, "delete": true, "select": true,
	"pgup": true, "pgdown": true, "home": true, "end": true,
	"f1": true, "f2": true, "f3": true, "f4": true, "f5": true, "f6": true,
	"f7": true, "f8": true, "f9": true, "f10": true, "f11": true, "f12": true,
	"f13": true, "f14": true, "f15": true, "f16": true, "f17": true,
	"f18": true, "f19": true, "f20": true,
}

// modifierOrder is the order tea.KeyPressMsg.String() always prints modifiers
// in, whatever order they were pressed or configured in.
var modifierOrder = []string{"ctrl", "alt", "shift", "meta", "hyper", "super"}

// keyModifiers are the modifier prefixes tea.KeyPressMsg.String() emits, in the
// "ctrl+shift+a" form.
var keyModifiers = map[string]bool{
	"ctrl": true, "alt": true, "shift": true,
	"meta": true, "hyper": true, "super": true,
}

// keyAliases maps spellings a user plausibly writes in config.toml onto the one
// the runtime produces.
var keyAliases = map[string]string{
	"spacebar": "space", "escape": "esc", "return": "enter", "del": "delete",
	"pgdn": "pgdown", "pagedown": "pgdown", "pagedn": "pgdown",
	"pageup": "pgup", "bs": "backspace",
}

// splitKey separates a key into the modifiers it names and the key they modify.
// seps lists the characters that may join a modifier to the rest, so both the
// spellings a config may use ("ctrl-x") and the canonical one ("ctrl+x") parse
// through here.
func splitKey(s, seps string) (map[string]bool, string) {
	mods, base := map[string]bool{}, s
	for {
		i := strings.IndexAny(base, seps)
		if i <= 0 || i == len(base)-1 {
			break
		}
		mod := strings.ToLower(base[:i])
		if !keyModifiers[mod] {
			break
		}
		mods[mod] = true
		base = base[i+1:]
	}
	return mods, base
}

// normalizeKey rewrites a configured key onto the spelling
// tea.KeyPressMsg.String() produces, so config.toml and the runtime agree: a
// literal space and "space" are the same key, "ctrl-x" is "ctrl+x", modifiers
// come out in the runtime's order, and named keys are case-insensitive. A bare
// character stays case-sensitive (G and g are different keys), but a modified
// one is lowercased, because a modifier suppresses the typed text and the
// runtime then reports the unshifted rune ("ctrl+z", never "ctrl+Z").
func normalizeKey(s string) string {
	if s == "" {
		return ""
	}
	if strings.TrimSpace(s) == "" {
		return "space"
	}
	mods, base := splitKey(strings.TrimSpace(s), "+-")
	switch {
	case keyAliases[strings.ToLower(base)] != "":
		base = keyAliases[strings.ToLower(base)]
	case utf8.RuneCountInString(base) > 1, len(mods) > 0:
		base = strings.ToLower(base)
	}
	prefix := ""
	for _, mod := range modifierOrder {
		if mods[mod] {
			prefix += mod + "+"
		}
	}
	return prefix + base
}

// producibleKey reports whether a normalized key can ever be produced by a
// keypress, so help never advertises a binding that cannot fire. A printable
// key pressed with shift alone arrives as the character it types ("Z", never
// "shift+z"); any further modifier suppresses that text, so "ctrl+shift+a" is
// real.
func producibleKey(k string) bool {
	mods, base := splitKey(k, "+")
	if runtimeKeyNames[base] {
		return true
	}
	if utf8.RuneCountInString(base) != 1 {
		return false
	}
	return !mods["shift"] || len(mods) != 1
}

// customKey returns the key a configured custom command fires on, or "" when it
// can never fire: no key, a spelling no keypress produces, a reserved key, or
// no command to run. Dispatch and help both resolve through this, so they can
// never disagree about which keys a custom command has taken over.
func customKey(cc config.CustomCommand, keys KeyMap) string {
	k, _ := classifyCustomKey(cc, keys)
	return k
}

// unusableReason classifies why a configured custom command can never fire on
// its configured key. Dispatch drops such bindings silently through customKey;
// the startup notice reuses the same classification to say which rule the
// config broke, so reporting and dispatch answer from one set of predicates
// and cannot drift apart.
type unusableReason string

const (
	// unusableNone marks a binding dispatch will honor; nothing to report.
	unusableNone unusableReason = ""
	// unusableNoCommand: the entry names no command, so there is nothing to run.
	unusableNoCommand unusableReason = "no command"
	// unusableUnproducible: the spelling matches no keypress ("shift+z").
	unusableUnproducible unusableReason = "unproducible"
	// unusableReserved: the key is one dispatch answers before custom commands.
	unusableReserved unusableReason = "reserved"
	// unusableDuplicate: an earlier entry already claimed the same usable key,
	// and dispatch and help both keep only that first one.
	unusableDuplicate unusableReason = "duplicate"
)

// Copy for the startup notice. The action-menu tail rides on the individual
// reason, not on the message as a whole: a binding that only lost its key does
// still run from the action menu, but one carrying no command has nothing to
// run there either, so promising it a menu entry would be a lie.
const (
	noticeIgnoredOneFmt  = "1 custom command is ignored: %s"
	noticeIgnoredManyFmt = "%d custom commands are ignored: %s"
	noticeDetailSep      = "; "
	noticeUnnamedName    = "(unnamed)"
	noticeReservedFmt    = `key %q is reserved`
	noticeUnproducibleFm = `key %q matches no keypress`
	noticeDuplicateFmt   = `key %q is already taken by an earlier custom command`
	noticeNoCommandText  = "has no command to run"
	noticeActionMenuTail = " (still in the action menu, x)"
)

// detail renders the reason as the fragment the notice quotes back at the
// user, naming the key where the key itself is the problem and promising the
// action menu only where the command really still runs from it.
func (r unusableReason) detail(key string) string {
	switch r {
	case unusableReserved:
		return fmt.Sprintf(noticeReservedFmt, key) + noticeActionMenuTail
	case unusableUnproducible:
		return fmt.Sprintf(noticeUnproducibleFm, key) + noticeActionMenuTail
	case unusableDuplicate:
		return fmt.Sprintf(noticeDuplicateFmt, key) + noticeActionMenuTail
	case unusableNoCommand:
		return noticeNoCommandText
	case unusableNone:
		return ""
	}
	return ""
}

// unusableBinding is one configured custom command whose key will never fire.
type unusableBinding struct {
	name   string
	key    string // exactly as configured, so the user can find the line
	reason unusableReason
}

// describe renders one offending binding as "name: what is wrong with it".
func (b unusableBinding) describe() string {
	name := b.name
	if name == "" {
		name = noticeUnnamedName
	}
	return name + ": " + b.reason.detail(b.key)
}

// classifyCustomKey resolves the exact predicate chain customKey dispatches on
// and additionally names the rule that failed, so the startup notice reports
// from what dispatch actually obeys rather than a parallel copy that could
// drift. An empty command wins over key problems: an entry with nothing to run
// is broken whatever its key does. A missing key with a command present is not
// a failure — the example config documents it as the deliberate action-menu-only
// form.
func classifyCustomKey(cc config.CustomCommand, keys KeyMap) (string, unusableReason) {
	k := normalizeKey(cc.Key)
	if cc.Command == "" {
		return "", unusableNoCommand
	}
	if k == "" {
		return "", unusableNone
	}
	if !producibleKey(k) {
		return "", unusableUnproducible
	}
	if keys.Reserved(k) {
		return "", unusableReserved
	}
	return k, unusableNone
}

// unusableBindings keeps every configured custom command whose key will never
// fire, classified by cause. Duplication is the one cause classifyCustomKey
// cannot see on its own: it is a property of the list, not of the entry, so it
// is resolved here in the same first-wins order customCommandFor and
// withCustomBindings already resolve it in.
func unusableBindings(custom []config.CustomCommand, keys KeyMap) []unusableBinding {
	var out []unusableBinding
	taken := map[string]bool{}
	for _, cc := range custom {
		k, reason := classifyCustomKey(cc, keys)
		if reason == unusableNone && taken[k] {
			reason = unusableDuplicate
		}
		if reason != unusableNone {
			out = append(out, unusableBinding{name: cc.Name, key: cc.Key, reason: reason})
			continue
		}
		if k != "" {
			taken[k] = true
		}
	}
	return out
}

// ignoredKeysNotice summarizes every configured custom command whose key can
// never fire, or "" when none do, so New can surface it once at startup.
func ignoredKeysNotice(custom []config.CustomCommand, keys KeyMap) string {
	bad := unusableBindings(custom, keys)
	if len(bad) == 0 {
		return ""
	}
	details := make([]string, 0, len(bad))
	for _, b := range bad {
		details = append(details, b.describe())
	}
	if len(bad) == 1 {
		return fmt.Sprintf(noticeIgnoredOneFmt, details[0])
	}
	return fmt.Sprintf(noticeIgnoredManyFmt, len(bad), strings.Join(details, noticeDetailSep))
}

// customCommandFor returns the command that fires when key k is pressed, or ""
// when none does.
func customCommandFor(custom []config.CustomCommand, keys KeyMap, k string) string {
	if k == "" {
		return ""
	}
	for _, cc := range custom {
		if customKey(cc, keys) == k {
			return cc.Command
		}
	}
	return ""
}

// activeCustom is one configured custom command dispatch actually honors:
// its key fires, and this entry is the one that owns it.
type activeCustom struct {
	key  string
	name string
}

// activeCustomCommands resolves which custom commands dispatch answers onto
// which keys: reachable keys only, first configuration winning duplicates.
// Help and the resting footer both derive their view of taken keys from this
// single pass through the same predicates customKey dispatch uses, so no
// surface can advertise a key a custom command has taken over.
func activeCustomCommands(custom []config.CustomCommand, keys KeyMap) []activeCustom {
	var out []activeCustom
	seen := map[string]bool{}
	for _, cc := range custom {
		k := customKey(cc, keys)
		if k == "" || seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, activeCustom{key: k, name: cc.Name})
	}
	return out
}

// logViewKeys maps the keys the log view handles itself onto what they do
// there. handleKey serves the log view before it ever reaches a custom command
// key, so these keep working after a custom command takes them over in browse
// mode, and their help row has to say so rather than disappear.
func logViewKeys(k KeyMap) map[string]string {
	return map[string]string{
		k.Follow: "follow / freeze logs (log view)",
		k.Yank:   "yank log line (log view)",
	}
}

// withCustomBindings appends a row per reachable custom command and drops the
// built-in rows whose keys it took over, so help never advertises a meaning a
// key no longer has. A row is dropped whole: its description is prose about the
// keys it lists and cannot be split when only some of them are shadowed. A row
// for a key the log view still answers keeps its line, restated for that view.
func withCustomBindings(base []helpRow, keys KeyMap, custom []config.CustomCommand) []helpRow {
	active := activeCustomCommands(custom, keys)
	if len(active) == 0 {
		return base
	}
	names := make(map[string]string, len(active))
	for _, ac := range active {
		names[ac.key] = ac.name
	}
	live := logViewKeys(keys)
	out := make([]helpRow, 0, len(base)+len(active))
	for _, b := range base {
		tokens := helpKeyTokens(b.key)
		shadowed := ""
		for _, t := range tokens {
			if _, ok := names[t]; ok {
				shadowed = t
				break
			}
		}
		switch {
		case shadowed == "":
			out = append(out, b)
		case len(tokens) == 1 && live[shadowed] != "":
			out = append(out, helpRow{b.key, live[shadowed]})
		}
	}
	for _, ac := range active {
		out = append(out, helpRow{ac.key, customHintLabel(ac.name)})
	}
	return out
}

// helpKeyTokens splits a help row's key column ("pgup / pgdown", "tab / 1 2 3")
// into the keys it documents, spelled the way a keypress spells them. Only
// " / " separates keys, so the filter row ("/") yields the key itself rather
// than nothing.
func helpKeyTokens(s string) []string {
	var out []string
	for _, part := range strings.Split(s, " / ") {
		out = append(out, strings.Fields(part)...)
	}
	return out
}

// Custom-command naming convention shared by the help rows, the action-menu
// entries and the footer hints: a named command reads "custom: name", and an
// unnamed one still says what it is.
const (
	customLabelPrefix   = "custom: "
	customLabelFallback = "custom command"
)

// customHintLabel names a custom command for display under its key.
func customHintLabel(name string) string {
	if name == "" {
		return customLabelFallback
	}
	return customLabelPrefix + name
}

// Resting-footer rendering pieces. Two spaces between hints is the grouping
// the line has always shown; the brackets make each key scannable at a glance.
const (
	footerHintSep    = "  "
	footerHintSpace  = " "
	footerKeyOpen    = "["
	footerKeyClose   = "]"
	footerKeyGroupBy = "/"
)

// Resting-footer labels. Deliberately shorter than their help-row cousins:
// the footer shares one terminal line with the cursor position, so every hint
// must stay readable even after truncation at the smallest supported width.
const (
	footerLabelShell     = "shell"
	footerLabelLogs      = "logs"
	footerLabelLifecycle = "lifecycle"
	footerLabelRun       = "run"
	footerLabelExec      = "exec"
	footerLabelRemove    = "remove"
	footerLabelDelete    = "delete"
	footerLabelPrune     = "prune"
	footerLabelCreate    = "create"
	footerLabelPull      = "pull"
	footerLabelFilter    = "filter"
	footerLabelActions   = "actions"
	footerLabelYank      = "yank"
	footerLabelNavigate  = "navigate"
	footerNoteReadOnly   = "(read-only)"
)

// footerHint is one piece of a view's resting footer line: the keys sharing a
// label, or plain text when keys is empty (the System view's read-only note).
type footerHint struct {
	keys  []string
	label string
}

// render draws one hint as the footer shows it: a bracketed key group followed
// by its label ("[s/u/r] lifecycle"), or the bare text for a key-less note.
func (h footerHint) render() string {
	if len(h.keys) == 0 {
		return h.label
	}
	return footerKeyOpen + strings.Join(h.keys, footerKeyGroupBy) + footerKeyClose +
		footerHintSpace + h.label
}

// footerViewHints lists one view's resting hints in display order, keys taken
// straight from the KeyMap so the line mirrors whatever dispatch answers.
func footerViewHints(view View, keys KeyMap) []footerHint {
	common := []footerHint{
		{[]string{keys.Filter}, footerLabelFilter},
		{[]string{keys.Actions}, footerLabelActions},
		{[]string{keys.Yank}, footerLabelYank},
	}
	switch view {
	case ViewImages:
		return append([]footerHint{
			{[]string{keys.Pull}, footerLabelPull},
			{[]string{keys.Create}, footerLabelRun},
			{[]string{keys.Remove}, footerLabelDelete},
			{[]string{keys.Prune}, footerLabelPrune},
		}, common...)
	case ViewVolumes:
		return append([]footerHint{
			{[]string{keys.Create}, footerLabelCreate},
			{[]string{keys.Remove}, footerLabelDelete},
			{[]string{keys.Prune}, footerLabelPrune},
		}, common...)
	case ViewNetworks:
		// Read-only by scope: no action-menu hint, unlike the other views.
		return []footerHint{
			{[]string{keys.Filter}, footerLabelFilter},
			{[]string{keys.Yank}, footerLabelYank},
		}
	case ViewSystem:
		// Navigation aliases are reserved, so this group can never be cut.
		return []footerHint{
			{[]string{navAliasDown, navAliasUp}, footerLabelNavigate},
			{[]string{keys.Yank}, footerLabelYank},
			{nil, footerNoteReadOnly},
		}
	default:
		return append([]footerHint{
			{[]string{keys.Enter}, footerLabelShell},
			{[]string{keys.Logs}, footerLabelLogs},
			{[]string{keys.Stop, keys.Start, keys.Restart}, footerLabelLifecycle},
			{[]string{keys.Create}, footerLabelRun},
			{[]string{keys.Exec}, footerLabelExec},
			{[]string{keys.Remove}, footerLabelRemove},
		}, common...)
	}
}

// footerHints renders one view's resting hint line from the resolved binding
// view, so the footer can never promise what a key no longer does: a key an
// active custom command has claimed stops advertising its built-in label and
// hands that slot to the owning command instead.
func footerHints(view View, keys KeyMap, custom []config.CustomCommand) string {
	taken := make(map[string]string)
	for _, ac := range activeCustomCommands(custom, keys) {
		taken[ac.key] = customHintLabel(ac.name)
	}
	var parts []string
	for _, h := range footerViewHints(view, keys) {
		switch {
		case len(h.keys) == 0, len(taken) == 0:
			parts = append(parts, h.render())
		default:
			parts = appendSplitHint(parts, h, taken)
		}
	}
	return strings.Join(parts, footerHintSep)
}

// appendSplitHint emits one hint whose key group an active custom command has
// cut into: surviving keys keep the built-in label as a possibly smaller group
// ("[u/r] lifecycle"), and each claimed key passes its slot, in place, to the
// command now firing on it.
func appendSplitHint(parts []string, h footerHint, taken map[string]string) []string {
	live := make([]string, 0, len(h.keys))
	flush := func() {
		if len(live) == 0 {
			return
		}
		parts = append(parts, footerHint{keys: live, label: h.label}.render())
		live = nil
	}
	for _, k := range h.keys {
		label, claimed := taken[k]
		if !claimed {
			live = append(live, k)
			continue
		}
		flush()
		parts = append(parts, footerHint{keys: []string{k}, label: label}.render())
	}
	flush()
	return parts
}
