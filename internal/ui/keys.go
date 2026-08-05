package ui

// KeyMap documents all keybindings for vessel.
// Matching is done via tea.KeyPressMsg.String() in Update().
type KeyMap struct {
	Up         string
	Down       string
	Left       string
	Right      string
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
}

// DefaultKeyMap returns the default keybindings.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up:         "up",
		Down:       "down",
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
	}
}

// helpBindings returns a list of (key, description) pairs for the help overlay.
func helpBindings() []struct{ key, desc string } {
	return []struct{ key, desc string }{
		{"j / ↓", "move down"},
		{"k / ↑", "move up"},
		{"g / G", "top / bottom"},
		{"enter", "open shell in container"},
		{"L", "view logs"},
		{"s", "stop container"},
		{"u", "start container"},
		{"r", "restart container"},
		{"d", "remove (confirm with y)"},
		{"/", "filter containers"},
		{"tab / 1 2 3", "switch Containers / Images / Volumes"},
		{"?", "toggle help"},
		{"q / ctrl+c", "quit"},
	}
}
