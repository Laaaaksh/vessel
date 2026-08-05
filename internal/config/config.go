package config

import (
	"os"
	"path/filepath"
	"time"

	"github.com/BurntSushi/toml"
)

// Config holds all user configuration for vessel.
type Config struct {
	PollInterval duration `toml:"poll_interval"`
	LogTailLines int      `toml:"log_tail_lines"`
	MouseEnabled bool     `toml:"mouse_enabled"`
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
	}
}

// Load reads the config from the standard location (~/.config/vessel/config.toml).
// If the file does not exist, Default() is returned with no error.
func Load() (Config, error) {
	cfg := Default()
	path := configPath()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return cfg, nil
	}
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func configPath() string {
	if dir, ok := os.LookupEnv("XDG_CONFIG_HOME"); ok {
		return filepath.Join(dir, "vessel", "config.toml")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "vessel", "config.toml")
}
