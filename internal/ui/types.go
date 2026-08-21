package ui

// View represents the active panel in the sidebar.
type View int

// Sidebar views.
const (
	ViewContainers View = iota
	ViewImages
	ViewVolumes
	ViewNetworks
)

// numViews is the count of View constants above, for cycling arithmetic.
const numViews = 4
