package ui

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"charm.land/lipgloss/v2"

	"github.com/Laaaaksh/vessel/internal/backend"
	"github.com/Laaaaksh/vessel/internal/ui/uiutil"
)

// runFieldKind distinguishes a free-text field from an on/off flag, since
// each takes input differently: text accepts runes and backspace, a flag
// only toggles.
type runFieldKind int

const (
	runFieldText runFieldKind = iota
	runFieldBool
)

// Field indices into runForm.text/bools. Order here is render order and
// (for text fields) Tab/↓ traversal order.
const (
	runFieldImage int = iota
	runFieldName
	runFieldPorts
	runFieldEnv
	runFieldVolumes
	runFieldMemory
	runFieldCPUs
	runFieldArch
	runFieldDetached
	runFieldTTY
	runFieldInteractive
	runFieldCount
)

// runListSeparator delimits repeated -p/-e/-v entries typed into one field.
const runListSeparator = ","

// runFieldLabelWidth aligns every field's label column in the form.
const runFieldLabelWidth = 18

type runFieldSpec struct {
	label string
	hint  string
	kind  runFieldKind
}

// runFieldSpecs is the practical `container run` flag subset the form
// exposes - deliberately not the CLI's full flag surface (--cap-add,
// --ulimit, --uid, --env-file, ... are out of scope).
var runFieldSpecs = [runFieldCount]runFieldSpec{
	runFieldImage:       {label: "Image", hint: "image[:tag] to run", kind: runFieldText},
	runFieldName:        {label: "Name (--name)", hint: "container name", kind: runFieldText},
	runFieldPorts:       {label: "Ports (-p)", hint: "host:container, comma-separated", kind: runFieldText},
	runFieldEnv:         {label: "Env (-e)", hint: "KEY=VALUE, comma-separated", kind: runFieldText},
	runFieldVolumes:     {label: "Volumes (-v)", hint: "host:container, comma-separated", kind: runFieldText},
	runFieldMemory:      {label: "Memory (-m)", hint: "e.g. 512M", kind: runFieldText},
	runFieldCPUs:        {label: "CPUs (-c)", hint: "e.g. 2", kind: runFieldText},
	runFieldArch:        {label: "Arch (--arch)", hint: "e.g. arm64, amd64", kind: runFieldText},
	runFieldDetached:    {label: "Detached (-d)", kind: runFieldBool},
	runFieldTTY:         {label: "TTY (-t)", kind: runFieldBool},
	runFieldInteractive: {label: "Interactive (-i)", kind: runFieldBool},
}

// runForm holds the in-progress state of the run/create modal.
type runForm struct {
	text  [runFieldCount]string
	bools [runFieldCount]bool
	focus int
	err   string
}

// newRunForm starts a form with image prefilled from the current selection
// (Images view) or blank (Containers view, where the user types it).
func newRunForm(image string) runForm {
	var f runForm
	f.text[runFieldImage] = image
	return f
}

// insert appends k to the focused text field, or toggles the focused bool
// field when k is a bare "space" press. It mirrors handlePromptKey's fix:
// bubbletea serialises a space bar press as the literal string "space" (never
// " "), and a multi-byte rune is still exactly one printable character even
// though it is more than one byte, so both must survive here the same way
// they now survive in the single-token prompt.
func (f *runForm) insert(k string) {
	spec := runFieldSpecs[f.focus]
	if spec.kind == runFieldBool {
		if k == keySpace {
			f.bools[f.focus] = !f.bools[f.focus]
		}
		return
	}
	if k == keySpace {
		f.text[f.focus] += " "
		return
	}
	if utf8.RuneCountInString(k) == 1 {
		f.text[f.focus] += k
	}
}

// backspace removes the focused text field's trailing rune (not just its
// last byte, or a multi-byte character would corrupt into invalid UTF-8).
// A bool field has nothing to erase.
func (f *runForm) backspace() {
	if runFieldSpecs[f.focus].kind != runFieldText {
		return
	}
	s := f.text[f.focus]
	if s == "" {
		return
	}
	_, size := utf8.DecodeLastRuneInString(s)
	f.text[f.focus] = s[:len(s)-size]
}

// move advances or retreats focus by delta, clamped to the field range.
func (f *runForm) move(delta int) {
	f.focus = uiutil.MoveCursor(f.focus, runFieldCount, delta)
}

