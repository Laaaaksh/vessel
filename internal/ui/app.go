package ui

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/Laaaaksh/vessel/internal/backend"
	"github.com/Laaaaksh/vessel/internal/config"
	"github.com/Laaaaksh/vessel/internal/ui/containers"
	"github.com/Laaaaksh/vessel/internal/ui/images"
	"github.com/Laaaaksh/vessel/internal/ui/logs"
	"github.com/Laaaaksh/vessel/internal/ui/volumes"
)

type tickMsg time.Time

type initMsg struct {
	client *backend.Client
	err    error
}

type containersLoadedMsg struct {
	items []backend.Container
	err   error
}

type imagesLoadedMsg struct {
	items []backend.Image
	err   error
}

type volumesLoadedMsg struct {
	items []backend.Volume
	err   error
}

type actionDoneMsg struct {
	err error
	msg string
}

type logsOpenedMsg struct {
	title string
	lines []string
	err   error
}

type logLineMsg struct {
	text string
}

type logErrMsg struct {
	err error
}

type shellDoneMsg struct {
	err error
}

// Mode is the top-level UI mode.
type Mode int

const (
	modeBrowse Mode = iota
	modeLogs
	modeConfirmDelete
)

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
	mode       Mode
	showHelp   bool

	cntPanel containers.Model
	imgPanel images.Model
	volPanel volumes.Model
	logPanel logs.Model

	lastErr   error
	status    string
	pending   string // id/name awaiting delete confirm
	logCancel context.CancelFunc
}

// New creates the root model. Backend connection happens in Init.
func New() Model {
	cfg, _ := config.Load()
	return Model{
		cfg:        cfg,
		keys:       DefaultKeyMap(),
		st:         newStyles(),
		activeView: ViewContainers,
		mode:       modeBrowse,
		cntPanel:   containers.New(),
		imgPanel:   images.New(),
		volPanel:   volumes.New(),
		logPanel:   logs.New(),
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
		return m, tea.Batch(m.startPollerCmd(), m.refreshCmd())
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.logPanel = m.logPanel.SetSize(msg.Width, msg.Height)
		return m, nil
	case tickMsg:
		return m, m.refreshCmd()
	case containersLoadedMsg:
		return m.applyContainersLoaded(msg)
	case imagesLoadedMsg:
		if msg.err != nil {
			m.lastErr = msg.err
		} else {
			m.imgPanel = m.imgPanel.SetItems(msg.items)
		}
		return m, nil
	case volumesLoadedMsg:
		if msg.err != nil {
			m.lastErr = msg.err
		} else {
			m.volPanel = m.volPanel.SetItems(msg.items)
		}
		return m, nil
	case actionDoneMsg:
		if msg.err != nil {
			m.lastErr = msg.err
			m.status = ""
		} else {
			m.lastErr = nil
			m.status = msg.msg
		}
		m.mode = modeBrowse
		m.pending = ""
		return m, m.refreshCmd()
	case logsOpenedMsg:
		if msg.err != nil {
			m.lastErr = msg.err
			return m, nil
		}
		m.mode = modeLogs
		m.logPanel = m.logPanel.Open(msg.title, msg.lines).SetSize(m.width, m.height)
		sel := m.cntPanel.Selected()
		if sel == nil || m.client == nil {
			return m, nil
		}
		if m.logCancel != nil {
			m.logCancel()
		}
		ctx, cancel := context.WithCancel(context.Background())
		m.logCancel = cancel
		ch := make(chan backend.LogLine, 64)
		logCh = ch
		client := m.client
		id := sel.ID
		go func() {
			_ = client.StreamLogs(ctx, id, ch)
			close(ch)
		}()
		return m, m.waitLogLineCmd()
	case logLineMsg:
		m.logPanel = m.logPanel.Append(msg.text)
		return m, m.waitLogLineCmd()
	case logErrMsg:
		if msg.err != nil && msg.err != context.Canceled {
			m.logPanel = m.logPanel.SetError(msg.err.Error())
		}
		return m, nil
	case shellDoneMsg:
		if msg.err != nil {
			m.lastErr = msg.err
		}
		return m, m.refreshCmd()
	case tea.KeyPressMsg:
		return m.handleKey(msg)
	}

	return m, nil
}

