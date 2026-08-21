package ui

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/Laaaaksh/vessel/internal/backend"
	"github.com/Laaaaksh/vessel/internal/config"
	"github.com/Laaaaksh/vessel/internal/ui/containers"
	"github.com/Laaaaksh/vessel/internal/ui/images"
	"github.com/Laaaaksh/vessel/internal/ui/logs"
	"github.com/Laaaaksh/vessel/internal/ui/uiutil"
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
	// pushRef names the image an image push was attempted for, empty for every
	// other verb. It both limits credential advice to the verb the user actually
	// ran and pins the advice to the row it is about.
	pushRef string
}

type logsOpenedMsg struct {
	id    string
	title string
	lines []string
	err   error
}

type logLineMsg struct {
	id   string
	text string
}

type logErrMsg struct {
	err error
}

type logStreamEndMsg struct{}

type shellDoneMsg struct {
	err error
}

type promptDoneMsg struct {
	kind string
	text string
}

// Mode is the top-level UI mode.
type Mode int

const (
	modeBrowse Mode = iota
	modeLogs
	modeConfirmDelete
	modeShell
	modeActions
	modePrompt
)

// deleteKind names which panel a staged delete belongs to, so the confirmation
// carries its targets as plain ids instead of a prefix-encoded string.
type deleteKind int

const (
	deleteContainers deleteKind = iota
	deleteImages
	deleteVolumes
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
	focus      Focus
	layout     LayoutMode
	mode       Mode
	showHelp   bool
	showCmdLog bool

	cntPanel containers.Model
	imgPanel images.Model
	volPanel volumes.Model
	logPanel logs.Model

	lastErr     error
	status      string
	pendingKind deleteKind
	pendingIDs  []string
	pendingLbl  string
	pendingVerb string
	pendingAct  func(Model) (Model, tea.Cmd)
	logCancel   context.CancelFunc
	logCh       chan backend.LogLine
	logID       string
	pollCancel  context.CancelFunc
	tickPaused  bool

	actionIdx   int
	actionItems []actionItem
	promptKind  string
	promptLabel string
	promptBuf   string
	promptRef   string
}

type actionItem struct {
	label string
	run   func(Model) (Model, tea.Cmd)
}

// New creates the root model. Backend connection happens in Init.
func New() Model {
	cfg, _ := config.Load()
	m := Model{
		cfg:        cfg,
		st:         newStyles(),
		activeView: ViewContainers,
		focus:      FocusList,
		layout:     LayoutNormal,
		mode:       modeBrowse,
		cntPanel:   containers.New(),
		imgPanel:   images.New(),
		volPanel:   volumes.New(),
		logPanel:   logs.New(),
	}
	return m.withKeys(DefaultKeyMap())
}

// withKeys installs k and hands the panels the bindings they match themselves,
// so a rebound key reaches them instead of a hardcoded literal.
func (m Model) withKeys(k KeyMap) Model {
	m.keys = k
	m.cntPanel = m.cntPanel.SetToggleMarkKey(k.ToggleMark)
	m.imgPanel = m.imgPanel.SetToggleMarkKey(k.ToggleMark)
	m.volPanel = m.volPanel.SetToggleMarkKey(k.ToggleMark)
	return m
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
		ctx, cancel := context.WithCancel(context.Background())
		m.pollCancel = cancel
		return m, m.startPollerCmd(ctx)
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		rows := max(5, msg.Height-6)
		m.cntPanel = m.cntPanel.SetPageRows(rows)
		m.imgPanel = m.imgPanel.SetPageRows(rows)
		m.volPanel = m.volPanel.SetPageRows(rows)
		m.logPanel = m.logPanel.SetSize(msg.Width, msg.Height)
		return m, nil
	case tickMsg:
		if m.tickPaused || m.mode == modeShell {
			return m, nil
		}
		return m, tea.Batch(m.refreshCmd(), m.scheduleTickCmd())
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
		m.imgPanel = m.imgPanel.SetNotice("", "")
		if msg.err != nil {
			m.lastErr = msg.err
			m.status = ""
			if msg.pushRef != "" {
				if notice := backend.PushDenialNotice(msg.err); notice != "" {
					m.imgPanel = m.imgPanel.SetNotice(msg.pushRef, notice)
				}
			}
		} else {
			m.lastErr = nil
			m.status = msg.msg
		}
		m = m.clearPending()
		return m, m.refreshCmd()
	case logsOpenedMsg:
		if msg.err != nil {
			m.lastErr = msg.err
			return m, nil
		}
		m.mode = modeLogs
		m.logPanel = m.logPanel.Open(msg.title, msg.lines).SetSize(m.width, m.height)
		if m.client == nil {
			return m, nil
		}
		if m.logCancel != nil {
			m.logCancel()
		}
		ctx, cancel := context.WithCancel(context.Background())
		m.logCancel = cancel
		m.logCh = make(chan backend.LogLine, 64)
		m.logID = msg.id
		return m, tea.Batch(m.streamLogsCmd(ctx, msg.id, m.logCh), m.waitLogLineCmd())
	case logLineMsg:
		if msg.id != m.logID {
			return m, nil
		}
		m.logPanel = m.logPanel.Append(msg.text)
		return m, m.waitLogLineCmd()
	case logStreamEndMsg:
		return m, nil
	case logErrMsg:
		if msg.err != nil && !errors.Is(msg.err, context.Canceled) {
			m.logPanel = m.logPanel.SetError(msg.err.Error())
		}
		return m, nil
	case shellDoneMsg:
		m = m.clearPending()
		m.tickPaused = false
		if msg.err != nil {
			m.lastErr = msg.err
		}
		return m, tea.Batch(tea.ClearScreen, m.refreshCmd(), m.scheduleTickCmd())
	case promptDoneMsg:
		return m.handlePrompt(msg)
	case tea.MouseClickMsg:
		return m.handleMouseClick(msg)
	case tea.MouseWheelMsg:
		return m.handleMouseWheel(msg)
	case tea.KeyPressMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

// View renders the full UI.
func (m Model) View() tea.View {
	var content string
	switch {
	case m.mode == modeShell:
		// Empty view suppresses the pre-exec frame leak into the TTY.
		content = ""
	case m.width == 0:
		content = "initialising vessel..."
	case m.width < 60 || m.height < 12:
		content = m.st.errorText.Render("terminal too small — resize to at least 60×12")
	case m.showHelp:
		content = m.helpView()
	case m.mode == modeLogs:
		content = m.logPanel.View()
	default:
		content = m.browseView()
	}

	v := tea.NewView(content)
	v.AltScreen = true
	if m.cfg.MouseEnabled && m.mode != modeShell {
		v.MouseMode = tea.MouseModeCellMotion
	}
	return v
}

func (m Model) browseView() string {
	sidebarW, detailW, listW, bodyH, cmdH := m.layoutDims()
	sidebar := m.sidebarView(sidebarW, bodyH)
	list, detail := m.mainPanels(listW, detailW, bodyH)
	body := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, list, detail)
	parts := []string{m.headerView(), body}
	if m.showCmdLog && cmdH > 0 {
		parts = append(parts, m.cmdLogView(cmdH))
	}
	parts = append(parts, m.footerView())
	content := lipgloss.JoinVertical(lipgloss.Left, parts...)
	if m.mode == modeConfirmDelete {
		content = m.overlayModal(content, m.confirmModal())
	}
	if m.mode == modeActions {
		content = m.overlayModal(content, m.actionsModal())
	}
	if m.mode == modePrompt {
		content = m.overlayModal(content, m.promptModal())
	}
	return content
}

