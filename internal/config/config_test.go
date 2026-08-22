package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// writeConfig plants config.toml under an isolated XDG_CONFIG_HOME and points
// Load at it; Path() honors the env var, so no host dotfiles can leak in.
func writeConfig(t *testing.T, body string) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	if err := os.MkdirAll(filepath.Join(dir, "vessel"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "vessel", "config.toml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoad_missingFileReturnsDefaults(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load with no config file: %v", err)
	}
	want := Default()
	if cfg.PollInterval.Duration != want.PollInterval.Duration ||
		cfg.LogTailLines != want.LogTailLines ||
		cfg.Shell != want.Shell ||
		cfg.MouseEnabled != want.MouseEnabled {
		t.Fatalf("missing-file config = %+v, want defaults %+v", cfg, want)
	}
}

// A bad duration must surface as an error, and the returned config must still
// be safe to run: UnmarshalText stores ParseDuration's zero result before
// returning its error, so without sanitize the poller would panic on a
// non-positive ticker interval (observed live as "non-positive interval for
// NewTicker" taking down the whole dashboard).
func TestLoad_badDurationErrorsButConfigStaysSafe(t *testing.T) {
	writeConfig(t, "poll_interval = \"banana\"\n")

	cfg, err := Load()
	if err == nil {
		t.Fatal("Load accepted poll_interval = \"banana\", want an error")
	}
	if cfg.PollInterval.Duration <= 0 {
		t.Fatalf("PollInterval = %v after failed decode, want the %v default", cfg.PollInterval.Duration, defaultPollInterval)
	}
	if cfg.Shell != defaultShell || cfg.LogTailLines != defaultLogTailLines {
		t.Fatalf("config = %+v, want shell/tail defaults preserved", cfg)
	}
}

// Even a successfully parsed non-positive interval panics time.NewTicker, so
// sanitize must clamp it on the success path too.
func TestLoad_nonPositivePollIntervalClamped(t *testing.T) {
	cases := map[string]struct {
		body string
	}{
		"zero":     {body: "poll_interval = \"0s\"\n"},
		"negative": {body: "poll_interval = \"-3s\"\n"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			writeConfig(t, tc.body)

			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load(%q): %v", tc.body, err)
			}
			if cfg.PollInterval.Duration <= 0 {
				t.Fatalf("PollInterval = %v, want clamped to %v", cfg.PollInterval.Duration, defaultPollInterval)
			}
		})
	}
}

// Which user values survive a broken file is nondeterministic (BurntSushi
// applies decoded keys in map-iteration order and aborts at the first type
// error), so the only contract Load can make is: the error surfaces AND every
// safety-critical field is clamped, whatever else happened.
func TestLoad_brokenFileAlwaysYieldsSafeConfig(t *testing.T) {
	writeConfig(t, "log_tail_lines = 7\npoll_interval = \"nope\"\nshell = \"/bin/bash\"\n")

	cfg, err := Load()
	if err == nil {
		t.Fatal("partial decode returned no error")
	}
	if cfg.PollInterval.Duration <= 0 {
		t.Errorf("PollInterval = %v, want clamp to %v", cfg.PollInterval.Duration, defaultPollInterval)
	}
	if cfg.Shell == "" {
		t.Error("Shell = \"\", want non-empty")
	}
	if cfg.LogTailLines <= 0 {
		t.Errorf("LogTailLines = %d, want positive", cfg.LogTailLines)
	}
}

// Root cause pinned: ParseDuration returns a zero duration alongside its
// error and UnmarshalText stores it, so a failed decode leaves the field
// zeroed rather than untouched - this is why sanitize must run after every
// decode, not just successful ones.
func TestDuration_unmarshalErrorZeroesField(t *testing.T) {
	var d duration
	if err := d.UnmarshalText([]byte("banana")); err == nil {
		t.Fatal("expected an error for 'banana'")
	}
	if d.Duration != 0 {
		t.Fatalf("failed UnmarshalText left Duration = %v, want zeroed", d.Duration)
	}
}

func TestLoad_validFileRoundTrips(t *testing.T) {
	writeConfig(t, `
poll_interval = "5s"
log_tail_lines = 42
mouse_enabled = false
shell = "/bin/zsh"
confirm_stop = true

[[custom_commands]]
name = "inspect"
key = "i"
command = "container inspect {{.ID}}"
`)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("valid config failed to load: %v", err)
	}
	if cfg.PollInterval.Duration != 5*time.Second {
		t.Errorf("PollInterval = %v, want 5s", cfg.PollInterval.Duration)
	}
	if cfg.LogTailLines != 42 || !cfg.ConfirmStop || cfg.MouseEnabled || cfg.Shell != "/bin/zsh" {
		t.Errorf("config = %+v, want authored values kept unclamped", cfg)
	}
	if len(cfg.CustomCommands) != 1 || cfg.CustomCommands[0].Key != "i" {
		t.Errorf("custom commands = %+v, want one 'inspect' binding on i", cfg.CustomCommands)
	}
}
