package networks

import (
	"strings"
	"testing"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/suite"

	"github.com/Laaaaksh/vessel/internal/backend"
)

func keyMsg(s string) tea.KeyPressMsg {
	if s == "esc" {
		return tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape})
	}
	r := rune(s[0])
	return tea.KeyPressMsg(tea.Key{Code: r, Text: s})
}

func enterKey() tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter})
}

// spaceKey mirrors a real space bar press: KeyPressMsg.String() returns
// "space", never a literal space.
func spaceKey() tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Code: ' ', Text: " "})
}

func net(name, mode string) backend.NetworkInfo {
	return backend.NetworkInfo{Name: name, Mode: mode}
}

type modelSuite struct {
	suite.Suite
}

func TestModelSuite(t *testing.T) {
	suite.Run(t, new(modelSuite))
}

// typeFilter opens the filter prompt, types s one character per key press
// (mirroring the real UI), then applies it with enter.
func (s *modelSuite) typeFilter(m Model, text string) Model {
	m, _ = m.Update(keyMsg("/"))
	for _, r := range text {
		m, _ = m.Update(keyMsg(string(r)))
	}
	m, _ = m.Update(enterKey())
	return m
}

func (s *modelSuite) TestSetItemsPopulatesTheVisibleList() {
	m := New().SetItems([]backend.NetworkInfo{net("default", "nat"), net("bridge0", "bridge")})

	s.Equal(2, m.Len())
	s.Equal("default", m.Selected().Name)
}

func (s *modelSuite) TestCursorDownMovesToTheNextRow() {
	m := New().SetItems([]backend.NetworkInfo{net("default", "nat"), net("bridge0", "bridge")})

	m, _ = m.Update(keyMsg("j"))

	s.Equal("bridge0", m.Selected().Name)
}

func (s *modelSuite) TestFilterNarrowsToMatchingNames() {
	m := New().SetItems([]backend.NetworkInfo{net("default", "nat"), net("isolated", "bridge")})

	m = s.typeFilter(m, "iso")

	s.Equal(1, m.Len())
	s.Equal("isolated", m.Selected().Name)
}

func (s *modelSuite) TestEscClearsAnActiveFilter() {
	m := New().SetItems([]backend.NetworkInfo{net("default", "nat"), net("isolated", "bridge")})
	m = s.typeFilter(m, "iso")

	m, _ = m.Update(keyMsg("esc"))

	s.Equal(2, m.Len())
}

func (s *modelSuite) TestSetItemsClampsCursorWhenTheListShrinks() {
	m := New().SetItems([]backend.NetworkInfo{net("default", "nat"), net("bridge0", "bridge")})
	m, _ = m.Update(keyMsg("j"))

	m = m.SetItems([]backend.NetworkInfo{net("default", "nat")})

	s.Equal(0, m.Cursor())
	s.Equal("default", m.Selected().Name)
}

func (s *modelSuite) TestSelectedOnAnEmptyListReturnsNil() {
	m := New()

	s.Nil(m.Selected())
}

func (s *modelSuite) TestDetailViewShowsTheSelectedNetworksIdentity() {
	m := New().SetItems([]backend.NetworkInfo{net("default", "nat")})

	view := ansi.Strip(m.DetailView(40, 12))

	s.Contains(view, "default")
	s.Contains(view, "nat")
}

func (s *modelSuite) TestDetailViewAtMinimumTerminalHeightKeepsTheNetworkName() {
	m := New().SetItems([]backend.NetworkInfo{net("default", "nat")})

	// 60x12 is the documented minimum terminal size; the body height handed to
	// panels after chrome is smaller still.
	view := ansi.Strip(m.DetailView(40, 6))

	s.Contains(view, "default")
}

// column returns the rune offset of sub within s, ignoring styling escapes.
func (s *modelSuite) column(line, sub string) int {
	s.T().Helper()
	i := strings.Index(line, sub)
	s.Require().GreaterOrEqualf(i, 0, "%q not found in %q", sub, line)
	return utf8.RuneCountInString(line[:i])
}

// A long name must not push MODE out of its column: the list has no mark
// gutter to absorb overflow into, so the column widths alone must hold.
func (s *modelSuite) TestRenderRowAlignsWithHeader() {
	m := New().SetItems([]backend.NetworkInfo{
		net("default", "nat"),
		net("a-very-long-network-name-that-overflows-its-column", "bridge"),
	})

	var header string
	var rows []string
	for _, ln := range strings.Split(ansi.Strip(m.ListView(80, 10)), "\n") {
		switch {
		case strings.Contains(ln, "MODE"):
			header = ln
		case strings.Contains(ln, "nat"), strings.Contains(ln, "bridge"):
			rows = append(rows, ln)
		}
	}
	s.Require().NotEmpty(header)
	s.Require().Len(rows, 2)

	want := s.column(header, "MODE")
	s.Equal(want, s.column(rows[0], "nat"))
	s.Equal(want, s.column(rows[1], "bridge"))
}

// The filter used to drop a space press (it arrives as "space") and every
// multi-byte rune (Key.String() is byte-lengthed), so a name containing a
// space could never be typed; backspace also trimmed single bytes.
func (s *modelSuite) TestFilterAcceptsSpacesAndUnicodeBackspaceTrimsWholeRunes() {
	m := New().SetItems([]backend.NetworkInfo{net("my net", "nat"), net("bridge0", "bridge")})

	m, _ = m.Update(keyMsg("/"))
	m, _ = m.Update(spaceKey())
	s.Require().Equal(" ", m.filter)

	for _, r := range "net" {
		m, _ = m.Update(keyMsg(string(r)))
	}
	s.Require().Equal(" net", m.filter)
	s.Require().Len(m.filtered, 1)
	s.Equal("my net", m.Selected().Name)

	m, _ = m.Update(keyMsg("é"))
	m, _ = m.Update(keyMsg("backspace"))
	s.Equal(" net", m.filter, "the multi-byte rune must be added and removed whole")
}
