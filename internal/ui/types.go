package ui

// View represents the active panel in the sidebar.
type View int

// Sidebar views.
const (
	ViewContainers View = iota
	ViewImages
	ViewVolumes
)

// Panel is a panel that can be focused and rendered.
type Panel interface {
	Width() int
	Height() int
}
