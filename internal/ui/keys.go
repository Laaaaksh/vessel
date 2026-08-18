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
// must not make the list unscrollable.
func (k KeyMap) Reserved(s string) bool {
	if k.NavUp(s) || k.NavDown(s) {
		return true
	}
	if Match(s, k.Left, k.Right, k.FocusNext, k.FocusPrev,
		k.PageUp, k.PageDown, k.HalfUp, k.HalfDown,
		k.GotoTop, k.GotoBottom, k.Filter, k.Escape, k.Tab,
		k.Quit, k.Help, k.LayoutNext, k.LayoutPrev) {
		return true
	}
	return Match(s, "1", "2", "3", "`", "ctrl+c")
}

// helpBindings returns contextual (key, description) pairs, including the
// custom commands reachable by their configured key.
func helpBindings(view View, focus Focus, mode Mode, keys KeyMap, custom []config.CustomCommand) []struct{ key, desc string } {
	base := []struct{ key, desc string }{
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
		base = append([]struct{ key, desc string }{
			{"p", "pull image (prompt)"},
			{"P", "prune unused images (confirm)"},
			{"d", "delete marked (confirm)"},
			{"c", "run container from image"},
		}, base...)
	case ViewVolumes:
		base = append([]struct{ key, desc string }{
			{"c", "create volume (prompt)"},
			{"P", "prune unused volumes (confirm)"},
			{"d", "delete marked (confirm)"},
		}, base...)
	default:
		base = append([]struct{ key, desc string }{
			{"enter", "open shell in running container"},
			{"L", "view logs"},
			{"s", "stop container"},
			{"u", "start container"},
			{"r", "restart container"},
			{"d", "delete marked (confirm)"},
			{"P", "prune stopped containers (confirm)"},
			{"c", "run new container (prompt image)"},
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

// normalizeKey rewrites a configured key onto the spelling
// tea.KeyPressMsg.String() produces, so config.toml and the runtime agree: a
// literal space and "space" are the same key, "ctrl-x" is "ctrl+x", and named
// keys are case-insensitive (single characters stay case-sensitive, since G and
// g are different keys).
func normalizeKey(s string) string {
	if s == "" {
		return ""
	}
	if strings.TrimSpace(s) == "" {
		return "space"
	}
	prefix, base := "", strings.TrimSpace(s)
	for {
		i := strings.IndexAny(base, "+-")
		if i <= 0 || i == len(base)-1 {
			break
		}
		mod := strings.ToLower(base[:i])
		if !keyModifiers[mod] {
			break
		}
		prefix += mod + "+"
		base = base[i+1:]
	}
	if alias, ok := keyAliases[strings.ToLower(base)]; ok {
		base = alias
	} else if utf8.RuneCountInString(base) > 1 {
		base = strings.ToLower(base)
	}
	return prefix + base
}

// producibleKey reports whether a normalized key can ever be produced by a
// keypress, so help never advertises a binding that cannot fire.
func producibleKey(k string) bool {
	base := k
	for {
		i := strings.Index(base, "+")
		if i <= 0 || i == len(base)-1 {
			break
		}
		if !keyModifiers[base[:i]] {
			break
		}
		base = base[i+1:]
	}
	return runtimeKeyNames[base] || utf8.RuneCountInString(base) == 1
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

// withCustomBindings appends a row per reachable custom command and drops the
// built-in rows whose keys it took over, so help never advertises a meaning a
// key no longer has. A row is dropped whole: its description is prose about the
// keys it lists and cannot be split when only some of them are shadowed.
func withCustomBindings(base []struct{ key, desc string }, keys KeyMap, custom []config.CustomCommand) []struct{ key, desc string } {
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
	out := make([]struct{ key, desc string }, 0, len(base)+len(order))
	for _, b := range base {
		shadowed := false
		for _, t := range helpKeyTokens(b.key) {
			if _, ok := names[t]; ok {
				shadowed = true
				break
			}
		}
		if !shadowed {
			out = append(out, b)
		}
	}
	for _, k := range order {
		desc := "custom command"
		if names[k] != "" {
			desc = "custom: " + names[k]
		}
		out = append(out, struct{ key, desc string }{k, desc})
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
