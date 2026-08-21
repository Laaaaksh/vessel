package ui

import (
	"fmt"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/suite"

	"github.com/Laaaaksh/vessel/internal/config"
)

const (
	testNoticeNameInspect = "inspect"
	testNoticeNameDeploy  = "deploy"
	testNoticeNameSync    = "sync"
	testNoticeUsableKey   = "z"
	testNoticeReservedKey = "j"
	testNoticeShiftedKey  = "shift+z"
	testNoticeCommand     = "container inspect {{.ID}}"
	testNoticeEchoCommand = "echo probe"
)

type ignoredKeysSuite struct {
	suite.Suite
	keys KeyMap
}

func TestIgnoredKeysSuite(t *testing.T) {
	suite.Run(t, new(ignoredKeysSuite))
}

func (s *ignoredKeysSuite) SetupTest() {
	s.keys = DefaultKeyMap()
}

func (s *ignoredKeysSuite) TestReservedKeyIsDetectedAndReported() {
	custom := []config.CustomCommand{{
		Name: testNoticeNameInspect, Key: testNoticeReservedKey, Command: testNoticeCommand,
	}}

	bad := unusableBindings(custom, s.keys)

	s.Len(bad, 1)
	s.Equal(unusableReserved, bad[0].reason)
	s.Equal(testNoticeReservedKey, bad[0].key)
	notice := ignoredKeysNotice(custom, s.keys)
	s.Contains(notice, testNoticeNameInspect)
	s.Contains(notice, fmt.Sprintf("%q", testNoticeReservedKey))
	s.Contains(notice, "is reserved")
	// Reporting classifies; it must not un-drop anything from dispatch.
	s.Empty(customKey(custom[0], s.keys))
}

func (s *ignoredKeysSuite) TestUnproducibleSpellingIsDetectedAndReported() {
	custom := []config.CustomCommand{{
		Name: testNoticeNameDeploy, Key: testNoticeShiftedKey, Command: testNoticeEchoCommand,
	}}

	bad := unusableBindings(custom, s.keys)

	s.Len(bad, 1)
	s.Equal(unusableUnproducible, bad[0].reason)
	s.Equal(testNoticeShiftedKey, bad[0].key)
	notice := ignoredKeysNotice(custom, s.keys)
	s.Contains(notice, testNoticeNameDeploy)
	s.Contains(notice, fmt.Sprintf("%q", testNoticeShiftedKey))
	s.Contains(notice, "matches no keypress")
	s.Empty(customKey(custom[0], s.keys))
}

func (s *ignoredKeysSuite) TestEmptyCommandIsDetectedAndReported() {
	custom := []config.CustomCommand{{
		Name: testNoticeNameSync, Key: testNoticeUsableKey, Command: "",
	}}

	bad := unusableBindings(custom, s.keys)

	s.Len(bad, 1)
	s.Equal(unusableNoCommand, bad[0].reason)
	notice := ignoredKeysNotice(custom, s.keys)
	s.Contains(notice, testNoticeNameSync)
	s.Contains(notice, "has no command")
	s.Empty(customKey(custom[0], s.keys))
}

func (s *ignoredKeysSuite) TestUsableConfigProducesNoStartupNotice() {
	custom := []config.CustomCommand{{
		Name: testNoticeNameInspect, Key: testNoticeUsableKey, Command: testNoticeCommand,
	}}

	s.Empty(unusableBindings(custom, s.keys))
	s.Empty(ignoredKeysNotice(custom, s.keys))
	s.NotEmpty(customKey(custom[0], s.keys)) // and it really dispatches
}

func (s *ignoredKeysSuite) TestDeliberatelyKeylessCommandStaysQuiet() {
	custom := []config.CustomCommand{{
		Name: testNoticeNameSync, Key: "", Command: testNoticeEchoCommand,
	}}

	// The example config documents a missing key as the action-menu-only form,
	// so it must not be flagged as broken.
	s.Empty(unusableBindings(custom, s.keys))
	s.Empty(ignoredKeysNotice(custom, s.keys))
	s.Empty(customKey(custom[0], s.keys))
}

func (s *ignoredKeysSuite) TestNoticeCountsAndNamesEveryOffendingBinding() {
	custom := []config.CustomCommand{
		{Name: testNoticeNameInspect, Key: testNoticeReservedKey, Command: testNoticeCommand},
		{Name: testNoticeNameDeploy, Key: testNoticeShiftedKey, Command: testNoticeEchoCommand},
		{Name: testNoticeNameSync, Key: testNoticeUsableKey, Command: ""},
	}

	notice := ignoredKeysNotice(custom, s.keys)

	s.Contains(notice, "3 custom commands ignore their keys")
	s.Contains(notice, testNoticeNameInspect)
	s.Contains(notice, testNoticeNameDeploy)
	s.Contains(notice, testNoticeNameSync)
}

func (s *ignoredKeysSuite) TestUnnamedOffendingCommandIsStillReported() {
	custom := []config.CustomCommand{{Key: testNoticeReservedKey, Command: testNoticeCommand}}

	notice := ignoredKeysNotice(custom, s.keys)

	s.Len(unusableBindings(custom, s.keys), 1)
	s.Contains(notice, "(unnamed)")
	s.Contains(notice, "is reserved")
}

func (s *ignoredKeysSuite) TestStartupSetsFooterStatusAtSmallestTerminal() {
	cfg := config.Config{CustomCommands: []config.CustomCommand{{
		Name: testNoticeNameInspect, Key: testNoticeReservedKey, Command: testNoticeCommand,
	}}}
	m := newModel(cfg)
	m.width, m.height = 60, 12

	s.NotEmpty(m.status)
	// The notice head fits inside 60 columns, so truncation can only cut the
	// detail tail, never the fact that something was dropped.
	footer := ansi.Strip(m.footerView())
	s.Contains(footer, "1 custom command ignores its key")

	// The footer must hold its single reserved row at the smallest supported
	// size, or every pane above loses a row of its budget.
	assertOneRow(s.T(), m, "startup ignored-binding notice")

	// Sidebar identity rows still render beside the notice.
	view := ansi.Strip(viewString(m.View()))
	s.Contains(view, "Containers")
	s.Contains(view, "Networks")
}

func (s *ignoredKeysSuite) TestStartupNoticeIsDismissedLikeAnyStatusMessage() {
	cfg := config.Config{CustomCommands: []config.CustomCommand{{
		Name: testNoticeNameInspect, Key: testNoticeReservedKey, Command: testNoticeCommand,
	}}}
	m := newModel(cfg)
	m.width, m.height = 100, 30
	startup := m.status
	s.NotEmpty(startup)

	next, _ := m.handleKey(keyMsg("+"))
	m = next.(Model)

	s.NotEqual(startup, m.status)
}