func (m Model) layoutDims() (sidebarW, detailW, listW, bodyH, cmdH int) {
	sidebarW = 18
	cmdH = 0
	if m.showCmdLog {
		cmdH = 4
	}
	bodyH = max(1, m.height-2-cmdH)
	switch m.layout {
	case LayoutWideList:
		detailW = min(28, m.width/5)
	case LayoutLogsEmphasis:
		detailW = min(50, m.width/2)
	default:
		detailW = min(42, m.width/3)
	}
	listW = max(20, m.width-sidebarW-detailW)
	return sidebarW, detailW, listW, bodyH, cmdH
}

func (m Model) mainPanels(listW, detailW, height int) (string, string) {
	listBorder := m.paneStyle(m.focus == FocusList, listW, height)
	detailBorder := m.paneStyle(m.focus == FocusDetail, detailW, height)
	var list, detail string
	switch m.activeView {
	case ViewImages:
		list = m.imgPanel.ListView(listW-2, height-2)
		detail = m.imgPanel.DetailView(detailW-2, height-2)
	case ViewVolumes:
		list = m.volPanel.ListView(listW-2, height-2)
		detail = m.volPanel.DetailView(detailW-2, height-2)
	default:
		list = m.cntPanel.ListView(listW-2, height-2, m.poller)
		detail = m.cntPanel.DetailView(detailW-2, height-2, m.poller)
	}
	return listBorder.Render(list), detailBorder.Render(detail)
}

func (m Model) paneStyle(focused bool, w, h int) lipgloss.Style {
	fg := colorBorder
	if focused {
		fg = colorPurple
	}
	return lipgloss.NewStyle().
		Width(w).Height(h).
		Border(lipgloss.NormalBorder()).
		BorderForeground(fg)
}

func (m Model) startPollerCmd(ctx context.Context) tea.Cmd {
	return func() tea.Msg {
		go m.poller.Run(ctx)
		return tickMsg(time.Now())
	}
}

