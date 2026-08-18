package ui

// Focus is which pane receives navigation keys.
type Focus int

// Panes that can hold focus, in cycle order.
const (
	FocusSidebar Focus = iota
	FocusList
	FocusDetail
)

func (f Focus) String() string {
	switch f {
	case FocusSidebar:
		return "sidebar"
	case FocusDetail:
		return "detail"
	default:
		return "list"
	}
}

// LayoutMode controls browse layout proportions.
type LayoutMode int

// Available browse layout proportions.
const (
	LayoutNormal LayoutMode = iota
	LayoutWideList
	LayoutLogsEmphasis
)

func (l LayoutMode) String() string {
	switch l {
	case LayoutWideList:
		return "wide"
	case LayoutLogsEmphasis:
		return "logs"
	default:
		return "normal"
	}
}