// View renders the full UI.
func (m Model) View() tea.View {
	var content string
	if m.width == 0 {
		content = "initialising vessel..."
	} else if m.showHelp {
		content = m.helpView()
	} else if m.mode == modeLogs {
		content = m.logPanel.View()
	} else {
		sidebarW := 18
		detailW := min(42, m.width/3)
		listW := m.width - sidebarW - detailW

		sidebar := m.sidebarView(sidebarW, m.height-2)
		list, detail := m.mainPanels(listW, detailW, m.height-2)
		body := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, list, detail)
		content = lipgloss.JoinVertical(lipgloss.Left, m.headerView(), body, m.footerView())
	}

	v := tea.NewView(content)
	v.AltScreen = true
	if m.cfg.MouseEnabled {
		v.MouseMode = tea.MouseModeCellMotion
	}
	return v
}

func (m Model) mainPanels(listW, detailW, height int) (string, string) {
	switch m.activeView {
	case ViewImages:
		return m.imgPanel.ListView(listW, height), m.imgPanel.DetailView(detailW, height)
	case ViewVolumes:
		return m.volPanel.ListView(listW, height), m.volPanel.DetailView(detailW, height)
	default:
		return m.cntPanel.ListView(listW, height, m.poller), m.cntPanel.DetailView(detailW, height, m.poller)
	}
}

func (m Model) startPollerCmd() tea.Cmd {
	return func() tea.Msg {
		go m.poller.Run(context.Background())
		return tickMsg(time.Now())
	}
}

func (m Model) refreshCmd() tea.Cmd {
	if m.client == nil {
		return nil
	}
	return tea.Batch(m.loadContainersCmd(), m.loadImagesCmd(), m.loadVolumesCmd())
}

func (m Model) loadContainersCmd() tea.Cmd {
	client := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		items, err := client.ListContainers(ctx)
		return containersLoadedMsg{items: items, err: err}
	}
}

func (m Model) loadImagesCmd() tea.Cmd {
	client := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		items, err := client.ListImages(ctx)
		return imagesLoadedMsg{items: items, err: err}
	}
}

func (m Model) loadVolumesCmd() tea.Cmd {
	client := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		items, err := client.ListVolumes(ctx)
		return volumesLoadedMsg{items: items, err: err}
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

	if m.mode == modeLogs {
		if k == "esc" || k == "q" {
			if m.logCancel != nil {
				m.logCancel()
				m.logCancel = nil
			}
			m.mode = modeBrowse
			return m, nil
		}
		var cmd tea.Cmd
		m.logPanel, cmd = m.logPanel.Update(msg)
		return m, cmd
	}

	if m.mode == modeConfirmDelete {
		switch k {
		case "y", "d":
			return m.confirmDelete()
		case "n", "esc":
			m.mode = modeBrowse
			m.pending = ""
			m.status = "delete cancelled"
			return m, nil
		}
		return m, nil
	}

	switch k {
	case m.keys.Quit, "ctrl+c":
		return m, tea.Quit
	case m.keys.Help:
		m.showHelp = !m.showHelp
		return m, nil
	case m.keys.Escape:
		if m.showHelp {
			m.showHelp = false
			return m, nil
		}
	case "tab":
		m.activeView = (m.activeView + 1) % 3
		return m, nil
	case "1":
		m.activeView = ViewContainers
		return m, nil
	case "2":
		m.activeView = ViewImages
		return m, nil
	case "3":
		m.activeView = ViewVolumes
		return m, nil
	}

	if m.showHelp {
		return m, nil
	}

	// Action keys for containers
	if m.activeView == ViewContainers && !m.cntPanel.Filtering() {
		switch k {
		case m.keys.Stop, "s":
			return m.runOnSelected("stop", func(ctx context.Context, id string) error {
				return m.client.StopContainer(ctx, id)
			})
		case m.keys.Start, "u":
			return m.runOnSelected("start", func(ctx context.Context, id string) error {
				return m.client.StartContainer(ctx, id)
			})
		case m.keys.Restart, "r":
			return m.runOnSelected("restart", func(ctx context.Context, id string) error {
				return m.client.RestartContainer(ctx, id)
			})
		case m.keys.Remove, "d":
			sel := m.cntPanel.Selected()
			if sel == nil {
				m.status = "nothing selected"
				return m, nil
			}
			m.mode = modeConfirmDelete
			m.pending = sel.ID
			m.status = fmt.Sprintf("delete %s? [y/n]", sel.Name)
			return m, nil
		case m.keys.Logs, "L":
			return m.openLogs()
		case m.keys.Enter, "enter":
			return m.openShell()
		}
	}

	if m.activeView == ViewImages {
		if k == "d" {
			sel := m.imgPanel.Selected()
			if sel == nil {
				return m, nil
			}
			m.mode = modeConfirmDelete
			m.pending = "image:" + sel.ID
			m.status = fmt.Sprintf("delete image %s? [y/n]", backend.FormatRef(*sel))
			return m, nil
		}
		var cmd tea.Cmd
		m.imgPanel, cmd = m.imgPanel.Update(msg)
		return m, cmd
	}

	if m.activeView == ViewVolumes {
		if k == "d" {
			sel := m.volPanel.Selected()
			if sel == nil {
				return m, nil
			}
			m.mode = modeConfirmDelete
			m.pending = "volume:" + sel.Name
			m.status = fmt.Sprintf("delete volume %s? [y/n]", sel.Name)
			return m, nil
		}
		var cmd tea.Cmd
		m.volPanel, cmd = m.volPanel.Update(msg)
		return m, cmd
	}

	var cmd tea.Cmd
	m.cntPanel, cmd = m.cntPanel.Update(msg)
	return m, cmd
}