func (m Model) scheduleTickCmd() tea.Cmd {
	return tea.Tick(m.cfg.PollInterval.Duration, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m Model) refreshCmd() tea.Cmd {
	if m.client == nil {
		return nil
	}
	if m.activeView == ViewContainers {
		return m.loadContainersCmd()
	}
	return tea.Batch(m.loadContainersCmd(), m.activeViewLoadCmd())
}

func (m Model) activeViewLoadCmd() tea.Cmd {
	if m.client == nil {
		return nil
	}
	switch m.activeView {
	case ViewImages:
		return m.loadImagesCmd()
	case ViewVolumes:
		return m.loadVolumesCmd()
	default:
		return m.loadContainersCmd()
	}
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
	return m, nil
}

func (m Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	k := msg.String()

	if k == "ctrl+c" {
		return m, tea.Quit
	}
	if m.mode == modeShell {
		return m, nil
	}

	if m.mode == modePrompt {
		return m.handlePromptKey(k)
	}
	if m.mode == modeActions {
		return m.handleActionsKey(k)
	}
	if m.mode == modeConfirmDelete {
		switch k {
		case "y":
			return m.confirmPending()
		case "n", "esc":
			m = m.clearPending()
			m.status = "cancelled"
			return m, nil
		}
		return m, nil
	}

	if m.mode == modeLogs {
		if k == "esc" || k == "q" {
			if m.logCancel != nil {
				m.logCancel()
				m.logCancel = nil
			}
			m.logCh = nil
			m.logID = ""
			m.mode = modeBrowse
			return m, nil
		}
		if Match(k, m.keys.Yank) {
			if err := CopyToClipboard(m.logPanel.SelectedLine()); err != nil {
				m.lastErr = err
			} else {
				m.status = "copied log line"
			}
			return m, nil
		}
		var cmd tea.Cmd
		m.logPanel, cmd = m.logPanel.Update(msg)
		return m, cmd
	}

	if m.panelFiltering() {
		return m.routeToPanel(msg)
	}

	switch {
	case Match(k, m.keys.Quit):
		return m, tea.Quit
	case Match(k, m.keys.Help):
		m.showHelp = !m.showHelp
		return m, nil
	case Match(k, m.keys.Escape):
		if m.showHelp {
			m.showHelp = false
			return m, nil
		}
	case Match(k, m.keys.Tab):
		m.activeView = (m.activeView + 1) % 3
		return m, m.activeViewLoadCmd()
	case k == "1":
		m.activeView = ViewContainers
		return m, m.activeViewLoadCmd()
	case k == "2":
		m.activeView = ViewImages
		return m, m.activeViewLoadCmd()
	case k == "3":
		m.activeView = ViewVolumes
		return m, m.activeViewLoadCmd()
	case Match(k, m.keys.LayoutNext):
		m.layout = (m.layout + 1) % 3
		m.status = "layout " + m.layout.String()
		return m, nil
	case Match(k, m.keys.LayoutPrev):
		m.layout = (m.layout + 2) % 3
		m.status = "layout " + m.layout.String()
		return m, nil
	case k == "`":
		m.showCmdLog = !m.showCmdLog
		return m, nil
	case Match(k, m.keys.FocusNext, m.keys.Right):
		m.focus = (m.focus + 1) % 3
		return m, nil
	case Match(k, m.keys.FocusPrev, m.keys.Left):
		m.focus = (m.focus + 2) % 3
		return m, nil
	}

	if m.showHelp {
		return m, nil
	}

	if m.focus == FocusSidebar {
		switch {
		case m.keys.NavDown(k):
			m.activeView = (m.activeView + 1) % 3
			return m, m.activeViewLoadCmd()
		case m.keys.NavUp(k):
			m.activeView = (m.activeView + 2) % 3
			return m, m.activeViewLoadCmd()
		case Match(k, m.keys.Enter):
			m.focus = FocusList
			return m, nil
		}
		return m, nil
	}

	if Match(k, m.keys.Actions) {
		return m.openActions()
	}
	if Match(k, m.keys.Yank) {
		return m.yankSelected()
	}

	// View-specific actions (work from list or detail focus).
	switch m.activeView {
	case ViewContainers:
		switch {
		case Match(k, m.keys.Stop):
			return m.runOnSelected("stop", func(ctx context.Context, id string) error {
				return m.client.StopContainer(ctx, id)
			})
		case Match(k, m.keys.Start):
			return m.runOnSelected("start", func(ctx context.Context, id string) error {
				return m.client.StartContainer(ctx, id)
			})
		case Match(k, m.keys.Restart):
			return m.runOnSelected("restart", func(ctx context.Context, id string) error {
				return m.client.RestartContainer(ctx, id)
			})
		case Match(k, m.keys.Remove):
			return m.beginDeleteContainer()
		case Match(k, m.keys.Logs):
			return m.openLogs()
		case Match(k, m.keys.Enter) && m.focus != FocusDetail:
			return m.openShell()
		case Match(k, m.keys.Prune):
			return m.runGlobal("prune stopped", func(ctx context.Context) error {
				return m.client.PruneContainers(ctx)
			})
		case Match(k, m.keys.Create):
			return m.beginPrompt("run", "image to run")
		}
	case ViewImages:
		switch {
		case Match(k, m.keys.Remove):
			return m.beginDeleteImages()
		case Match(k, m.keys.Pull):
			return m.beginPrompt("pull", "image to pull")
		case Match(k, m.keys.Prune):
			return m.runGlobal("prune images", func(ctx context.Context) error {
				return m.client.PruneImages(ctx)
			})
		case Match(k, m.keys.Create):
			sel := m.imgPanel.Selected()
			if sel == nil {
				return m.beginPrompt("run", "image to run")
			}
			ref := backend.FormatRef(*sel)
			return m.runGlobal("run "+ref, func(ctx context.Context) error {
				return m.client.RunDetached(ctx, ref)
			})
		}
	case ViewVolumes:
		switch {
		case Match(k, m.keys.Remove):
			return m.beginDeleteVolumes()
		case Match(k, m.keys.Create):
			return m.beginPrompt("volcreate", "volume name")
		case Match(k, m.keys.Prune):
			return m.runGlobal("prune volumes", func(ctx context.Context) error {
				return m.client.PruneVolumes(ctx)
			})
		}
	}

	if m.focus == FocusDetail {
		return m, nil
	}
	return m.routeToPanel(msg)
}

func (m Model) panelFiltering() bool {
	switch m.activeView {
	case ViewImages:
		return m.imgPanel.Filtering()
	case ViewVolumes:
		return m.volPanel.Filtering()
	default:
		return m.cntPanel.Filtering()
	}
}

func (m Model) routeToPanel(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch m.activeView {
	case ViewImages:
		m.imgPanel, cmd = m.imgPanel.Update(msg)
	case ViewVolumes:
		m.volPanel, cmd = m.volPanel.Update(msg)
	default:
		m.cntPanel, cmd = m.cntPanel.Update(msg)
	}
	return m, cmd
}

func (m Model) handleMouseClick(msg tea.MouseClickMsg) (tea.Model, tea.Cmd) {
	if !m.cfg.MouseEnabled || m.mode != modeBrowse {
		return m, nil
	}
	ev := msg.Mouse()
	// Rough mapping: rows below header (~1) into list region.
	row := ev.Y - 2
	if row < 0 {
		return m, nil
	}
	m.focus = FocusList
	switch m.activeView {
	case ViewImages:
		m.imgPanel = m.imgPanel.SetCursor(row)
	case ViewVolumes:
		m.volPanel = m.volPanel.SetCursor(row)
	default:
		m.cntPanel = m.cntPanel.SetCursor(row)
	}
	return m, nil
}

func (m Model) handleMouseWheel(msg tea.MouseWheelMsg) (tea.Model, tea.Cmd) {
	if !m.cfg.MouseEnabled || m.mode != modeBrowse {
		return m, nil
	}
	delta := 1
	if msg.Mouse().Button == tea.MouseWheelUp {
		delta = -1
	}
	m.focus = FocusList
	switch m.activeView {
	case ViewImages:
		m.imgPanel = m.imgPanel.MoveBy(delta)
	case ViewVolumes:
		m.volPanel = m.volPanel.MoveBy(delta)
	default:
		m.cntPanel = m.cntPanel.MoveBy(delta)
	}
	return m, nil
}

// beginDelete stages a confirmation for ids of the given kind. Every delete
// path funnels through here so a target is always paired with the panel that
// owns it. Marks are not touched: the panels' SetItems prunes them on the next
// refresh, so a delete that fails stays retryable.
func (m Model) beginDelete(kind deleteKind, ids []string, label string) (tea.Model, tea.Cmd) {
	if len(ids) == 0 {
		return m, nil
	}
	m = m.beginConfirm("Delete", label, nil)
	m.pendingKind = kind
	m.pendingIDs = ids
	return m, nil
}

func (m Model) beginDeleteContainer() (tea.Model, tea.Cmd) {
	if marked := m.cntPanel.MarkedIDs(); len(marked) > 1 {
		return m.beginDelete(deleteContainers, marked, fmt.Sprintf("%d containers", len(marked)))
	}
	sel := m.cntPanel.Selected()
	if sel == nil {
		m.status = "nothing selected"
		return m, nil
	}
	return m.beginDelete(deleteContainers, []string{sel.ID}, sel.Name)
}

func (m Model) beginDeleteImages() (tea.Model, tea.Cmd) {
	if marked := m.imgPanel.MarkedIDs(); len(marked) > 1 {
		return m.beginDelete(deleteImages, marked, fmt.Sprintf("%d images", len(marked)))
	}
	sel := m.imgPanel.Selected()
	if sel == nil {
		return m, nil
	}
	return m.beginDelete(deleteImages, []string{sel.ID}, backend.FormatRef(*sel))
}

func (m Model) beginDeleteVolumes() (tea.Model, tea.Cmd) {
	if marked := m.volPanel.MarkedIDs(); len(marked) > 1 {
		return m.beginDelete(deleteVolumes, marked, fmt.Sprintf("%d volumes", len(marked)))
	}
	sel := m.volPanel.Selected()
	if sel == nil {
		return m, nil
	}
	return m.beginDelete(deleteVolumes, []string{sel.Name}, sel.Name)
}

// beginConfirm parks an action behind the confirm modal, labelled with the
// concrete target it will act on. Delete stages its targets as pendingIDs; the
// image actions carry more than one value, so they hand over a closure. Every
// confirmation opens through here, so a closure can never outlive its modal and
// be picked up by the next one.
func (m Model) beginConfirm(verb, label string, act func(Model) (Model, tea.Cmd)) Model {
	m = m.clearPending()
	m.mode = modeConfirmDelete
	m.pendingLbl = label
	m.pendingVerb = verb
	m.pendingAct = act
	return m
}

func (m Model) clearPending() Model {
	m.mode = modeBrowse
	m.pendingIDs = nil
	m.pendingLbl = ""
	m.pendingVerb = ""
	m.pendingAct = nil
	return m
}

// confirmPending runs whatever the confirm modal is currently holding. Delete
// is the common case; push and an overwriting save are routed here too because
// publishing an image or truncating a local file is as unrecoverable.
func (m Model) confirmPending() (tea.Model, tea.Cmd) {
	if act := m.pendingAct; act != nil {
		return act(m.clearPending())
	}
	return m.confirmDelete()
}

func (m Model) confirmDelete() (tea.Model, tea.Cmd) {
	kind, ids := m.pendingKind, m.pendingIDs
	client := m.client
	m = m.clearPending()
	if len(ids) == 0 {
		return m, nil
	}
	return m, func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		var err error
		switch kind {
		case deleteImages:
			err = client.RemoveImage(ctx, ids...)
		case deleteVolumes:
			err = client.RemoveVolume(ctx, ids...)
		case deleteContainers:
			for _, id := range ids {
				if e := client.RemoveContainer(ctx, id); e != nil {
					err = e
					break
				}
			}
		default:
			return actionDoneMsg{err: fmt.Errorf("delete: unhandled target kind %d", kind)}
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

func (m Model) runGlobal(label string, fn func(context.Context) error) (tea.Model, tea.Cmd) {
	return m.runAction(label, "", fn)
}

// runPush is runGlobal for the one verb whose failures may carry registry
// credential advice; ref is the image that advice would be about.
func (m Model) runPush(label, ref string, fn func(context.Context) error) (tea.Model, tea.Cmd) {
	return m.runAction(label, ref, fn)
}

func (m Model) runAction(label, pushRef string, fn func(context.Context) error) (tea.Model, tea.Cmd) {
	m.status = label + "…"
	return m, func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()
		if err := fn(ctx); err != nil {
			return actionDoneMsg{err: err, pushRef: pushRef}
		}
		return actionDoneMsg{msg: label + " ok"}
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
		return logsOpenedMsg{id: id, title: name, lines: lines, err: err}
	}
}

func (m Model) streamLogsCmd(ctx context.Context, id string, ch chan backend.LogLine) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		if err := client.StreamLogs(ctx, id, ch); err != nil {
			return logErrMsg{err: err}
		}
		return nil
	}
}

