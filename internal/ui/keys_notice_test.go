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
	testNoticeOtherKey    = "w"
	testNoticeReservedKey = "j"
	testNoticeNetworksKey = "5"
	testNoticeShiftedKey  = "shift+z"
	testNoticeCommand     = "container inspect {{.ID}}"
	testNoticeEchoCommand = "echo probe"
	testNoticeOtherCmd    = "echo second"
	testNoticeViewRowKey  = "tab / 1 2 3 4 5"
	testNoticeViewRowDesc = "switch Containers / Images / Volumes / System / Networks"
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
	// Losing the key does not lose the command: the menu claim is true here.
	s.Contains(notice, "still in the action menu")
	// Reporting classifies; it must not un-drop anything from dispatch.
	s.Empty(customKey(custom[0], s.keys))
}

func (s *ignoredKeysSuite) TestNumericViewShortcutIsReserved() {
	custom := []config.CustomCommand{{
		Name: testNoticeNameInspect, Key: testNoticeNetworksKey, Command: testNoticeCommand,
	}}

	bad := unusableBindings(custom, s.keys)

	// handleKey answers 1-5 as view switches before it reaches a custom
	// command, so all five must classify as reserved rather than dispatch.
	s.Len(bad, 1)
	s.Equal(unusableReserved, bad[0].reason)
	s.Contains(ignoredKeysNotice(custom, s.keys), "is reserved")
	s.Empty(customKey(custom[0], s.keys))
}

func (s *ignoredKeysSuite) TestReservedNumericKeyNeverReachesCustomDispatch() {
	cfg := config.Config{CustomCommands: []config.CustomCommand{{
		Name: testNoticeNameInspect, Key: testNoticeNetworksKey, Command: testNoticeCommand,
	}}}
	m := newModel(cfg)
	m.width, m.height = 100, 30
	m.focus = FocusList

	next, _ := m.handleKey(keyMsg(testNoticeNetworksKey))
	m = next.(Model)

	s.Equal(ViewNetworks, m.activeView)

	// Help must neither advertise a key dispatch spends elsewhere nor drop the
	// view-switching row that key belongs to.
	base := []helpRow{{testNoticeViewRowKey, testNoticeViewRowDesc}}
	rows := withCustomBindings(base, s.keys, cfg.CustomCommands)
	s.Equal(base, rows)
}

func (s *ignoredKeysSuite) TestDuplicateKeyIsDetectedAndReported() {
	custom := []config.CustomCommand{
		{Name: testNoticeNameInspect, Key: testNoticeUsableKey, Command: testNoticeCommand},
		{Name: testNoticeNameDeploy, Key: testNoticeUsableKey, Command: testNoticeOtherCmd},
	}

	bad := unusableBindings(custom, s.keys)

	// First wins in dispatch, so the second entry is the dropped one.
	s.Len(bad, 1)
	s.Equal(testNoticeNameDeploy, bad[0].name)
	s.Equal(unusableDuplicate, bad[0].reason)
	notice := ignoredKeysNotice(custom, s.keys)
	s.Contains(notice, testNoticeNameDeploy)
	s.Contains(notice, "already taken by an earlier custom command")
	s.Contains(notice, "still in the action menu")
	// The report must not disturb which of the two actually fires.
	s.Equal(testNoticeCommand, customCommandFor(custom, s.keys, testNoticeUsableKey))
}

func (s *ignoredKeysSuite) TestDistinctKeysAreNotReportedAsDuplicates() {
	custom := []config.CustomCommand{
		{Name: testNoticeNameInspect, Key: testNoticeUsableKey, Command: testNoticeCommand},
		{Name: testNoticeNameDeploy, Key: testNoticeOtherKey, Command: testNoticeOtherCmd},
	}

	s.Empty(unusableBindings(custom, s.keys))
	s.Empty(ignoredKeysNotice(custom, s.keys))
}

func (s *ignoredKeysSuite) TestSeveralKeylessCommandsAreNotDuplicatesOfEachOther() {
	custom := []config.CustomCommand{
		{Name: testNoticeNameInspect, Command: testNoticeCommand},
		{Name: testNoticeNameDeploy, Command: testNoticeOtherCmd},
	}

	// Both run from the action menu only; sharing "no key" is not a clash.
	s.Empty(unusableBindings(custom, s.keys))
	s.Empty(ignoredKeysNotice(custom, s.keys))
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
	s.Contains(notice, "has no command to run")
	// An entry with nothing to run does not "still run from the action menu",
	// so the notice must not promise it does.
	s.NotContains(notice, "action menu")
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

	s.Contains(notice, "3 custom commands are ignored")
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
	s.Contains(footer, "1 custom command is ignored")

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
