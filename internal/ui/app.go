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

// inspectSettledMsg fires once the images/volumes selection has stopped
// moving, so holding a cursor key spawns one inspect subprocess instead of one
// per step. It is keyed by selection rather than by a counter, so a repeated
// request for the selection already pending cannot supersede its own timer.
type inspectSettledMsg struct {
	key string
}

type imageInspectMsg struct {
	ref string
	ins *backend.ImageInspect
	err error
}

type volumeInspectMsg struct {
	name string
	ins  *backend.VolumeInspect
	err  error
}

type actionDoneMsg struct {
	err error
	msg string
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

// deleteKind names what a staged confirmation will do, so the confirmation
// carries its targets as plain ids instead of a prefix-encoded string. Prune and
// stop reuse the same modeConfirmDelete plumbing as their own kinds.
type deleteKind int

const (
	deleteContainers deleteKind = iota
	deleteImages
	deleteVolumes
	pruneContainers
	pruneImages
	pruneVolumes
	stopContainer
)

// isPrune reports whether kind is a prune, which stages no ids of its own. It
// derives from pruneSpecs so the two can never disagree about which kinds are
// prunes: a kind listed here without a spec of its own would sweep whatever
// store the fallback picked.
func (k deleteKind) isPrune() bool {
	_, ok := pruneSpecs[k]
	return ok
}

const (
	// lifecycleTimeout bounds a container lifecycle verb (stop, start, restart),
	// whether or not it went through the confirm modal.
	lifecycleTimeout = 30 * time.Second
	// confirmTimeout bounds a confirmed removal.
	confirmTimeout = 60 * time.Second
	// globalTimeout bounds whole-store verbs such as prune, which sweep every
	// container/image/volume and take far longer than a single removal.
	globalTimeout = 120 * time.Second
	// All three are outer bounds only: backend.Client re-wraps every CLI
	// invocation with its own 10s timeout and the earlier deadline wins, so
	// today it is that one, not these, that actually fires.
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
	helpScroll int
	showCmdLog bool

	cntPanel containers.Model
	imgPanel images.Model
	volPanel volumes.Model
	logPanel logs.Model

	lastErr     error
	status      string
	footerSeq   uint64
	statusGen   uint64
	errGen      uint64
	pendingKind deleteKind
	pendingIDs  []string
	pendingLbl  string
	logCancel   context.CancelFunc
	logCh       chan backend.LogLine
	logID       string
	pollCancel  context.CancelFunc
	tickPaused  bool

	inspectKey  string
	inspectRun  string
	actionIdx   int
	actionItems []actionItem
	promptKind  string
	promptBuf   string
}

type actionItem struct {
	label string
	run   func(Model) (Model, tea.Cmd)
}

// setStatus and setLastErr are the only writers of status and lastErr. Each
// stamps its field with the next value of a shared counter, which is what lets
// footerLine decide by recency in one place instead of every caller having to
// clear the other field to be seen.
func (m *Model) setStatus(s string) {
	m.footerSeq++
	m.statusGen = m.footerSeq
	m.status = s
}

func (m *Model) setLastErr(err error) {
	m.footerSeq++
	m.errGen = m.footerSeq
	m.lastErr = err
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
			m.setLastErr(msg.err)
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
			m.setLastErr(msg.err)
		} else {
			m.imgPanel = m.imgPanel.SetItems(msg.items)
		}
		return m.scheduleInspect()
	case volumesLoadedMsg:
		if msg.err != nil {
			m.setLastErr(msg.err)
		} else {
			m.volPanel = m.volPanel.SetItems(msg.items)
		}
		return m.scheduleInspect()
	case inspectSettledMsg:
		if msg.key == m.inspectKey {
			m.inspectKey = ""
		}
		if msg.key != m.selectionKey() || msg.key == m.inspectRun {
			return m, nil
		}
		var cmd tea.Cmd
		switch m.activeView {
		case ViewImages:
			cmd = m.loadImageInspectCmd()
		case ViewVolumes:
			cmd = m.loadVolumeInspectCmd()
		}
		if cmd != nil {
			m.inspectRun = msg.key
		}
		return m, cmd
	case imageInspectMsg:
		if m.inspectRun == imageKey(msg.ref) {
			m.inspectRun = ""
		}
		m.imgPanel = m.imgPanel.SetInspect(msg.ref, msg.ins, msg.err)
		return m, nil
	case volumeInspectMsg:
		if m.inspectRun == volumeKey(msg.name) {
			m.inspectRun = ""
		}
		m.volPanel = m.volPanel.SetInspect(msg.name, msg.ins, msg.err)
		return m, nil
	case actionDoneMsg:
		if msg.err != nil {
			m.setLastErr(msg.err)
			m.setStatus("")
		} else {
			m.setLastErr(nil)
			m.setStatus(msg.msg)
		}
		m.mode = modeBrowse
		m.pendingIDs = nil
		m.pendingLbl = ""
		return m, m.refreshCmd()
	case logsOpenedMsg:
		if msg.err != nil {
			m.setLastErr(msg.err)
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
		m.mode = modeBrowse
		m.tickPaused = false
		if msg.err != nil {
			m.setLastErr(msg.err)
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

// inspectDebounce is how long the selection must hold still before the pane is
// inspected.
const inspectDebounce = 120 * time.Millisecond

// selectionKey identifies what the active view would inspect right now. It is
// empty for views and selections that have nothing to inspect.
func (m Model) selectionKey() string {
	switch m.activeView {
	case ViewImages:
		// A dangling image has no reference, and the reference is the only
		// thing the CLI can be asked to inspect, so there is nothing to run.
		if sel := m.imgPanel.Selected(); sel != nil {
			if ref := backend.FormatRef(*sel); ref != "" {
				return imageKey(ref)
			}
		}
	case ViewVolumes:
		if sel := m.volPanel.Selected(); sel != nil {
			return volumeKey(sel.Name)
		}
	}
	return ""
}

func imageKey(ref string) string { return "image:" + ref }

func volumeKey(name string) string { return "volume:" + name }

// scheduleInspect coalesces inspect requests: a burst of cursor movement
// results in a single inspect of the selection the user settled on, and a
// request for a selection already awaiting its timer or already out at the
// CLI is a no-op rather than a fresh timer, so neither a fast poll interval
// nor a slow inspect can multiply the subprocesses for one selection.
func (m Model) scheduleInspect() (tea.Model, tea.Cmd) {
	key := m.selectionKey()
	if m.client == nil || key == "" || key == m.inspectKey || key == m.inspectRun {
		return m, nil
	}
	m.inspectKey = key
	return m, tea.Tick(inspectDebounce, func(time.Time) tea.Msg {
		return inspectSettledMsg{key: key}
	})
}

// loadImageInspectCmd inspects the currently selected image (if any) so the
// images detail pane can render the enriched fields.
func (m Model) loadImageInspectCmd() tea.Cmd {
	if m.client == nil {
		return nil
	}
	sel := m.imgPanel.Selected()
	if sel == nil {
		return nil
	}
	ref := backend.FormatRef(*sel)
	if ref == "" {
		return nil
	}
	// The panel already holds a successful inspect for this exact image, so
	// re-running the subprocess on every poll tick would only reproduce it.
	if m.imgPanel.InspectedRef() == ref {
		return nil
	}
	client := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		ins, err := client.ImageInspect(ctx, ref)
		return imageInspectMsg{ref: ref, ins: ins, err: err}
	}
}

// loadVolumeInspectCmd inspects the currently selected volume (if any) so the
// volumes detail pane can render size/format/labels/options.
func (m Model) loadVolumeInspectCmd() tea.Cmd {
	if m.client == nil {
		return nil
	}
	sel := m.volPanel.Selected()
	if sel == nil {
		return nil
	}
	name := sel.Name
	if m.volPanel.InspectedName() == name {
		return nil
	}
	client := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		ins, err := client.VolumeInspect(ctx, name)
		return volumeInspectMsg{name: name, ins: ins, err: err}
	}
}

func (m Model) applyContainersLoaded(msg containersLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.setLastErr(msg.err)
	} else {
		// `container list` is a top-level verb that keeps working while the
		// plugin-gated prune/create verbs report services-down, so a successful
		// poll is not evidence the services came back. Keep a services-down hint
		// visible until the user acts or a different failure replaces it, rather
		// than wiping it on the next refresh.
		if !backend.IsServicesDown(m.lastErr) {
			m.setLastErr(nil)
		}
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
			return m.confirmDelete()
		case "n", "esc":
			m.mode = modeBrowse
			m.pendingIDs = nil
			m.pendingLbl = ""
			m.setStatus("cancelled")
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
				m.setLastErr(err)
			} else {
				m.setStatus("copied log line")
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
		m.helpScroll = 0
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
		m.setStatus("layout " + m.layout.String())
		return m, nil
	case Match(k, m.keys.LayoutPrev):
		m.layout = (m.layout + 2) % 3
		m.setStatus("layout " + m.layout.String())
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
		page := helpVisibleRows(m.height)
		switch {
		case m.keys.NavDown(k):
			m.helpScroll = m.clampHelpScroll(m.helpScroll + 1)
		case m.keys.NavUp(k):
			m.helpScroll = m.clampHelpScroll(m.helpScroll - 1)
		case Match(k, m.keys.PageDown, m.keys.HalfDown):
			m.helpScroll = m.clampHelpScroll(m.helpScroll + page)
		case Match(k, m.keys.PageUp, m.keys.HalfUp):
			m.helpScroll = m.clampHelpScroll(m.helpScroll - page)
		case Match(k, m.keys.GotoTop):
			m.helpScroll = 0
		case Match(k, m.keys.GotoBottom):
			m.helpScroll = m.clampHelpScroll(len(m.helpBindings()))
		}
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

	// A custom command configured with this key takes precedence over the
	// built-in single-key actions below.
	if cmd := m.customCommandForKey(k); cmd != "" {
		return m.runCustom(cmd)
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
			return m.beginStop()
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
			return m.beginPrune(pruneContainers)
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
			return m.beginPrune(pruneImages)
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
			return m.beginPrune(pruneVolumes)
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
		before := m.imgPanel.Selected()
		m.imgPanel, cmd = m.imgPanel.Update(msg)
		if selectionRefChanged(before, m.imgPanel.Selected()) {
			next, icmd := m.scheduleInspect()
			return next, tea.Batch(cmd, icmd)
		}
	case ViewVolumes:
		before := m.volPanel.Selected()
		m.volPanel, cmd = m.volPanel.Update(msg)
		if selectionNameChanged(before, m.volPanel.Selected()) {
			next, icmd := m.scheduleInspect()
			return next, tea.Batch(cmd, icmd)
		}
	default:
		m.cntPanel, cmd = m.cntPanel.Update(msg)
	}
	return m, cmd
}

// selectionRefChanged reports whether the selected image changed, so the app
// can trigger a fresh inspect for the new selection.
func selectionRefChanged(before, after *backend.Image) bool {
	if before == nil || after == nil {
		return before != after
	}
	return backend.FormatRef(*before) != backend.FormatRef(*after)
}

// selectionNameChanged reports whether the selected volume changed.
func selectionNameChanged(before, after *backend.Volume) bool {
	if before == nil || after == nil {
		return before != after
	}
	return before.Name != after.Name
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
		before := m.imgPanel.Selected()
		m.imgPanel = m.imgPanel.SetCursor(row)
		if selectionRefChanged(before, m.imgPanel.Selected()) {
			return m.scheduleInspect()
		}
	case ViewVolumes:
		before := m.volPanel.Selected()
		m.volPanel = m.volPanel.SetCursor(row)
		if selectionNameChanged(before, m.volPanel.Selected()) {
			return m.scheduleInspect()
		}
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
		before := m.imgPanel.Selected()
		m.imgPanel = m.imgPanel.MoveBy(delta)
		if selectionRefChanged(before, m.imgPanel.Selected()) {
			return m.scheduleInspect()
		}
	case ViewVolumes:
		before := m.volPanel.Selected()
		m.volPanel = m.volPanel.MoveBy(delta)
		if selectionNameChanged(before, m.volPanel.Selected()) {
			return m.scheduleInspect()
		}
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
	m.mode = modeConfirmDelete
	m.pendingKind = kind
	m.pendingIDs = ids
	m.pendingLbl = label
	return m, nil
}

func (m Model) beginDeleteContainer() (tea.Model, tea.Cmd) {
	if marked := m.cntPanel.MarkedIDs(); len(marked) > 1 {
		return m.beginDelete(deleteContainers, marked, fmt.Sprintf("%d containers", len(marked)))
	}
	sel := m.cntPanel.Selected()
	if sel == nil {
		m.setStatus("nothing selected")
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

// beginPrune opens the confirm modal before a destructive prune. A prune targets
// whatever the CLI deems unused, so it stages a kind with no ids.
func (m Model) beginPrune(kind deleteKind) (tea.Model, tea.Cmd) {
	m.mode = modeConfirmDelete
	m.pendingKind = kind
	m.pendingIDs = nil
	m.pendingLbl = ""
	return m, nil
}

// beginStop stops the selected container, asking for confirmation first when
// the user opted into Config.ConfirmStop.
func (m Model) beginStop() (tea.Model, tea.Cmd) {
	sel := m.cntPanel.Selected()
	if sel == nil {
		m.setStatus("nothing selected")
		return m, nil
	}
	id, name := sel.ID, sel.Name
	if !m.cfg.ConfirmStop {
		return m.runOnSelected("stop", func(ctx context.Context, containerID string) error {
			return m.client.StopContainer(ctx, containerID)
		})
	}
	m.mode = modeConfirmDelete
	m.pendingKind = stopContainer
	m.pendingIDs = []string{id}
	m.pendingLbl = name
	return m, nil
}

// pruneSpec pairs the question asked about a prune target with the footer label
// and the call that performs it, so the modal can never ask about one store and
// sweep another.
type pruneSpec struct {
	label    string
	question string
	run      func(context.Context, *backend.Client) error
}

var pruneSpecs = map[deleteKind]pruneSpec{
	pruneContainers: {"prune containers", "Prune stopped containers?", func(ctx context.Context, c *backend.Client) error {
		return c.PruneContainers(ctx)
	}},
	pruneImages: {"prune images", "Prune unused images?", func(ctx context.Context, c *backend.Client) error {
		return c.PruneImages(ctx)
	}},
	pruneVolumes: {"prune volumes", "Prune unused volumes?", func(ctx context.Context, c *backend.Client) error {
		return c.PruneVolumes(ctx)
	}},
}

func pruneSpecFor(kind deleteKind) (pruneSpec, bool) {
	spec, ok := pruneSpecs[kind]
	return spec, ok
}

// pendingAction describes a confirmed action: the footer label shown while it
// runs, the message reported on success, and its time budget. A prune sweeps a
// whole container/image/volume store, so it keeps the longer budget it had
// before it was routed through the confirm modal; the other paths remove a
// single resource.
func pendingAction(kind deleteKind) (label, done string, timeout time.Duration) {
	if spec, ok := pruneSpecFor(kind); ok {
		return spec.label, "pruned", globalTimeout
	}
	switch kind {
	case stopContainer:
		return "stop", "stopped", lifecycleTimeout
	default:
		return "delete", "deleted", confirmTimeout
	}
}

func (m Model) confirmDelete() (tea.Model, tea.Cmd) {
	kind, ids := m.pendingKind, m.pendingIDs
	client := m.client
	label, done, timeout := pendingAction(kind)
	m.mode = modeBrowse
	m.pendingIDs = nil
	m.pendingLbl = ""
	if len(ids) == 0 && !kind.isPrune() {
		return m, nil
	}
	m.setStatus(label + "…")
	return m, func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		var err error
		if spec, ok := pruneSpecFor(kind); ok {
			err = spec.run(ctx, client)
		} else {
			switch kind {
			case stopContainer:
				err = client.StopContainer(ctx, ids[0])
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
		}
		if err != nil {
			return actionDoneMsg{err: err}
		}
		return actionDoneMsg{msg: done}
	}
}

func (m Model) runOnSelected(verb string, fn func(context.Context, string) error) (tea.Model, tea.Cmd) {
	sel := m.cntPanel.Selected()
	if sel == nil {
		m.setStatus("nothing selected")
		return m, nil
	}
	id := sel.ID
	name := sel.Name
	m.setStatus(verb + " " + name + "…")
	return m, func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), lifecycleTimeout)
		defer cancel()
		if err := fn(ctx, id); err != nil {
			return actionDoneMsg{err: err}
		}
		return actionDoneMsg{msg: verb + " " + name}
	}
}