func (m Model) waitLogLineCmd() tea.Cmd {
	ch := m.logCh
	if ch == nil {
		return nil
	}
	return func() tea.Msg {
		line, ok := <-ch
		if !ok {
			return logStreamEndMsg{}
		}
		return logLineMsg{id: line.ContainerID, text: line.Text}
	}
}

func (m Model) openShell() (tea.Model, tea.Cmd) {
	sel := m.cntPanel.Selected()
	if sel == nil {
		m.status = "nothing selected"
		return m, nil
	}
	if !sel.IsRunning() {
		m.status = "shell disabled: container is not running"
		return m, nil
	}
	m.mode = modeShell
	m.tickPaused = true
	cmd := m.client.ShellCmd(sel.ID, m.cfg.Shell)
	return m, tea.ExecProcess(cmd, func(err error) tea.Msg {
		if err != nil {
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				return shellDoneMsg{}
			}
			return shellDoneMsg{err: err}
		}
		return shellDoneMsg{}
	})
}

func (m Model) yankSelected() (tea.Model, tea.Cmd) {
	var text string
	switch m.activeView {
	case ViewImages:
		if sel := m.imgPanel.Selected(); sel != nil {
			text = backend.FormatRef(*sel)
		}
	case ViewVolumes:
		if sel := m.volPanel.Selected(); sel != nil {
			text = sel.Mountpoint
			if text == "" {
				text = sel.Name
			}
		}
	default:
		if sel := m.cntPanel.Selected(); sel != nil {
			text = sel.ID
		}
	}
	if err := CopyToClipboard(text); err != nil {
		m.lastErr = err
	} else {
		m.status = "copied " + text
	}
	return m, nil
}

