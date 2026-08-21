package ui

import (
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

// Match reports whether k equals any of the candidates.
func Match(k string, candidates ...string) bool {
	for _, c := range candidates {
		if c != "" && k == c {
			return true
		}
	}
	return false
}

// NavDown matches down / j.
func (k KeyMap) NavDown(s string) bool { return Match(s, k.Down, "j") }

// NavUp matches up / k.
func (k KeyMap) NavUp(s string) bool { return Match(s, k.Up, "k") }

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
	return Match(s, "1", "2", "3", "`", "ctrl+c")
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
		{"tab / 1 2 3", "switch Containers / Images / Volumes"},
		{"+ / _", "cycle layout"},
		{"space", "toggle multi-select mark"},
		{"y", "yank id/name to clipboard"},
		{"x", "action menu"},
		{"/", "filter"},
		{"f", "follow / freeze logs (log view)"},
		{"`", "toggle command log"},
		{"?", "toggle help"},
		{"esc", "cancel / close"},
		{"q / ctrl+c", "quit"},
	}
	switch view {
	case ViewImages:
		base = append([]helpRow{
			{"p", "pull image (prompt)"},
			{"P", "prune unused images (confirm)"},
			{"d", "delete marked (confirm)"},
			{"c", "run container from image (form)"},
			{"x → image mobility", "tag, save, load, push (push confirms)"},
			{"", "large save/load/push may be cut off by a 10s CLI cap"},
		}, base...)
	case ViewVolumes:
		base = append([]helpRow{
			{"c", "create volume (prompt)"},
			{"P", "prune unused volumes (confirm)"},
			{"d", "delete marked (confirm)"},
		}, base...)
	default:
		base = append([]helpRow{
			{"enter", "open shell in running container"},
			{"L", "view logs"},
			{"s", "stop container"},
			{"u", "start container"},
			{"r", "restart container"},
			{"d", "delete marked (confirm)"},
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
	k := normalizeKey(cc.Key)
	if k == "" || cc.Command == "" || !producibleKey(k) || keys.Reserved(k) {
		return ""
	}
	return k
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
	names := map[string]string{}
	var order []string
	for _, cc := range custom {
		k := customKey(cc, keys)
		if k == "" {
			continue
		}
		if _, dup := names[k]; dup {
			continue
		}
		names[k] = cc.Name
		order = append(order, k)
	}
	if len(order) == 0 {
		return base
	}
	live := logViewKeys(keys)
	out := make([]helpRow, 0, len(base)+len(order))
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
	for _, k := range order {
		desc := "custom command"
		if names[k] != "" {
			desc = "custom: " + names[k]
		}
		out = append(out, helpRow{k, desc})
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
