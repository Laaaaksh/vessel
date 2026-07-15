package containers

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/Laaaaksh/vessel/internal/backend"
)

const (
	colName   = 20
	colStatus = 10
	colCPU    = 8
	colMem    = 14
)

// ListView renders the container list table.
func (m Model) ListView(width, height int) string {
	header := m.renderHeader(width)
	var rows []string
	for i, c := range m.filtered {
		rows = append(rows, m.renderRow(c, i == m.cursor, width))
	}
	if len(m.filtered) == 0 {
		if m.filter != "" {
			rows = append(rows, lipgloss.NewStyle().Foreground(lipgloss.Color("#6b7280")).Render("  no matches"))
		} else {
			rows = append(rows, lipgloss.NewStyle().Foreground(lipgloss.Color("#6b7280")).Render("  no containers"))
		}
	}

	var filterBar string
	if m.filtering {
		filterBar = lipgloss.NewStyle().Foreground(lipgloss.Color("#60a5fa")).
			Render(fmt.Sprintf("  filter: %s_", m.filter))
	} else if m.filter != "" {
		filterBar = lipgloss.NewStyle().Foreground(lipgloss.Color("#6b7280")).
			Render(fmt.Sprintf("  filter: %s  (esc to clear)", m.filter))
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
		pad("NAME", colName),
		pad("STATUS", colStatus),
		pad("CPU", colCPU),
		pad("MEMORY", colMem),
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

func (m Model) renderRow(c backend.Container, selected bool, width int) string {
	indicator := "○ "
	statusStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6b7280"))
	if c.IsRunning() {
		indicator = "● "
		statusStyle = m.styleRunning
	}

	name := truncate(indicator+c.Name, colName)
	status := pad(c.Status, colStatus)
	cpu := pad("-", colCPU)
	mem := pad("-", colMem)
	ports := backend.FormatPorts(c.Ports)

	row := lipgloss.JoinHorizontal(lipgloss.Top,
		statusStyle.Render(name)+" ",
		statusStyle.Render(status)+" ",
		lipgloss.NewStyle().Width(colCPU).Render(cpu)+" ",
		lipgloss.NewStyle().Width(colMem).Render(mem)+" ",
		lipgloss.NewStyle().Foreground(lipgloss.Color("#60a5fa")).Render(ports),
	)

	if selected {
		return m.styleSelected.Width(width).Render(row)
	}
	return m.styleRow.Width(width).Render(row)
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
	lines = append(lines, kv("Image", sel.Image))
	lines = append(lines, kv("ID", truncate(sel.ID, 12)+"..."))
	if !sel.Created.IsZero() {
		lines = append(lines, kv("Created", ago(sel.Created)))
	}
	lines = append(lines, kv("Status", sel.Status))
	lines = append(lines, kv("Ports", backend.FormatPorts(sel.Ports)))

	if poller != nil {
		m2, ok := poller.Snapshot().Get(sel.ID)
		lines = append(lines, kv("CPU", backend.FormatCPU(m2, ok)))
		lines = append(lines, kv("Memory", backend.FormatMem(m2, ok)))
		if ok {
			lines = append(lines, renderBar("CPU", m2.CPUPercent/100.0, width-4))
			if m2.MemLimit > 0 {
				lines = append(lines, renderBar("Mem", float64(m2.MemUsage)/float64(m2.MemLimit), width-4))
			}
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
				Render("  "+truncate(e, width-6)))
		}
	}

	return lipgloss.NewStyle().
		Width(width).Height(height).
		PaddingLeft(1).
		Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}

func kv(key, val string) string {
	k := lipgloss.NewStyle().Foreground(lipgloss.Color("#6b7280")).Width(9).Render(key+":")
	v := lipgloss.NewStyle().Foreground(lipgloss.Color("#e2e8f0")).Render(val)
	return lipgloss.JoinHorizontal(lipgloss.Top, k, " ", v)
}

func renderBar(label string, pct float64, width int) string {
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

func pad(s string, w int) string {
	return fmt.Sprintf("%-*s", w, truncate(s, w))
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}

func ago(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return t.Format("2006-01-02")
	}
}