func (m Model) beginPrompt(kind, label string) (tea.Model, tea.Cmd) {
	m.mode = modePrompt
	m.promptKind = kind
	m.promptLabel = label
	m.promptBuf = ""
	return m, nil
}

// beginPromptForImage opens a prompt that remembers a selected image ref so the
// submitted text can be combined with it (e.g. a live image + a new tag). The
// label names both halves, since the field wants the other one.
func (m Model) beginPromptForImage(kind, label, ref string) (tea.Model, tea.Cmd) {
	m.mode = modePrompt
	m.promptKind = kind
	m.promptLabel = label
	m.promptBuf = ""
	m.promptRef = ref
	return m, nil
}

func (m Model) handlePromptKey(k string) (tea.Model, tea.Cmd) {
	switch k {
	case "esc":
		m.mode = modeBrowse
		m.promptBuf = ""
		m.promptRef = ""
		m.promptLabel = ""
		m.status = "cancelled"
		return m, nil
	case "enter":
		kind := m.promptKind
		text := strings.TrimSpace(m.promptBuf)
		m.mode = modeBrowse
		m.promptBuf = ""
		return m.handlePrompt(promptDoneMsg{kind: kind, text: text})
	case "backspace":
		if len(m.promptBuf) > 0 {
			m.promptBuf = m.promptBuf[:len(m.promptBuf)-1]
		}
		return m, nil
	default:
		if len(k) == 1 {
			m.promptBuf += k
		}
		return m, nil
	}
}

func (m Model) handlePrompt(msg promptDoneMsg) (tea.Model, tea.Cmd) {
	ref := m.promptRef
	m.promptRef = ""
	if msg.text == "" {
		m.status = "empty input"
		return m, nil
	}
	switch msg.kind {
	case "pull":
		ref := msg.text
		return m.runGlobal("pull "+ref, func(ctx context.Context) error {
			return m.client.PullImage(ctx, ref)
		})
	case "run":
		ref := msg.text
		return m.runGlobal("run "+ref, func(ctx context.Context) error {
			return m.client.RunDetached(ctx, ref)
		})
	case "volcreate":
		name := msg.text
		return m.runGlobal("create volume "+name, func(ctx context.Context) error {
			return m.client.CreateVolume(ctx, name)
		})
	case "tag":
		newRef := msg.text
		return m.runGlobal("tag "+newRef, func(ctx context.Context) error {
			return m.client.TagImage(ctx, ref, newRef)
		})
	case "save to":
		imageRef, path := ref, msg.text
		save := func(m Model) (Model, tea.Cmd) {
			x, c := m.runGlobal("save "+imageRef+" → "+path, func(ctx context.Context) error {
				return m.client.SaveImage(ctx, imageRef, path)
			})
			return x.(Model), c
		}
		if backend.FileExists(path) {
			return m.beginConfirm("Overwrite", path+" with "+imageRef, save), nil
		}
		return save(m)
	case "load from":
		path := msg.text
		return m.runGlobal("load "+path, func(ctx context.Context) error {
			return m.client.LoadImage(ctx, path)
		})
	case "custom":
		return m.runCustom(msg.text)
	}
	return m, nil
}

