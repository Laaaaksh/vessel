package containers

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/Laaaaksh/vessel/internal/backend"
	"github.com/Laaaaksh/vessel/internal/ui/uiutil"
)

const (
	colName   = 20
	colStatus = 10
	colCPU    = 8
	colMem    = 14
)

// ListView renders the container list table.
func (m Model) ListView(width, height int, poller *backend.Poller) string {
	header := m.renderHeader(width)

	var filterBar string
	if m.filtering {
		filterBar = lipgloss.NewStyle().Foreground(lipgloss.Color("#60a5fa")).
			Render(fmt.Sprintf("  filter: %s_  (%d/%d)", m.filter, len(m.filtered), len(m.items)))
	} else if m.filter != "" {
		filterBar = lipgloss.NewStyle().Foreground(lipgloss.Color("#6b7280")).
			Render(fmt.Sprintf("  filter: %s  (%d/%d, esc clear)", m.filter, len(m.filtered), len(m.items)))
	}

	rowsH := height - 2
	if filterBar != "" {
		rowsH--
	}
	start, end := uiutil.Window(len(m.filtered), m.cursor, max(1, rowsH))

	var rows []string
	for i := start; i < end; i++ {
		rows = append(rows, m.renderRow(m.filtered[i], i == m.cursor, width, poller))
	}
	if len(m.filtered) == 0 {
		if m.filter != "" {
			rows = append(rows, lipgloss.NewStyle().Foreground(lipgloss.Color("#6b7280")).Render("  no matches"))
		} else {
			rows = append(rows, lipgloss.NewStyle().Foreground(lipgloss.Color("#6b7280")).Render("  no containers"))
		}
	}

	content := lipgloss.JoinVertical(lipgloss.Left,
		append([]string{header}, rows...)...)
	if filterBar != "" {
		content = lipgloss.JoinVertical(lipgloss.Left, content, filterBar)
	}

	return lipgloss.NewStyle().
		Width(width).Height(height).
		Render(content)
}

func (m Model) renderHeader(width int) string {
	cols := []string{
		uiutil.Pad("NAME", colName),
		uiutil.Pad("STATUS", colStatus),
		uiutil.Pad("CPU", colCPU),
		uiutil.Pad("MEMORY", colMem),
		"PORTS",
	}
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6b7280")).
		BorderBottom(true).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("#374151")).
		Width(width).
		Render(strings.Join(cols, " "))
}

func (m Model) renderRow(c backend.Container, selected bool, width int, poller *backend.Poller) string {
	indicator := "○ "
	if c.IsRunning() {
		indicator = "● "
	}
	mark := " "
	if m.marked[c.ID] {
		mark = "*"
	}

	cpu, mem := "-", "-"
	if poller != nil {
		metrics, ok := poller.Snapshot().Get(c.ID)
		if ok {
			cpu = backend.FormatCPU(metrics, true)
			mem = backend.FormatMem(metrics, true)
			if mem == "N/A" {
				mem = "-"
			}
		}
	}

	// Plain text first — one style pass avoids nested colour fights on selection.
	line := fmt.Sprintf("%s%s %s %s %s %s",
		mark,
		uiutil.Pad(indicator+c.Name, colName-1),
		uiutil.Pad(c.Status, colStatus),
		uiutil.Pad(cpu, colCPU),
		uiutil.Pad(mem, colMem),
		backend.FormatPorts(c.Ports),
	)

	if selected {
		return m.styleSelected.Width(width).Render(line)
	}
	st := m.styleExited
	if c.IsRunning() {
		st = m.styleRunning
	}
	return st.Width(width).Render(line)
}

// DetailView renders the right-hand detail panel for the selected container.
func (m Model) DetailView(width, height int, poller *backend.Poller) string {
	sel := m.Selected()
	if sel == nil {
		return lipgloss.NewStyle().Width(width).Height(height).
			Foreground(lipgloss.Color("#6b7280")).
			Render("  no container selected")
	}

	var lines []string
	lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("#a78bfa")).Bold(true).Render(sel.Name))
	lines = append(lines, "")
	lines = append(lines, uiutil.KV("Image", sel.Image))
	lines = append(lines, uiutil.KV("ID", uiutil.Truncate(sel.ID, 12)))
	if !sel.Created.IsZero() {
		lines = append(lines, uiutil.KV("Created", uiutil.Ago(sel.Created)))
	}
	lines = append(lines, uiutil.KV("Status", sel.Status))
	lines = append(lines, uiutil.KV("Ports", backend.FormatPorts(sel.Ports)))

	if poller != nil {
		m2, ok := poller.Snapshot().Get(sel.ID)
		lines = append(lines, uiutil.KV("CPU", backend.FormatCPU(m2, ok)))
		lines = append(lines, uiutil.KV("Memory", backend.FormatMem(m2, ok)))
		if ok {
			lines = append(lines, renderBar(m2.CPUPercent/100.0, width-4))
			if m2.MemLimit > 0 {
				lines = append(lines, renderBar(float64(m2.MemUsage)/float64(m2.MemLimit), width-4))
			}
			if spark := poller.Sparkline(sel.ID, min(24, width-6)); spark != "" {
				lines = append(lines, uiutil.KV("CPU hist", spark))
			}
		}
	}

	if len(sel.Labels) > 0 {
		lines = append(lines, "")
		lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("#6b7280")).Render("-- Labels --"))
		for k, v := range sel.Labels {
			if len(lines) > height-4 {
				break
			}
			lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("#6b7280")).
				Render("  "+uiutil.Truncate(k+"="+v, width-6)))
		}
	}

	if len(sel.Env) > 0 {
		lines = append(lines, "")
		lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("#6b7280")).Render("-- Env --"))
		for _, e := range sel.Env {
			if len(lines) > height-4 {
				break
			}
			lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("#6b7280")).
				Render("  "+uiutil.Truncate(e, width-6)))
		}
	}

	return lipgloss.NewStyle().
		Width(width).Height(height).
		PaddingLeft(1).
		Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}

func renderBar(pct float64, width int) string {
	if pct < 0 {
		pct = 0
	}
	if pct > 1 {
		pct = 1
	}
	barWidth := width - 6
	if barWidth < 4 {
		barWidth = 4
	}
	filled := int(float64(barWidth) * pct)
	bar := lipgloss.NewStyle().Foreground(lipgloss.Color("#34d399")).Render(strings.Repeat("▓", filled)) +
		lipgloss.NewStyle().Foreground(lipgloss.Color("#374151")).Render(strings.Repeat("░", barWidth-filled))
	return lipgloss.NewStyle().Foreground(lipgloss.Color("#6b7280")).Render(fmt.Sprintf("%3.0f%% ", pct*100)) + bar
}
