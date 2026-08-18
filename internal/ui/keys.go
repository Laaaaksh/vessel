package ui

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

// helpBindings returns contextual (key, description) pairs.
func helpBindings(view View, focus Focus, mode Mode) []struct{ key, desc string } {
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
	return base
}
