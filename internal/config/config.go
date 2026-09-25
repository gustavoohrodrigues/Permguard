package config

import (
	"errors"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Language  string `yaml:"language"`
	StartPath string `yaml:"start_path"`
	Display   struct {
		Theme         string `yaml:"theme"`
		ColorMode     string `yaml:"color_mode"`
		Unicode       string `yaml:"unicode"`
		Borders       string `yaml:"borders"`
		CompactMode   bool   `yaml:"compact_mode"`
		ShowIcons     bool   `yaml:"show_icons"`
		ShowHelpBar   bool   `yaml:"show_help_bar"`
		ShowStatusBar bool   `yaml:"show_status_bar"`
	} `yaml:"display"`
	Security struct {
		RequireTypedConfirmation bool   `yaml:"require_typed_confirmation"`
		ConfirmationWord         string `yaml:"confirmation_word"`
		AllowReadOnlyMode        bool   `yaml:"allow_read_only_mode"`
		FollowSymlinksForRead    bool   `yaml:"follow_symlinks_for_read"`
		FollowSymlinksForWrite   bool   `yaml:"follow_symlinks_for_write"`
		AllowRecursiveOperations bool   `yaml:"allow_recursive_operations"`
	} `yaml:"security"`
	Audit struct {
		Enabled  bool   `yaml:"enabled"`
		UserPath string `yaml:"user_path"`
		RootPath string `yaml:"root_path"`
		Mode     string `yaml:"mode"`
	} `yaml:"audit"`
}

func Default() Config {
	var c Config
	c.Language = "pt-BR"
	c.StartPath = "."
	c.Display.Theme = "midnight"
	c.Display.ColorMode = "auto"
	c.Display.Unicode = "auto"
	c.Display.Borders = "rounded"
	c.Display.ShowIcons = true
	c.Display.ShowHelpBar = true
	c.Display.ShowStatusBar = true
	c.Security.AllowReadOnlyMode = true
	c.Security.RequireTypedConfirmation = true
	c.Security.ConfirmationWord = "ALTERAR"
	c.Audit.Enabled = true
	c.Audit.UserPath = "~/.local/state/permguard/audit.jsonl"
	c.Audit.RootPath = "/var/log/permguard/audit.jsonl"
	c.Audit.Mode = "0600"
	return c
}

func DefaultPath() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		if home, err := os.UserHomeDir(); err == nil {
			base = filepath.Join(home, ".config")
		}
	}
	return filepath.Join(base, "permguard", "config.yaml")
}

func Load(path string) (Config, error) {
	c := Default()
	if path == "" {
		path = DefaultPath()
	}
	data, err := os.ReadFile(path) // #nosec G304 -- caminho de configuração escolhido explicitamente pelo operador.
	if errors.Is(err, os.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return Config{}, err
	}
	if err := yaml.Unmarshal(data, &c); err != nil {
		return Config{}, err
	}
	if c.Language == "" {
		c.Language = "pt-BR"
	}
	return c, nil
}
