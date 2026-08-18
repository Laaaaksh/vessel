package config

import (
	"os"
	"path/filepath"
	"time"

	"github.com/BurntSushi/toml"
)

// CustomCommand is a user-defined action bound to a key.
type CustomCommand struct {
	Name    string `toml:"name"`
	Key     string `toml:"key"`
	Command string `toml:"command"`
}

// Theme holds optional colour overrides (hex strings). Empty = defaults.
type Theme struct {
	SelectedBg string `toml:"selected_bg"`
	SelectedFg string `toml:"selected_fg"`
	Accent     string `toml:"accent"`
	StatusRun  string `toml:"status_running"`
	StatusStop string `toml:"status_stopped"`
}

// Config holds all user configuration for vessel.
type Config struct {
	PollInterval   duration        `toml:"poll_interval"`
	LogTailLines   int             `toml:"log_tail_lines"`
	MouseEnabled   bool            `toml:"mouse_enabled"`
	Shell          string          `toml:"shell"`
	ConfirmStop    bool            `toml:"confirm_stop"`
	CustomCommands []CustomCommand `toml:"custom_commands"`
	Theme          Theme           `toml:"theme"`
}

// duration is a TOML-friendly time.Duration wrapper.
type duration struct{ time.Duration }

func (d *duration) UnmarshalText(text []byte) error {
	var err error
	d.Duration, err = time.ParseDuration(string(text))
	return err
}

// Default returns the default configuration.
func Default() Config {
	return Config{
		PollInterval: duration{2 * time.Second},
		LogTailLines: 100,
		MouseEnabled: true,
		Shell:        "/bin/sh",
		ConfirmStop:  false,
	}
}

// Load reads the config from the standard location (~/.config/vessel/config.toml).
// If the file does not exist, Default() is returned with no error.
func Load() (Config, error) {
	cfg := Default()
	path := Path()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return cfg, nil
	}
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return cfg, err
	}
	if cfg.Shell == "" {
		cfg.Shell = "/bin/sh"
	}
	if cfg.LogTailLines <= 0 {
		cfg.LogTailLines = 100
	}
	return cfg, nil
}

// Path returns the path to the user config file.
func Path() string {
	if dir, ok := os.LookupEnv("XDG_CONFIG_HOME"); ok {
		return filepath.Join(dir, "vessel", "config.toml")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "vessel", "config.toml")
}
