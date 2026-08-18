package ui

import (
	"strings"

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
			{"s / u / r", "stop / start / restart"},
			{"d", "delete marked (confirm)"},
			{"P", "prune stopped containers (confirm)"},
			{"c", "run new container (prompt image)"},
		}, base...)
	}
	_ = focus
	_ = mode
	return withCustomBindings(base, keys, custom)
}

// withCustomBindings appends a row per reachable custom command and strips the
// key it shadows from the built-in rows, so help never advertises a meaning a
// key no longer has.
func withCustomBindings(base []struct{ key, desc string }, keys KeyMap, custom []config.CustomCommand) []struct{ key, desc string } {
	names := map[string]string{}
	var order []string
	for _, cc := range custom {
		if cc.Key == "" || keys.Reserved(cc.Key) {
			continue
		}
		if _, dup := names[cc.Key]; dup {
			continue
		}
		names[cc.Key] = cc.Name
		order = append(order, cc.Key)
	}
	if len(order) == 0 {
		return base
	}
	out := make([]struct{ key, desc string }, 0, len(base)+len(order))
	for _, b := range base {
		tokens := helpKeyTokens(b.key)
		kept := make([]string, 0, len(tokens))
		shadowed := false
		for _, t := range tokens {
			if _, ok := names[t]; ok {
				shadowed = true
				continue
			}
			kept = append(kept, t)
		}
		switch {
		case !shadowed:
			out = append(out, b)
		case len(kept) > 0:
			out = append(out, struct{ key, desc string }{strings.Join(kept, " / "), b.desc})
		}
	}
	for _, k := range order {
		desc := "custom command"
		if names[k] != "" {
			desc = "custom: " + names[k]
		}
		label := k
		if label == " " {
			label = "space"
		}
		out = append(out, struct{ key, desc string }{label, desc})
	}
	return out
}

// helpKeyTokens splits a help row's key column ("s / u / r") into the single
// keys it documents, mapping the "space" label back to the key it stands for.
func helpKeyTokens(s string) []string {
	var out []string
	for _, t := range strings.FieldsFunc(s, func(r rune) bool { return r == ' ' || r == '/' }) {
		if t == "space" {
			t = " "
		}
		out = append(out, t)
	}
	return out
}
