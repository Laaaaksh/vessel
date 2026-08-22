package ui

import (
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// readmeParity guards the README keybindings table against the in-app help
// overlay in both directions, because they have drifted before: the help once
// spelled the page-down key "pgdown" while the README said "pgdn", and the
// exec binding shipped in help without any README row at all.
//
// The table is the contract users read; helpBindings is what the app really
// answers. Every key named in one must be discoverable in the other.

// readmeGlyphNames normalizes the arrow glyphs README.md spells arrow keys
// with onto the names tea.KeyPressMsg.String() emits for them.
var readmeGlyphNames = map[string]string{
	"←": "left",
	"→": "right",
	"↑": "up",
	"↓": "down",
}

var (
	readmeBacktickSpan = regexp.MustCompile("`([^`]+)`")
	readmeDigitRange   = regexp.MustCompile("`([0-9])`-`([0-9])`")
)

// readmeHelpMenuRow is the one help row whose key column is not a set of keys:
// it points into the action menu ("x → image mobility"), so its extra words
// are exempt from the reverse README-coverage check.
const readmeHelpMenuRow = "x → image mobility"

// readmeBacktickKey is how README.md writes a literal backtick key inline
// (double-backtick quoting); readmeBtickPlaceholder stands in for it so the
// ordinary single-backtick span extraction cannot swallow it.
const (
	readmeBacktickKey      = "`` ` ``"
	readmeBtickPlaceholder = "btick"
)

// readmeTableRows parses every Key-column cell of README.md's keybindings
// table and returns its key tokens, normalized onto runtime key spellings:
// arrows become left/right/up/down and `1`-`5` expands to 1 2 3 4 5.
func readmeTableRows(t *testing.T) [][]string {
	t.Helper()
	data, err := os.ReadFile("../../README.md")
	if err != nil {
		t.Fatalf("read README.md: %v", err)
	}
	var rows [][]string
	sawHeading := false
	sawTable := false
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "### Keybindings") {
			sawHeading = true
			continue
		}
		if !sawHeading {
			continue
		}
		if !strings.HasPrefix(line, "|") {
			// Blank lines separate the heading from the table; once the
			// table has started, the first non-table line ends it.
			if sawTable {
				break
			}
			continue
		}
		sawTable = true
		fields := strings.Split(line, "|")
		if len(fields) < 3 {
			continue
		}
		cell := strings.TrimSpace(fields[1])
		if cell == "" || cell == "Key" || strings.Trim(cell, "- ") == "" {
			continue // header or column-separator row
		}
		cell = strings.ReplaceAll(cell, readmeBacktickKey, "`"+readmeBtickPlaceholder+"`")
		expanded := readmeDigitRange.ReplaceAllStringFunc(cell, func(m string) string {
			parts := readmeDigitRange.FindStringSubmatch(m)
			from, _ := strconv.Atoi(parts[1])
			to, _ := strconv.Atoi(parts[2])
			toks := make([]string, 0, to-from+1)
			for d := from; d <= to; d++ {
				toks = append(toks, "`"+strconv.Itoa(d)+"`")
			}
			return strings.Join(toks, " ")
		})
		var toks []string
		for _, span := range readmeBacktickSpan.FindAllStringSubmatch(expanded, -1) {
			k := span[1]
			if k == readmeBtickPlaceholder {
				k = "`"
			} else if named, ok := readmeGlyphNames[k]; ok {
				k = named
			}
			if strings.TrimSpace(k) == "" {
				t.Fatalf("README keybindings cell %q yielded a whitespace key - the parser mis-handled it", cell)
			}
			toks = append(toks, k)
		}
		if len(toks) > 0 {
			rows = append(rows, toks)
		}
	}
	if len(rows) < 20 {
		t.Fatalf("parsed only %d keybinding rows from README.md - the table parser broke, not the docs", len(rows))
	}
	return rows
}

// readmeRowScope says which sidebar views' help overlays must list every key
// of a README table row, keyed by that row's normalized tokens joined with a
// comma. Rows are matched by key set rather than prose so rewording a
// description cannot orphan an entry, but changing a row's keys fails loudly
// until this scope is revisited. A parsed README row missing here is itself a
// failure: a new user-facing binding needs a deliberate scope decision, never
// a silent pass.
func readmeRowScope(toks []string) []View {
	key := strings.Join(toks, ",")
	all := []View{ViewContainers, ViewImages, ViewVolumes, ViewSystem, ViewNetworks}
	listAndSystem := []View{ViewContainers, ViewImages, ViewVolumes, ViewSystem}
	switch key {
	case "enter", "e", "L", "s,u,r":
		// Container actions live in the containers view; the system view
		// shares the same help branch today.
		return []View{ViewContainers, ViewSystem}
	case "d", "P", "c":
		return listAndSystem
	case "p":
		return []View{ViewImages}
	default:
		return all
	}
}

// helpTokensForView collects the union of key tokens one view's help overlay
// advertises, skipping non-key rows.
func helpTokensForView(v View) map[string]bool {
	tokens := map[string]bool{}
	for _, b := range helpBindings(v, FocusList, modeBrowse, DefaultKeyMap(), nil) {
		if b.key == readmeHelpMenuRow {
			continue
		}
		for _, tok := range helpKeyTokens(b.key) {
			tokens[tok] = true
		}
	}
	return tokens
}

// TestReadmeKeysAreAdvertisedByHelp fails when a README-documented key has no
// help entry in any view whose help is supposed to document it.
func TestReadmeKeysAreAdvertisedByHelp(t *testing.T) {
	helpTokens := make(map[View]map[string]bool)
	for _, v := range []View{ViewContainers, ViewImages, ViewVolumes, ViewSystem, ViewNetworks} {
		helpTokens[v] = helpTokensForView(v)
	}
	for _, toks := range readmeTableRows(t) {
		for _, v := range readmeRowScope(toks) {
			for _, k := range toks {
				if !helpTokens[v][k] {
					t.Errorf("README documents key %q but the %v help overlay does not advertise it", k, v)
				}
			}
		}
	}
}

// TestHelpKeysAreDocumentedInReadme fails when help advertises a key no README
// table row names - the direction the exec (`e`) binding drifted in.
func TestHelpKeysAreDocumentedInReadme(t *testing.T) {
	readmeTokens := map[string]bool{}
	for _, toks := range readmeTableRows(t) {
		for _, k := range toks {
			readmeTokens[k] = true
		}
	}
	for _, v := range []View{ViewContainers, ViewImages, ViewVolumes, ViewSystem, ViewNetworks} {
		for k := range helpTokensForView(v) {
			if !readmeTokens[k] {
				t.Errorf("%v help advertises key %q but no README keybindings row documents it", v, k)
			}
		}
	}
}
