package ui

// View represents the active panel in the sidebar.
type View int

// Sidebar views.
const (
	ViewContainers View = iota
	ViewImages
	ViewVolumes
	ViewSystem
	ViewNetworks
)

// viewCount is the number of sidebar views. Cycling activeView uses this
// instead of a hardcoded literal, so it cannot silently drift from the enum
// above whenever a view is added or removed.
const viewCount = 5
