package ui

// View represents the active panel in the sidebar.
type View int

// Sidebar views.
const (
	ViewContainers View = iota
	ViewImages
	ViewVolumes
)