// validate builds the run submission from the form's current text, or
// reports the first invalid field. Validation happens here, before anything
// reaches the CLI, so a malformed entry produces a specific message instead
// of a raw CLI error dump.
func (f runForm) validate() (image string, opts backend.RunOptions, errMsg string) {
	image = strings.TrimSpace(f.text[runFieldImage])
	if image == "" {
		return "", backend.RunOptions{}, "image is required"
	}
	ports, bad := splitValidated(f.text[runFieldPorts], hasNonEmptyColonParts)
	if bad != "" {
		return "", backend.RunOptions{}, fmt.Sprintf("invalid port mapping %q (want host:container)", bad)
	}
	env, bad := splitValidated(f.text[runFieldEnv], hasNonEmptyKeyValue)
	if bad != "" {
		return "", backend.RunOptions{}, fmt.Sprintf("invalid env entry %q (want KEY=VALUE)", bad)
	}
	volumes, bad := splitValidated(f.text[runFieldVolumes], hasNonEmptyColonParts)
	if bad != "" {
		return "", backend.RunOptions{}, fmt.Sprintf("invalid volume mount %q (want host:container)", bad)
	}
	opts = backend.RunOptions{
		Name:        strings.TrimSpace(f.text[runFieldName]),
		Ports:       ports,
		Env:         env,
		Volumes:     volumes,
		Memory:      strings.TrimSpace(f.text[runFieldMemory]),
		CPUs:        strings.TrimSpace(f.text[runFieldCPUs]),
		Arch:        strings.TrimSpace(f.text[runFieldArch]),
		Detached:    f.bools[runFieldDetached],
		TTY:         f.bools[runFieldTTY],
		Interactive: f.bools[runFieldInteractive],
	}
	return image, opts, ""
}

// splitList splits a comma-separated field into trimmed, non-empty entries.
func splitList(raw string) []string {
	var out []string
	for _, part := range strings.Split(raw, runListSeparator) {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

// splitValidated splits raw and reports the first entry that fails valid, if
// any, so the caller can name exactly which entry was malformed.
func splitValidated(raw string, valid func(string) bool) (items []string, badEntry string) {
	for _, it := range splitList(raw) {
		if !valid(it) {
			return nil, it
		}
		items = append(items, it)
	}
	return items, ""
}

// hasNonEmptyColonParts reports whether s has a ':' with content on both
// sides, the shape both -p (host:container[/proto]) and -v (host:container)
// entries share.
func hasNonEmptyColonParts(s string) bool {
	i := strings.Index(s, ":")
	return i > 0 && i < len(s)-1
}

// hasNonEmptyKeyValue reports whether s is a non-empty KEY=VALUE pair.
func hasNonEmptyKeyValue(s string) bool {
	return strings.Index(s, "=") > 0
}

// runFormVisibleRows returns how many field rows fit in a modal of the given
// height once the title, hint and (if present) error line are accounted for.
func runFormVisibleRows(height int, hasErr bool) int {
	reserved := 2 // title + hint
	if hasErr {
		reserved++
	}
	return max(1, height-reserved)
}

// runFormWindow returns the [start,end) field range to render so the focused
// field is always visible within a window of `visible` rows.
func runFormWindow(focus, visible int) (start, end int) {
	if visible >= runFieldCount {
		return 0, runFieldCount
	}
	start = focus - visible/2
	if start < 0 {
		start = 0
	}
	end = start + visible
	if end > runFieldCount {
		end = runFieldCount
		start = end - visible
	}
	return start, end
}

// runFormModal renders the run/create form. At the smallest supported
// terminal (60x12) the field list cannot all fit alongside the border and
// padding, so it windows around the focused field and shows a "(n/total)"
// counter instead of overflowing.
func (m Model) runFormModal() string {
	width := min(60, max(44, m.width-4))
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorBlue).
		Padding(1, 2).
		Width(width)
	// Ask the style for its own frame size rather than assuming how many
	// columns/rows its border and padding cost: guessing that wrong is
	// exactly what overflowed the modal past a 60x12 terminal before.
	frameW, frameH := box.GetFrameSize()
	innerWidth := width - frameW
	innerHeight := max(1, m.height-frameH)
	visible := runFormVisibleRows(innerHeight, m.runForm.err != "")
	start, end := runFormWindow(m.runForm.focus, visible)

	title := m.st.title.Render("run container")
	if start > 0 || end < runFieldCount {
		title += m.st.dimText.Render(fmt.Sprintf("  (%d/%d)", m.runForm.focus+1, runFieldCount))
	}
	rows := []string{title}
	for i := start; i < end; i++ {
		rows = append(rows, m.renderRunField(i, innerWidth))
	}
	if m.runForm.err != "" {
		rows = append(rows, m.st.errorText.Render(uiutil.Truncate(m.runForm.err, innerWidth)))
	}
	hint := uiutil.Truncate("[enter] run  [tab] field  [space] toggle  [esc] cancel", innerWidth)
	rows = append(rows, m.st.dimText.Render(hint))

	return box.Render(lipgloss.JoinVertical(lipgloss.Left, rows...))
}

// renderRunField renders one field row: a checkbox for a bool flag, or a
// label/value pair for text, with a placeholder hint shown while empty and a
// cursor shown while focused.
func (m Model) renderRunField(i, width int) string {
	spec := runFieldSpecs[i]
	focused := i == m.runForm.focus

	var body string
	if spec.kind == runFieldBool {
		mark := " "
		if m.runForm.bools[i] {
			mark = "x"
		}
		body = fmt.Sprintf("[%s] %s", mark, spec.label)
	} else {
		display := m.runForm.text[i]
		switch {
		case focused:
			display += "_"
		case display == "":
			display = spec.hint
		}
		body = fmt.Sprintf("%-*s %s", runFieldLabelWidth, spec.label, display)
	}
	body = uiutil.Truncate(body, width)
	if focused {
		return m.st.navItemActive.Width(width).Render(body)
	}
	return m.st.detailValue.Render(body)
}
