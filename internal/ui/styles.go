package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

type styles struct {
	// Base surfaces
	appBg     lipgloss.Style
	sidebarBg lipgloss.Style
	mainBg    lipgloss.Style
	detailBg  lipgloss.Style

	// Header / title
	title        lipgloss.Style
	sectionTitle lipgloss.Style

	// Sidebar items
	navItemActive lipgloss.Style
	navItem       lipgloss.Style
	statRunning   lipgloss.Style
	statStopped   lipgloss.Style

	// Table
	tableHeader      lipgloss.Style
	tableRowSelected lipgloss.Style
	tableRow         lipgloss.Style

	// Status colours
	statusRunning lipgloss.Style
	statusExited  lipgloss.Style
	statusOther   lipgloss.Style

	// Detail panel
	detailKey   lipgloss.Style
	detailValue lipgloss.Style
	logLine     lipgloss.Style
	logDim      lipgloss.Style

	// Bars
	barFill  lipgloss.Style
	barEmpty lipgloss.Style

	// Misc
	dimText    lipgloss.Style
	errorText  lipgloss.Style
	helpText   lipgloss.Style
	border     lipgloss.Style
	footerHelp lipgloss.Style
}

var (
	colorPurple   = lipgloss.Color("#a78bfa")
	colorGreen    = lipgloss.Color("#34d399")
	colorYellow   = lipgloss.Color("#fbbf24")
	colorRed      = lipgloss.Color("#f87171")
	colorBlue     = lipgloss.Color("#60a5fa")
	colorDim      = lipgloss.Color("#6b7280")
	colorBase     = lipgloss.Color("#0d0d0d")
	colorSurface  = lipgloss.Color("#111827")
	colorSelected = lipgloss.Color("#2d1b69")
	colorText     = lipgloss.Color("#e2e8f0")
	colorBorder   = lipgloss.Color("#374151")
)

func newStyles() styles {
	return styles{
		appBg:     lipgloss.NewStyle().Background(colorBase),
		sidebarBg: lipgloss.NewStyle().Background(colorSurface),
		mainBg:    lipgloss.NewStyle().Background(colorBase),
		detailBg: lipgloss.NewStyle().Background(colorSurface).
			BorderLeft(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(colorBorder),

		title: lipgloss.NewStyle().
			Foreground(colorPurple).
			Bold(true),
		sectionTitle: lipgloss.NewStyle().
			Foreground(colorDim).
			Transform(strings.ToUpper).
			MarginBottom(1),

		navItemActive: lipgloss.NewStyle().
			Background(colorSelected).
			Foreground(colorPurple).
			PaddingLeft(1).PaddingRight(1),
		navItem: lipgloss.NewStyle().
			Foreground(colorDim).
			PaddingLeft(1).PaddingRight(1),
		statRunning: lipgloss.NewStyle().Foreground(colorGreen),
		statStopped: lipgloss.NewStyle().Foreground(colorDim),

		tableHeader: lipgloss.NewStyle().
			Foreground(colorDim).
			Transform(strings.ToUpper).
			BorderBottom(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(colorBorder),
		tableRowSelected: lipgloss.NewStyle().
			Background(colorSelected).
			Foreground(colorPurple),
		tableRow: lipgloss.NewStyle().Foreground(colorText),

		statusRunning: lipgloss.NewStyle().Foreground(colorGreen),
		statusExited:  lipgloss.NewStyle().Foreground(colorDim),
		statusOther:   lipgloss.NewStyle().Foreground(colorYellow),

		detailKey:   lipgloss.NewStyle().Foreground(colorDim).Width(10),
		detailValue: lipgloss.NewStyle().Foreground(colorText),
		logLine:     lipgloss.NewStyle().Foreground(colorText),
		logDim:      lipgloss.NewStyle().Foreground(colorDim),

		barFill:  lipgloss.NewStyle().Foreground(colorGreen),
		barEmpty: lipgloss.NewStyle().Foreground(colorDim),

		dimText:    lipgloss.NewStyle().Foreground(colorDim),
		errorText:  lipgloss.NewStyle().Foreground(colorRed),
		helpText:   lipgloss.NewStyle().Foreground(colorBlue),
		border:     lipgloss.NewStyle().Foreground(colorBorder),
		footerHelp: lipgloss.NewStyle().Foreground(colorDim),
	}
}
