package ui

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/Laaaaksh/vessel/internal/backend"
	"github.com/Laaaaksh/vessel/internal/config"
	"github.com/Laaaaksh/vessel/internal/ui/containers"
	"github.com/Laaaaksh/vessel/internal/ui/images"
	"github.com/Laaaaksh/vessel/internal/ui/logs"
	"github.com/Laaaaksh/vessel/internal/ui/networks"
	"github.com/Laaaaksh/vessel/internal/ui/system"
	"github.com/Laaaaksh/vessel/internal/ui/uiutil"
	"github.com/Laaaaksh/vessel/internal/ui/volumes"
)

// keySpace is the literal tea.KeyPressMsg.String() serialisation of a space
// bar press: ultraviolet's Keystroke() special-cases KeySpace to the word
// "space" rather than a literal " " (see AGENTS.md, UI key handling), so a
// prompt that only appended len==1 runes silently dropped every space typed
// into it.
const keySpace = "space"

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

type networksLoadedMsg struct {
	items []backend.NetworkInfo
	err   error
}

// systemLoadedMsg carries both system-status and disk-usage poll results.
// They are combined into one message because the panel always shows them
// together, and each field's error is independent: a services-down df
// failure must not blank out a successful status, or vice versa.
type systemLoadedMsg struct {
	status    *backend.SystemStatus
	statusErr error
	usage     *backend.DiskUsage
	usageErr  error
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
	modeRunForm
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
	// All four are outer bounds: backend.Client applies a per-call budget
	// of its own to the known-long verbs — the lifecycle window matches
	// lifecycleTimeout, the transfer/sweep window matches globalTimeout,
	// the batched-delete window matches confirmTimeout and the one-shot
	// exec window matches execTimeout — while quick commands keep a short
	// default of their own. Both halves of each pair are pinned
	// (TestOuterBounds_matchBackendPerCallBudgets here,
	// TestLongOperationBudgets_matchInternalUIOuterBounds in backend), so
	// neither side can drift alone and quietly become the earlier deadline.
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
	netPanel networks.Model
	sysPanel system.Model
	logPanel logs.Model

	lastErr     error
	status      string
	footerSeq   uint64
	statusGen   uint64
	errGen      uint64
	errDurable  bool
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

	inspectKey  string
	inspectRun  string
	actionIdx   int
	actionItems []actionItem
	promptKind  string
	promptLabel string
	promptBuf   string
	promptRef   string
	runForm     runForm
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
	m.errDurable = false
}

// setActionErr records an error from a user-initiated action (tag, save,
// load, exec, ...) as durable: applyContainersLoaded's self-heal on the next
// successful poll must not wipe it, because that poll carries no information
// about whether the action itself succeeded. It stays up until a newer
// status/error is set or a subsequent action succeeds - see footerLine and
// applyContainersLoaded.
func (m *Model) setActionErr(err error) {
	m.setLastErr(err)
	m.errDurable = true
}

// New creates the root model. Backend connection happens in Init.
func New() Model {
	cfg, err := config.Load()
	m := newModel(cfg)
	if err != nil {
		// A broken config.toml must not silently degrade to defaults: join
		// the load error onto whatever startup notice newModel stamped so
		// the user learns about it through the ordinary footer lifecycle.
		m.setStatus(joinNotices(m.status, configLoadNotice(err)))
	}
	return m
}

// configLoadNotice renders a failed config load as a one-time startup footer
// message, or "" when the load succeeded. The error leads so footer
// truncation keeps the actionable part; the path trails so users can find
// the file (vessel doctor prints the same path).
func configLoadNotice(err error) string {
	if err == nil {
		return ""
	}
	return fmt.Sprintf("config: %v (%s)", err, config.Path())
}

// joinNotices merges two independent one-time startup messages into a single
// footer line; either side may be empty.
func joinNotices(a, b string) string {
	switch {
	case a == "":
		return b
	case b == "":
		return a
	default:
		return a + noticeDetailSep + b
	}
}