func (m Model) confirmDelete() (tea.Model, tea.Cmd) {
	pending := m.pending
	client := m.client
	m.mode = modeBrowse
	m.pending = ""
	return m, func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		var err error
		switch {
		case len(pending) > 6 && pending[:6] == "image:":
			err = client.RemoveImage(ctx, pending[6:])
		case len(pending) > 7 && pending[:7] == "volume:":
			err = client.RemoveVolume(ctx, pending[7:])
		default:
			err = client.RemoveContainer(ctx, pending)
		}
		if err != nil {
			return actionDoneMsg{err: err}
		}
		return actionDoneMsg{msg: "deleted"}
	}
}

func (m Model) runOnSelected(verb string, fn func(context.Context, string) error) (tea.Model, tea.Cmd) {
	sel := m.cntPanel.Selected()
	if sel == nil {
		m.status = "nothing selected"
		return m, nil
	}
	id := sel.ID
	name := sel.Name
	m.status = verb + " " + name + "…"
	return m, func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := fn(ctx, id); err != nil {
			return actionDoneMsg{err: err}
		}
		return actionDoneMsg{msg: verb + " " + name}
	}
}

func (m Model) openLogs() (tea.Model, tea.Cmd) {
	sel := m.cntPanel.Selected()
	if sel == nil {
		m.status = "nothing selected"
		return m, nil
	}
	id := sel.ID
	name := sel.Name
	client := m.client
	n := m.cfg.LogTailLines
	return m, func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		lines, err := client.TailLogs(ctx, id, n)
		return logsOpenedMsg{title: name, lines: lines, err: err}
	}
}

// logStream holds the channel for the active log follow.
var logCh chan backend.LogLine

func (m Model) waitLogLineCmd() tea.Cmd {
	ch := logCh
	if ch == nil {
		return nil
	}
	return func() tea.Msg {
		line, ok := <-ch
		if !ok {
			return logErrMsg{err: context.Canceled}
		}
		return logLineMsg{text: line.Text}
	}
}

func (m Model) openShell() (tea.Model, tea.Cmd) {
	sel := m.cntPanel.Selected()
	if sel == nil {
		m.status = "nothing selected"
		return m, nil
	}
	if !sel.IsRunning() {
		m.status = "container is not running"
		return m, nil
	}
	cmd := m.client.ShellCmd(sel.ID)
	return m, tea.ExecProcess(cmd, func(err error) tea.Msg {
		// Treat exit as success if the process ran; surface real launch errors.
		if err != nil {
			if ee, ok := err.(*exec.ExitError); ok {
				_ = ee
				return shellDoneMsg{}
			}
			return shellDoneMsg{err: err}
		}
		return shellDoneMsg{}
	})
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
	if m.status != "" {
		return m.st.footerHelp.Width(m.width).Render(m.status)
	}
	switch m.activeView {
	case ViewImages:
		return m.st.footerHelp.Width(m.width).
			Render("[tab] views  [d] delete image  [j/k] move  [?] help")
	case ViewVolumes:
		return m.st.footerHelp.Width(m.width).
			Render("[tab] views  [d] delete volume  [j/k] move  [?] help")
	default:
		return m.st.footerHelp.Width(m.width).
			Render("[enter] shell  [L] logs  [s] stop  [u] start  [r] restart  [d] remove  [/] filter  [tab] views")
	}
}

func (m Model) sidebarView(width, height int) string {
	views := []struct {
		label string
		view  View
		key   string
	}{
		{"📦 Containers", ViewContainers, "1"},
		{"🖼  Images", ViewImages, "2"},
		{"💾 Volumes", ViewVolumes, "3"},
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