func (m Model) openActions() (tea.Model, tea.Cmd) {
	items := m.buildActions()
	if len(items) == 0 {
		m.status = "no actions"
		return m, nil
	}
	m.mode = modeActions
	m.actionItems = items
	m.actionIdx = 0
	return m, nil
}

// imageActionRef resolves the highlighted row to a reference safe to act on.
// Tag, Save and Push all reach outside the process, so a row that formats to an
// ambiguous reference is refused rather than silently resolved to ":latest".
func (m Model) imageActionRef() (Model, string, bool) {
	sel := m.imgPanel.Selected()
	if sel == nil {
		m.status = "nothing selected"
		return m, "", false
	}
	ref, ok := backend.ExactRef(*sel)
	if !ok {
		m.status = "digest-pinned image has no named reference — tag, save and push cannot address it"
		return m, "", false
	}
	return m, ref, true
}

func (m Model) buildActions() []actionItem {
	var items []actionItem
	switch m.activeView {
	case ViewContainers:
		items = append(items,
			actionItem{"Start", func(m Model) (Model, tea.Cmd) {
				x, c := m.runOnSelected("start", func(ctx context.Context, id string) error {
					return m.client.StartContainer(ctx, id)
				})
				return x.(Model), c
			}},
			actionItem{"Stop", func(m Model) (Model, tea.Cmd) {
				x, c := m.runOnSelected("stop", func(ctx context.Context, id string) error {
					return m.client.StopContainer(ctx, id)
				})
				return x.(Model), c
			}},
			actionItem{"Restart", func(m Model) (Model, tea.Cmd) {
				x, c := m.runOnSelected("restart", func(ctx context.Context, id string) error {
					return m.client.RestartContainer(ctx, id)
				})
				return x.(Model), c
			}},
			actionItem{"Logs", func(m Model) (Model, tea.Cmd) {
				x, c := m.openLogs()
				return x.(Model), c
			}},
			actionItem{"Shell", func(m Model) (Model, tea.Cmd) {
				x, c := m.openShell()
				return x.(Model), c
			}},
			actionItem{"Prune stopped", func(m Model) (Model, tea.Cmd) {
				x, c := m.runGlobal("prune stopped", func(ctx context.Context) error {
					return m.client.PruneContainers(ctx)
				})
				return x.(Model), c
			}},
		)
	case ViewImages:
		items = append(items,
			actionItem{"Pull…", func(m Model) (Model, tea.Cmd) {
				x, c := m.beginPrompt("pull", "image to pull")
				return x.(Model), c
			}},
			actionItem{"Run", func(m Model) (Model, tea.Cmd) {
				sel := m.imgPanel.Selected()
				if sel == nil {
					return m, nil
				}
				ref := backend.FormatRef(*sel)
				x, c := m.runGlobal("run "+ref, func(ctx context.Context) error {
					return m.client.RunDetached(ctx, ref)
				})
				return x.(Model), c
			}},
			actionItem{"Tag…", func(m Model) (Model, tea.Cmd) {
				m, ref, ok := m.imageActionRef()
				if !ok {
					return m, nil
				}
				x, c := m.beginPromptForImage("tag", "tag "+ref+" as (new reference)", ref)
				return x.(Model), c
			}},
			actionItem{"Save…", func(m Model) (Model, tea.Cmd) {
				m, ref, ok := m.imageActionRef()
				if !ok {
					return m, nil
				}
				x, c := m.beginPromptForImage("save to", "save "+ref+" to (path)", ref)
				return x.(Model), c
			}},
			actionItem{"Load…", func(m Model) (Model, tea.Cmd) {
				x, c := m.beginPrompt("load from", "load from (tar archive path)")
				return x.(Model), c
			}},
			actionItem{"Push", func(m Model) (Model, tea.Cmd) {
				m, ref, ok := m.imageActionRef()
				if !ok {
					return m, nil
				}
				label := ref
				if dest, ok := backend.PushTarget(ref); ok {
					label = ref + " → " + dest
				}
				return m.beginConfirm("Push", label, func(m Model) (Model, tea.Cmd) {
					x, c := m.runPush("push "+ref, ref, func(ctx context.Context) error {
						return m.client.PushImage(ctx, ref)
					})
					return x.(Model), c
				}), nil
			}},
			actionItem{"Prune unused", func(m Model) (Model, tea.Cmd) {
				x, c := m.runGlobal("prune images", func(ctx context.Context) error {
					return m.client.PruneImages(ctx)
				})
				return x.(Model), c
			}},
		)
	case ViewVolumes:
		items = append(items,
			actionItem{"Create…", func(m Model) (Model, tea.Cmd) {
				x, c := m.beginPrompt("volcreate", "volume name")
				return x.(Model), c
			}},
			actionItem{"Prune unused", func(m Model) (Model, tea.Cmd) {
				x, c := m.runGlobal("prune volumes", func(ctx context.Context) error {
					return m.client.PruneVolumes(ctx)
				})
				return x.(Model), c
			}},
		)
	}
	for _, cc := range m.cfg.CustomCommands {
		cc := cc
		items = append(items, actionItem{
			label: "custom: " + cc.Name,
			run: func(m Model) (Model, tea.Cmd) {
				return m.runCustom(cc.Command)
			},
		})
	}
	return items
}