// newModel builds the root model around an already-loaded config, so tests can
// drive startup without depending on the host's ~/.config/vessel/config.toml.
// Configured bindings that can never fire are reported once here, through
// setStatus, so they ride the ordinary footer lifecycle: visible until real
// activity replaces them, never blocking anything.
func newModel(cfg config.Config) Model {
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
		netPanel:   networks.New(),
		sysPanel:   system.New(),
		logPanel:   logs.New(),
	}
	m = m.withKeys(DefaultKeyMap())
	if notice := ignoredKeysNotice(cfg.CustomCommands, m.keys); notice != "" {
		m.setStatus(notice)
	}
	return m
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
		m.netPanel = m.netPanel.SetPageRows(rows)
		m.sysPanel = m.sysPanel.SetPageRows(rows)
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
	case systemLoadedMsg:
		// Errors stay local to the panel rather than the global error banner:
		// a services-down df failure is this view's own normal subject, not
		// an app-wide failure to report on.
		m.sysPanel = m.sysPanel.SetStatus(msg.status, msg.statusErr).SetDiskUsage(msg.usage, msg.usageErr)
		return m, nil
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
	case networksLoadedMsg:
		if msg.err != nil {
			m.setLastErr(msg.err)
		} else {
			m.netPanel = m.netPanel.SetItems(msg.items)
		}
		return m, nil
	case actionDoneMsg:
		// An action failure is durable (setActionErr): it must survive the
		// refreshCmd below and the next periodic tick, both of which land a
		// containersLoadedMsg carrying no information about this action. See
		// setActionErr and applyContainersLoaded's errDurable check.
		m.imgPanel = m.imgPanel.SetNotice("", "")
		if msg.err != nil {
			m.setActionErr(msg.err)
			m.setStatus("")
			if msg.pushRef != "" {
				if notice := backend.PushDenialNotice(msg.err); notice != "" {
					m.imgPanel = m.imgPanel.SetNotice(msg.pushRef, notice)
				}
			}
		} else {
			m.setLastErr(nil)
			m.setStatus(msg.msg)
		}
		m = m.clearPending()
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
		m = m.clearPending()
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
	if m.mode == modeRunForm {
		content = m.overlayModal(content, m.runFormModal())
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
	case ViewNetworks:
		list = m.netPanel.ListView(listW-2, height-2)
		detail = m.netPanel.DetailView(detailW-2, height-2)
	case ViewSystem:
		list = m.sysPanel.ListView(listW-2, height-2)
		detail = m.sysPanel.DetailView(detailW-2, height-2)
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
	case ViewNetworks:
		return m.loadNetworksCmd()
	case ViewSystem:
		return m.loadSystemCmd()
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

func (m Model) loadNetworksCmd() tea.Cmd {
	client := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		items, err := client.ListNetworks(ctx)
		return networksLoadedMsg{items: items, err: err}
	}
}