func (m Model) runGlobal(label string, fn func(context.Context) error) (tea.Model, tea.Cmd) {
	m.setStatus(label + "…")
	return m, func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), globalTimeout)
		defer cancel()
		if err := fn(ctx); err != nil {
			return actionDoneMsg{err: err}
		}
		return actionDoneMsg{msg: label + " ok"}
	}
}

func (m Model) openLogs() (tea.Model, tea.Cmd) {
	sel := m.cntPanel.Selected()
	if sel == nil {
		m.setStatus("nothing selected")
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
		m.setStatus("nothing selected")
		return m, nil
	}
	if !sel.IsRunning() {
		m.setStatus("shell disabled: container is not running")
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
		m.setLastErr(err)
	} else {
		m.setStatus("copied " + text)
	}
	return m, nil
}

func (m Model) beginPrompt(kind, _ string) (tea.Model, tea.Cmd) {
	m.mode = modePrompt
	m.promptKind = kind
	m.promptBuf = ""
	return m, nil
}

func (m Model) handlePromptKey(k string) (tea.Model, tea.Cmd) {
	switch k {
	case "esc":
		m.mode = modeBrowse
		m.promptBuf = ""
		m.setStatus("cancelled")
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
	if msg.text == "" {
		m.setStatus("empty input")
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
	case "custom":
		return m.runCustom(msg.text)
	}
	return m, nil
}

func (m Model) openActions() (tea.Model, tea.Cmd) {
	items := m.buildActions()
	if len(items) == 0 {
		m.setStatus("no actions")
		return m, nil
	}
	m.mode = modeActions
	m.actionItems = items
	m.actionIdx = 0
	return m, nil
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
				x, c := m.beginStop()
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
				x, c := m.beginPrune(pruneContainers)
				return x.(Model), c
			}},
		)
	case ViewImages:
		items = append(items,
			actionItem{"Pull…", func(m Model) (Model, tea.Cmd) {
				x, c := m.beginPrompt("pull", "image")
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
			actionItem{"Prune unused", func(m Model) (Model, tea.Cmd) {
				x, c := m.beginPrune(pruneImages)
				return x.(Model), c
			}},
		)
	case ViewVolumes:
		items = append(items,
			actionItem{"Create…", func(m Model) (Model, tea.Cmd) {
				x, c := m.beginPrompt("volcreate", "name")
				return x.(Model), c
			}},
			actionItem{"Prune unused", func(m Model) (Model, tea.Cmd) {
				x, c := m.beginPrune(pruneVolumes)
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

// customCommandForKey returns the command string of the first custom command
// configured with key k, or "" if none. A configured key is the user's explicit
// opt-in and shadows the built-in action it collides with, except for the
// reserved navigation/filter/global keys, which stay usable no matter what the
// config binds.
func (m Model) customCommandForKey(k string) string {
	return customCommandFor(m.cfg.CustomCommands, m.keys, k)
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
	m.setStatus("custom: " + cmdStr)
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
		return actionDoneMsg{msg: uiutil.Truncate(msg, 80)}
	}
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

// footerView renders the footer. layoutDims reserves exactly one row for it
// (bodyH = m.height-2-cmdH) and lipgloss word-wraps rather than truncates, so
// every path is flattened onto one line and cut to the terminal width here.
func (m Model) footerView() string {
	style, line := m.footerLine()
	line = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == '\t' {
			return ' '
		}
		return r
	}, line)
	return style.Width(m.width).Render(uiutil.TruncateCells(line, m.width))
}

func (m Model) footerLine() (lipgloss.Style, string) {
	// Whichever of status and lastErr was set last takes the footer. A status
	// set after a sticky services-down hint must be visible (that hint survives
	// an unrelated success, see applyContainersLoaded, so it would otherwise
	// mask every later message), but a failure reported after that status must
	// not stay hidden behind it. Neither field is cleared here: the loser just
	// gives up the line until it is written again.
	if m.status != "" && (m.lastErr == nil || m.statusGen > m.errGen) {
		return m.st.footerHelp, strings.Join(strings.Fields(m.status), " ")
	}
	if m.lastErr != nil {
		const prefix = "error: "
		hint := ""
		if backend.IsServicesDown(m.lastErr) {
			hint = " — run `container system start` to start services"
		}
		// The raw CLI error is long and multi-line; collapse and truncate it to
		// what is left beside the hint so the hint itself is never cut.
		msg := strings.Join(strings.Fields(m.lastErr.Error()), " ")
		msg = uiutil.TruncateCells(msg, max(0, m.width-lipgloss.Width(prefix)-lipgloss.Width(hint)))
		return m.st.errorText, prefix + msg + hint
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
	return m.st.footerHelp, prefix + keys
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

// helpVisibleRows is how many binding rows fit beside helpView's fixed chrome
// (title, view/focus line, blank, blank, close hint). lipgloss pads a box to its
// declared height but never truncates, so a help list longer than this would
// render past the alt screen and lose its last rows.
func helpVisibleRows(height int) int {
	return max(1, height-5)
}

func (m Model) helpBindings() []helpRow {
	return helpBindings(m.activeView, m.focus, m.mode, m.keys, m.cfg.CustomCommands)
}

func (m Model) clampHelpScroll(v int) int {
	overflow := max(0, len(m.helpBindings())-helpVisibleRows(m.height))
	return max(0, min(v, overflow))
}

func (m Model) helpView() string {
	bindings := m.helpBindings()
	start := m.clampHelpScroll(m.helpScroll)
	end := min(len(bindings), start+helpVisibleRows(m.height))

	keyW := 0
	for _, b := range bindings {
		keyW = max(keyW, lipgloss.Width(b.key))
	}
	keyW = min(keyW, max(1, m.width/2))
	descW := max(1, m.width-keyW-1)

	var rows []string
	rows = append(rows, m.st.title.Render("vessel — keybindings"))
	rows = append(rows, m.st.dimText.Render(fmt.Sprintf("view=%s focus=%s", m.viewName(), m.focus.String())))
	rows = append(rows, "")
	for _, b := range bindings[start:end] {
		key := m.st.helpText.Render(uiutil.PadCells(b.key, keyW))
		rows = append(rows, key+" "+uiutil.TruncateCells(b.desc, descW))
	}
	rows = append(rows, "")
	hint := "press ? or esc to close"
	if end-start < len(bindings) {
		hint = fmt.Sprintf("%d-%d of %d — j/k scroll — press ? or esc to close", start+1, end, len(bindings))
	}
	rows = append(rows, m.st.dimText.Render(uiutil.TruncateCells(hint, m.width)))
	return lipgloss.NewStyle().
		Width(m.width).Height(m.height).
		Align(lipgloss.Center, lipgloss.Center).
		Render(lipgloss.JoinVertical(lipgloss.Left, rows...))
}

// confirmQuestion returns the concrete question for the pending confirm (delete,
// prune, stop), rather than a generic "are you sure?".
func (m Model) confirmQuestion() string {
	if spec, ok := pruneSpecFor(m.pendingKind); ok {
		return spec.question
	}
	label := m.pendingLbl
	if label == "" {
		label = strings.Join(m.pendingIDs, ", ")
	}
	if m.pendingKind == stopContainer {
		return fmt.Sprintf("Stop %s?", label)
	}
	return fmt.Sprintf("Delete %s?", label)
}

func (m Model) confirmModal() string {
	body := fmt.Sprintf("%s\n\n[y] confirm   [n/esc] cancel", m.confirmQuestion())
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorRed).
		Padding(1, 2).
		Width(min(48, m.width-4)).
		Render(body)
}

func (m Model) actionsModal() string {
	var rows []string
	rows = append(rows, m.st.title.Render("actions"))
	rows = append(rows, "")
	for i, a := range m.actionItems {
		line := "  " + a.label
		if i == m.actionIdx {
			line = m.st.navItemActive.Render("> " + a.label)
		}
		rows = append(rows, line)
	}
	rows = append(rows, "")
	rows = append(rows, m.st.dimText.Render("[enter] run  [esc] close"))
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorPurple).
		Padding(1, 2).
		Width(min(40, m.width-4)).
		Render(lipgloss.JoinVertical(lipgloss.Left, rows...))
}

func (m Model) promptModal() string {
	title := m.promptKind
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
