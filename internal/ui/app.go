package ui

import (
	"context"
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/Laaaaksh/vessel/internal/backend"
	"github.com/Laaaaksh/vessel/internal/config"
	"github.com/Laaaaksh/vessel/internal/ui/containers"
)

// tickMsg is sent on each poll interval.
type tickMsg time.Time

// initMsg is sent after the backend client is created.
type initMsg struct {
	client *backend.Client
	err    error
}

// containersLoadedMsg is sent when the container list refresh completes.
type containersLoadedMsg struct {
	items []backend.Container
	err   error
}

// Model is the root bubbletea model for vessel.
type Model struct {
	cfg    config.Config
	client *backend.Client
	poller *backend.Poller
	keys   KeyMap
	st     styles
	width  int
	height int

	activeView View
	focus      Focus
	showHelp   bool

	cntPanel containers.Model
	lastErr  error
}

// New creates the root model. Backend connection happens in Init.
func New() Model {
	cfg, _ := config.Load()
	return Model{
		cfg:        cfg,
		keys:       DefaultKeyMap(),
		st:         newStyles(),
		activeView: ViewContainers,
		focus:      FocusList,
		cntPanel:   containers.New(),
	}
}

// Init connects to the container backend and kicks off the first poll.
func (m Model) Init() tea.Cmd {
	return func() tea.Msg {
		client, err := backend.NewClient()
		return initMsg{client: client, err: err}
	}
}

// Update routes all incoming messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case initMsg:
		if msg.err != nil {
			m.lastErr = msg.err
			return m, nil
		}
		m.client = msg.client
		m.poller = backend.NewPoller(msg.client, m.cfg.PollInterval.Duration)
		return m, tea.Batch(m.startPollerCmd(), m.loadContainersCmd())
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tickMsg:
		return m, m.loadContainersCmd()
	case containersLoadedMsg:
		return m.applyContainersLoaded(msg)
	case tea.KeyPressMsg:
		return m.handleKey(msg)
	}

	var cmd tea.Cmd
	m.cntPanel, cmd = m.cntPanel.Update(msg)
	return m, cmd
}

// View renders the full UI.
func (m Model) View() tea.View {
	var content string
	if m.width == 0 {
		content = "initialising vessel..."
	} else if m.showHelp {
		content = m.helpView()
	} else {
		sidebarW := 18
		detailW := min(42, m.width/3)
		listW := m.width - sidebarW - detailW

		sidebar := m.sidebarView(sidebarW, m.height-2)
		list := m.cntPanel.ListView(listW, m.height-2)
		detail := m.cntPanel.DetailView(detailW, m.height-2, m.poller)

		body := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, list, detail)
		content = lipgloss.JoinVertical(lipgloss.Left, m.headerView(), body, m.footerView())
	}

	v := tea.NewView(content)
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	return v
}

// --- private ---

func (m Model) startPollerCmd() tea.Cmd {
	return func() tea.Msg {
		go m.poller.Run(context.Background())
		return tickMsg(time.Now())
	}
}

func (m Model) loadContainersCmd() tea.Cmd {
	if m.client == nil {
		return nil
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		items, err := m.client.ListContainers(ctx)
		return containersLoadedMsg{items: items, err: err}
	}
}

func (m Model) applyContainersLoaded(msg containersLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.lastErr = msg.err
	} else {
		m.lastErr = nil
		m.cntPanel = m.cntPanel.SetItems(msg.items)
	}
	return m, tea.Tick(m.cfg.PollInterval.Duration, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	k := msg.String()
	switch {
	case k == m.keys.Quit || k == "ctrl+c":
		return m, tea.Quit
	case k == m.keys.Help:
		m.showHelp = !m.showHelp
		return m, nil
	case k == m.keys.Escape:
		if m.showHelp {
			m.showHelp = false
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.cntPanel, cmd = m.cntPanel.Update(msg)
	return m, cmd
}

func (m Model) headerView() string {
	title := m.st.title.Render("⬡ vessel")
	hint := m.st.dimText.Render("[ ? help ]  [ q quit ]")
	spacer := lipgloss.NewStyle().
		Width(max(0, m.width-lipgloss.Width(title)-lipgloss.Width(hint))).
		Render("")
	return lipgloss.JoinHorizontal(lipgloss.Top, title, spacer, hint)
}

func (m Model) footerView() string {
	if m.lastErr != nil {
		return m.st.errorText.Width(m.width).Render("error: " + m.lastErr.Error())
	}
	return m.st.footerHelp.Width(m.width).
		Render("[enter] shell  [L] logs  [s] stop  [u] start  [r] restart  [d] remove  [/] filter")
}

func (m Model) sidebarView(width, height int) string {
	views := []struct {
		label string
		view  View
	}{
		{"📦 Containers", ViewContainers},
		{"🖼  Images", ViewImages},
		{"💾 Volumes", ViewVolumes},
	}

	var rows []string
	rows = append(rows, m.st.sectionTitle.Width(width).Render("Views"))
	for _, v := range views {
		st := m.st.navItem
		if m.activeView == v.view {
			st = m.st.navItemActive
		}
		rows = append(rows, st.Width(width).Render(v.label))
	}

	rows = append(rows, "")
	rows = append(rows, m.st.sectionTitle.Width(width).Render("Fleet"))
	rows = append(rows, m.st.statRunning.Render(
		fmt.Sprintf("● %d running", m.cntPanel.RunningCount())))
	rows = append(rows, m.st.statStopped.Render(
		fmt.Sprintf("○ %d stopped", m.cntPanel.StoppedCount())))

	return lipgloss.NewStyle().
		Width(width).Height(height).
		Background(colorSurface).
		BorderRight(true).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(colorBorder).
		Render(lipgloss.JoinVertical(lipgloss.Left, rows...))
}

func (m Model) helpView() string {
	var rows []string
	rows = append(rows, m.st.title.Render("vessel - keybindings"))
	rows = append(rows, "")
	for _, b := range helpBindings() {
		key := m.st.helpText.Width(18).Render(b.key)
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, key, b.desc))
	}
	rows = append(rows, "")
	rows = append(rows, m.st.dimText.Render("press ? or esc to close"))

	return lipgloss.NewStyle().
		Width(m.width).Height(m.height).
		Align(lipgloss.Center, lipgloss.Center).
		Render(lipgloss.JoinVertical(lipgloss.Left, rows...))
}