func (m Model) handleActionsKey(k string) (tea.Model, tea.Cmd) {
	switch {
	case k == "esc", Match(k, m.keys.Actions):
		m.mode = modeBrowse
		return m, nil
	case m.keys.NavDown(k):
		if m.actionIdx < len(m.actionItems)-1 {
			m.actionIdx++
		}
		return m, nil
	case m.keys.NavUp(k):
		if m.actionIdx > 0 {
			m.actionIdx--
		}
		return m, nil
	case Match(k, m.keys.Enter):
		item := m.actionItems[m.actionIdx]
		m.mode = modeBrowse
		return item.run(m)
	}
	return m, nil
}

func (m Model) runCustom(tmpl string) (Model, tea.Cmd) {
	id, name, image := "", "", ""
	switch m.activeView {
	case ViewImages:
		if sel := m.imgPanel.Selected(); sel != nil {
			id, name, image = sel.ID, backend.FormatRef(*sel), backend.FormatRef(*sel)
		}
	case ViewVolumes:
		if sel := m.volPanel.Selected(); sel != nil {
			id, name = sel.Name, sel.Name
		}
	default:
		if sel := m.cntPanel.Selected(); sel != nil {
			id, name, image = sel.ID, sel.Name, sel.Image
		}
	}
	cmdStr := tmpl
	cmdStr = strings.ReplaceAll(cmdStr, "{{.ID}}", id)
	cmdStr = strings.ReplaceAll(cmdStr, "{{.Name}}", name)
	cmdStr = strings.ReplaceAll(cmdStr, "{{.Image}}", image)
	m.status = "custom: " + cmdStr
	return m, func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		c := exec.CommandContext(ctx, "bash", "-lc", cmdStr)
		out, err := c.CombinedOutput()
		if err != nil {
			return actionDoneMsg{err: fmt.Errorf("%w: %s", err, string(out))}
		}
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = "custom ok"
		}
		return actionDoneMsg{msg: uiutilTruncate(msg, 80)}
	}
}

func uiutilTruncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

func (m Model) headerView() string {
	title := m.st.title.Render("⬡ vessel")
	meta := m.st.dimText.Render(fmt.Sprintf("[%s] focus:%s layout:%s", m.viewName(), m.focus.String(), m.layout.String()))
	hint := m.st.dimText.Render("[ ? help ]  [ q quit ]")
	spacer := lipgloss.NewStyle().
		Width(max(0, m.width-lipgloss.Width(title)-lipgloss.Width(meta)-lipgloss.Width(hint)-2)).
		Render("")
	return lipgloss.JoinHorizontal(lipgloss.Top, title, "  ", meta, spacer, hint)
}

func (m Model) viewName() string {
	switch m.activeView {
	case ViewImages:
		return "images"
	case ViewVolumes:
		return "volumes"
	default:
		return "containers"
	}
}

func (m Model) footerView() string {
	if m.lastErr != nil {
		return m.st.errorText.Width(m.width).Render(m.clampToRow("error: " + m.lastErr.Error()))
	}
	if m.status != "" {
		return m.st.footerHelp.Width(m.width).Render(m.clampToRow(m.status))
	}
	cur, n := m.cursorInfo()
	prefix := fmt.Sprintf("%d/%d  ", cur+1, n)
	if n == 0 {
		prefix = "0/0  "
	}
	var keys string
	switch m.activeView {
	case ViewImages:
		keys = "[p] pull  [c] run  [d] delete  [P] prune  [/] filter  [x] actions  [y] yank"
	case ViewVolumes:
		keys = "[c] create  [d] delete  [P] prune  [/] filter  [x] actions  [y] yank"
	default:
		keys = "[enter] shell  [L] logs  [s/u/r] lifecycle  [d] remove  [/] filter  [x] actions  [y] yank"
	}
	return m.st.footerHelp.Width(m.width).Render(prefix + keys)
}

// clampToRow flattens s onto one row no wider than the frame. It guards the two
// footer branches that render unbounded text — CLI errors arrive with embedded
// newlines and hundreds of characters of stderr — and deliberately not the key
// hints, whose grouping is authored to be read as-is. Measurement is by display
// cell, matching what lipgloss wraps on: a rune count lets wide glyphs through
// and still overflows.
func (m Model) clampToRow(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if m.width <= 0 {
		return s
	}
	return ansi.Truncate(s, m.width, "…")
}

func (m Model) cursorInfo() (int, int) {
	switch m.activeView {
	case ViewImages:
		return m.imgPanel.Cursor(), m.imgPanel.Len()
	case ViewVolumes:
		return m.volPanel.Cursor(), m.volPanel.Len()
	default:
		return m.cntPanel.Cursor(), m.cntPanel.Len()
	}
}