// loadSystemCmd loads service status and disk usage. Both are fetched
// unconditionally on every poll: unlike an inspect, this view has no
// selection to debounce against, and a services-down df failure is expected
// to happen alongside a perfectly good status result.
func (m Model) loadSystemCmd() tea.Cmd {
	client := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		status, statusErr := client.SystemStatus(ctx)
		usage, usageErr := client.DiskUsage(ctx)
		return systemLoadedMsg{status: status, statusErr: statusErr, usage: usage, usageErr: usageErr}
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
		// than wiping it on the next refresh. A durable action error (see
		// setActionErr) is the same story for a different reason: this poll
		// never touched the action that produced it, so it is not evidence
		// either way and must not clear it.
		if !backend.IsServicesDown(m.lastErr) && !m.errDurable {
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
	if m.mode == modeRunForm {
		return m.handleRunFormKey(k)
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
		m.activeView = (m.activeView + 1) % viewCount
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
	case k == "4":
		m.activeView = ViewSystem
		return m, m.activeViewLoadCmd()
	case k == "5":
		m.activeView = ViewNetworks
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
			m.activeView = (m.activeView + 1) % viewCount
			return m, m.activeViewLoadCmd()
		case m.keys.NavUp(k):
			m.activeView = (m.activeView + viewCount - 1) % viewCount
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
			return m.beginRunForm("")
		case Match(k, m.keys.Exec):
			return m.beginExec()
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
			ref := ""
			if sel := m.imgPanel.Selected(); sel != nil {
				ref = backend.FormatRef(*sel)
			}
			return m.beginRunForm(ref)
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
	case ViewNetworks:
		return m.netPanel.Filtering()
	case ViewSystem:
		return false
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
	case ViewNetworks:
		m.netPanel, cmd = m.netPanel.Update(msg)
	case ViewSystem:
		m.sysPanel, cmd = m.sysPanel.Update(msg)
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
	case ViewNetworks:
		m.netPanel = m.netPanel.SetCursor(row)
	case ViewSystem:
		m.sysPanel = m.sysPanel.SetCursor(row)
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
	case ViewNetworks:
		m.netPanel = m.netPanel.MoveBy(delta)
	case ViewSystem:
		m.sysPanel = m.sysPanel.MoveBy(delta)
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
	m = m.clearPending()
	if client == nil {
		m.setStatus(label + "…")
		return m, unavailableCmd()
	}
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
	if m.client == nil {
		return m, unavailableCmd()
	}
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
	return m.runAction(label, "", fn)
}

// runPush is runGlobal for the one verb whose failures may carry registry
// credential advice; ref is the image that advice would be about.
func (m Model) runPush(label, ref string, fn func(context.Context) error) (tea.Model, tea.Cmd) {
	return m.runAction(label, ref, fn)
}

func (m Model) runAction(label, pushRef string, fn func(context.Context) error) (tea.Model, tea.Cmd) {
	m.setStatus(label + "…")
	if m.client == nil {
		return m, unavailableCmd()
	}
	return m, func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), globalTimeout)
		defer cancel()
		if err := fn(ctx); err != nil {
			return actionDoneMsg{err: err, pushRef: pushRef}
		}
		return actionDoneMsg{msg: label + " ok"}
	}
}

