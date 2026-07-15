package ui

// View represents the active panel in the sidebar.
type View int

const (
	ViewContainers View = iota
	ViewImages
	ViewVolumes
)

// Focus tracks which panel has keyboard focus.
type Focus int

const (
	FocusSidebar Focus = iota
	FocusList
	FocusDetail
)

// Panel is a panel that can be focused and rendered.
type Panel interface {
	Width() int
	Height() int
}