func (m Model) sidebarView(width, height int) string {
	views := []struct {
		label string
		view  View
	}{
		{"Containers", ViewContainers},
		{"Images", ViewImages},
		{"Volumes", ViewVolumes},
	}

	focused := m.focus == FocusSidebar
	var rows []string
	rows = append(rows, m.st.sectionTitle.Width(width-2).Render("Views"))
	for _, v := range views {
		st := m.st.navItem
		if m.activeView == v.view {
			st = m.st.navItemActive
		}
		label := fmt.Sprintf("%d %s", int(v.view)+1, v.label)
		rows = append(rows, st.Width(width-2).Render(label))
	}
	rows = append(rows, "")
	rows = append(rows, m.st.sectionTitle.Width(width-2).Render("Fleet"))
	rows = append(rows, m.st.statRunning.Render(
		fmt.Sprintf("● %d running", m.cntPanel.RunningCount())))
	rows = append(rows, m.st.statStopped.Render(
		fmt.Sprintf("○ %d stopped", m.cntPanel.StoppedCount())))

	inner := lipgloss.JoinVertical(lipgloss.Left, rows...)
	return m.paneStyle(focused, width, height).Render(inner)
}

func (m Model) cmdLogView(height int) string {
	lines := []string{m.st.sectionTitle.Render("command log  [`] toggle")}
	if m.client != nil {
		log := m.client.CommandLog()
		start := max(0, len(log)-(height-1))
		for _, l := range log[start:] {
			lines = append(lines, m.st.dimText.Render("  "+l))
		}
	}
	return lipgloss.NewStyle().Width(m.width).Height(height).
		BorderTop(true).BorderStyle(lipgloss.NormalBorder()).BorderForeground(colorBorder).
		Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}

func (m Model) helpView() string {
	var rows []string
	rows = append(rows, m.st.title.Render("vessel — keybindings"))
	rows = append(rows, m.st.dimText.Render(fmt.Sprintf("view=%s focus=%s", m.viewName(), m.focus.String())))
	rows = append(rows, "")
	for _, b := range helpBindings(m.activeView, m.focus, m.mode) {
		key := m.st.helpText.Width(22).Render(b.key)
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, key, b.desc))
	}
	rows = append(rows, "")
	rows = append(rows, m.st.dimText.Render("press ? or esc to close"))
	return lipgloss.NewStyle().
		Width(m.width).Height(m.height).
		Align(lipgloss.Center, lipgloss.Center).
		Render(lipgloss.JoinVertical(lipgloss.Left, rows...))
}

func (m Model) confirmModal() string {
	label := m.pendingLbl
	if label == "" {
		label = strings.Join(m.pendingIDs, ", ")
	}
	verb := m.pendingVerb
	if verb == "" {
		verb = "Delete"
	}
	body := fmt.Sprintf("%s %s?\n\n[y] confirm   [n/esc] cancel", verb, label)
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorRed).
		Padding(1, 2).
		Width(min(48, m.width-4)).
		Render(body)
}

// actionsModalChrome is what the modal spends on border, padding, title, the
// blank rows and the hint — everything that is not a menu item.
const actionsModalChrome = 8

func (m Model) actionsModalWidth() int { return min(40, m.width-4) }

// actionsModalContent is the cells a row may occupy. lipgloss counts Width as
// the outer box, so the rounded border takes one cell on each side and
// Padding(1, 2) another two. Every row is truncated to what is left, so one item
// is always exactly one row — which is what lets actionWindow size the window by
// item count rather than by rendered rows.
func (m Model) actionsModalContent() int { return max(1, m.actionsModalWidth()-6) }

// actionWindow is the slice of the menu that fits the frame. lipgloss.Place pads
// but never truncates, so a menu taller than the terminal would push the header
// off the alt-screen; the window follows the selection instead.
func (m Model) actionWindow() (start, end int) {
	size := len(m.actionItems)
	if m.height > 0 {
		size = min(size, max(1, m.height-actionsModalChrome))
	}
	return uiutil.Window(len(m.actionItems), m.actionIdx, size)
}

func (m Model) actionsModal() string {
	content := m.actionsModalContent()
	fit := func(s string) string { return ansi.Truncate(s, content, "…") }
	rows := []string{m.st.title.Render(fit("actions")), ""}
	start, end := m.actionWindow()
	for i := start; i < end; i++ {
		label := m.actionItems[i].label
		if i == m.actionIdx {
			// navItemActive adds a cell of padding on each side.
			rows = append(rows, m.st.navItemActive.Render(
				ansi.Truncate("> "+label, max(1, content-2), "…")))
			continue
		}
		rows = append(rows, fit("  "+label))
	}
	hint := "[enter] run  [esc] close"
	if end-start < len(m.actionItems) {
		hint = fmt.Sprintf("%d/%d  ", m.actionIdx+1, len(m.actionItems)) + hint
	}
	rows = append(rows, "", m.st.dimText.Render(fit(hint)))
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorPurple).
		Padding(1, 2).
		Width(m.actionsModalWidth()).
		Render(lipgloss.JoinVertical(lipgloss.Left, rows...))
}

func (m Model) promptModal() string {
	title := m.promptLabel
	if title == "" {
		title = m.promptKind
	}
	body := fmt.Sprintf("%s: %s_", title, m.promptBuf)
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorBlue).
		Padding(1, 2).
		Width(min(56, m.width-4)).
		Render(body + "\n\n[enter] ok  [esc] cancel")
}

func (m Model) overlayModal(_, modal string) string {
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, modal)
}