// unavailableCmd is what a write action returns when Init could not find the
// container CLI (m.client == nil): an ordinary failed-action message through
// the one error-rendering path, instead of the action closure dereferencing
// the nil client inside its goroutine and panicking.
func unavailableCmd() tea.Cmd {
	return func() tea.Msg {
		return actionDoneMsg{err: errors.New("container CLI unavailable")}
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
	case ViewNetworks:
		if sel := m.netPanel.Selected(); sel != nil {
			text = sel.Name
		}
	case ViewSystem:
		text = m.sysPanel.YankText()
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

// handlePromptKey drives the text prompt. Phase 3's Save… and Load… prompts
// take filesystem paths, so the space bar (which serialises as the literal
// string "space", never " ") and multi-byte non-ASCII runes must round-trip
// correctly: a dropped space would make Save write to a path the user never
// typed and Load report "no such file" for a file that exists.
func (m Model) handlePromptKey(k string) (tea.Model, tea.Cmd) {
	switch k {
	case "esc":
		m.mode = modeBrowse
		m.promptBuf = ""
		m.promptRef = ""
		m.promptLabel = ""
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
			_, size := utf8.DecodeLastRuneInString(m.promptBuf)
			m.promptBuf = m.promptBuf[:len(m.promptBuf)-size]
		}
		return m, nil
	case keySpace:
		m.promptBuf += " "
		return m, nil
	default:
		// A byte-length check here would reject any multi-byte rune (accents,
		// CJK, emoji) even though it is exactly one printable character; count
		// runes instead so only real multi-key strings ("tab", "ctrl+a", ...)
		// are excluded.
		if utf8.RuneCountInString(k) == 1 {
			m.promptBuf += k
		}
		return m, nil
	}
}

func (m Model) handlePrompt(msg promptDoneMsg) (tea.Model, tea.Cmd) {
	ref := m.promptRef
	m.promptRef = ""
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
	case "exec":
		return m.submitExec(msg.text)
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

// beginRunForm opens the run/create form, prefilled with image when one was
// already selected (Images view); Containers view opens it blank since there
// is no image selection to seed it from.
func (m Model) beginRunForm(image string) (tea.Model, tea.Cmd) {
	m.mode = modeRunForm
	m.runForm = newRunForm(image)
	return m, nil
}

func (m Model) handleRunFormKey(k string) (tea.Model, tea.Cmd) {
	switch {
	case k == "esc":
		m.mode = modeBrowse
		m.status = "cancelled"
		return m, nil
	case Match(k, m.keys.Enter):
		return m.submitRunForm()
	case Match(k, m.keys.Tab) || m.keys.NavDown(k):
		m.runForm.move(1)
		return m, nil
	case m.keys.NavUp(k):
		m.runForm.move(-1)
		return m, nil
	case k == "backspace":
		m.runForm.backspace()
		return m, nil
	default:
		m.runForm.insert(k)
		return m, nil
	}
}

// submitRunForm validates the form and, if valid, runs the container and
// closes the form. An invalid field reports its own message inline and
// leaves the form open rather than reaching the CLI with bad input.
func (m Model) submitRunForm() (tea.Model, tea.Cmd) {
	image, opts, errMsg := m.runForm.validate()
	if errMsg != "" {
		m.runForm.err = errMsg
		return m, nil
	}
	m.mode = modeBrowse
	client := m.client
	return m.runGlobal("run "+image, func(ctx context.Context) error {
		_, err := client.Run(ctx, image, opts)
		return err
	})
}

// execTimeout bounds a one-shot exec, matching runOnSelected's per-item budget
// and backend's identically named per-call budget for the same verb.
const execTimeout = 30 * time.Second

// execOutputTruncate keeps a one-shot exec's result on one footer line, the
// same width runCustom truncates a custom command's output to.
const execOutputTruncate = 80

// beginExec starts the one-shot exec prompt: a single command line run once
// in the selected running container via its shell, not an interactive
// session (see openShell for that).
func (m Model) beginExec() (tea.Model, tea.Cmd) {
	sel := m.cntPanel.Selected()
	if sel == nil {
		m.status = "nothing selected"
		return m, nil
	}
	if !sel.IsRunning() {
		m.status = "exec disabled: container is not running"
		return m, nil
	}
	return m.beginPrompt("exec", "command to run in "+sel.Name)
}

// submitExec runs command in the container selected when the exec prompt was
// opened (selection cannot change while a prompt has key focus) and reports
// its output, truncated like other one-off command results in this app.
func (m Model) submitExec(command string) (tea.Model, tea.Cmd) {
	sel := m.cntPanel.Selected()
	if sel == nil {
		m.status = "nothing selected"
		return m, nil
	}
	id, name := sel.ID, sel.Name
	client := m.client
	shell := m.cfg.Shell
	m.status = "exec: " + command + "…"
	return m, func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), execTimeout)
		defer cancel()
		out, err := client.Exec(ctx, id, shell, command)
		if err != nil {
			return actionDoneMsg{err: err}
		}
		if out == "" {
			out = "exec ok (no output)"
		}
		return actionDoneMsg{msg: "exec " + name + ": " + uiutil.Truncate(out, execOutputTruncate)}
	}
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

// imageActionRef resolves the highlighted row to a reference safe to act on.
// Tag, Save and Push all reach outside the process, and ExactRef refuses two
// separate shapes for separate reasons, so the status names the actual one:
// a pin the CLI is unproven against, or a row that would resolve to ":latest".
func (m Model) imageActionRef() (Model, string, bool) {
	sel := m.imgPanel.Selected()
	if sel == nil {
		m.status = "nothing selected"
		return m, "", false
	}
	ref, ok := backend.ExactRef(*sel)
	if !ok {
		m.status = imageRefRefusal(*sel)
		return m, "", false
	}
	return m, ref, true
}

func imageRefRefusal(img backend.Image) string {
	if img.Digest != "" {
		return "digest-pinned image — tag, save and push refuse a pin until a probe verifies it"
	}
	return "no named reference — the bare repository would resolve to a moving :latest"
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
			actionItem{"Exec…", func(m Model) (Model, tea.Cmd) {
				x, c := m.beginExec()
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
				x, c := m.beginPrompt("pull", "image to pull")
				return x.(Model), c
			}},
			actionItem{"Run…", func(m Model) (Model, tea.Cmd) {
				ref := ""
				if sel := m.imgPanel.Selected(); sel != nil {
					ref = backend.FormatRef(*sel)
				}
				x, c := m.beginRunForm(ref)
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
				x, c := m.beginPrune(pruneImages)
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
	case ViewNetworks:
		if sel := m.netPanel.Selected(); sel != nil {
			id, name = sel.Name, sel.Name
		}
	case ViewSystem:
		// No selection this view reports on maps to {{.ID}}/{{.Name}}/{{.Image}};
		// a custom command run here gets none of them rather than borrowing
		// the container panel's, which routeToPanel keeps updated in the
		// background regardless of the active view.
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
		// Capture stdout and stderr separately: the login shell (-l) sources
		// the user's profile, whose chatter (a stale line in ~/.profile, a
		// version manager's notice) lands on stderr. Merging the streams made
		// that chatter part of the result text and pushed the command's real
		// output past the footer's 80-char truncation window.
		var stdout, stderr bytes.Buffer
		c.Stdout = &stdout
		c.Stderr = &stderr
		err := c.Run()
		if err != nil {
			diag := strings.TrimSpace(stderr.String())
			if diag == "" {
				diag = strings.TrimSpace(stdout.String())
			}
			if diag != "" {
				return actionDoneMsg{err: fmt.Errorf("%w: %s", err, diag)}
			}
			return actionDoneMsg{err: err}
		}
		msg := strings.TrimSpace(stdout.String())
		if msg == "" {
			// Deliberately no stderr fallback here: on a machine whose login
			// profile writes to stderr, the fallback would show that chatter
			// for every silent command instead of the honest "custom ok".
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
	case ViewNetworks:
		return "networks"
	case ViewSystem:
		return "system"
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
	case ViewNetworks:
		keys = "[/] filter  [y] yank"
	case ViewSystem:
		keys = "[j/k] navigate  [y] yank  (read-only)"
	default:
		keys = "[enter] shell  [L] logs  [s/u/r] lifecycle  [c] run  [e] exec  [d] remove  [/] filter  [x] actions  [y] yank"
	}
	return m.st.footerHelp, prefix + keys
}

func (m Model) cursorInfo() (int, int) {
	switch m.activeView {
	case ViewImages:
		return m.imgPanel.Cursor(), m.imgPanel.Len()
	case ViewVolumes:
		return m.volPanel.Cursor(), m.volPanel.Len()
	case ViewNetworks:
		return m.netPanel.Cursor(), m.netPanel.Len()
	case ViewSystem:
		return m.sysPanel.Cursor(), m.sysPanel.Len()
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
		{"System", ViewSystem},
		{"Networks", ViewNetworks},
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
	label := m.pendingLbl
	if label == "" {
		label = strings.Join(m.pendingIDs, ", ")
	}
	// pendingVerb is set only by beginConfirm (the image actions' generic
	// confirm path) and cleared by clearPending on every exit, so it is
	// checked before pendingKind: pendingKind is never reset back to zero and
	// would otherwise misreport this question using a stale prune/stop kind
	// left over from an earlier, unrelated confirmation.
	if m.pendingVerb != "" {
		return fmt.Sprintf("%s %s?", m.pendingVerb, label)
	}
	if spec, ok := pruneSpecFor(m.pendingKind); ok {
		return spec.question
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
