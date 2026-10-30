package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Program identifies a process. Match is either a plain executable name
// ("Nextcloud.exe", "nextcloud") or "flatpak:<app-id>".
type Program struct {
	Label string `json:"label"`
	Match string `json:"match"`
}

func (p Program) Display() string {
	if p.Label == "" || strings.EqualFold(p.Label, p.Match) {
		return p.Match
	}
	return p.Label + "  (" + p.Match + ")"
}

type Rule struct {
	Enabled  bool      `json:"enabled"`
	Target   Program   `json:"target"`
	Triggers []Program `json:"triggers"`
}

type Config struct {
	Rules         []Rule `json:"rules"`
	Notifications bool   `json:"notifications"`
	Paused        bool   `json:"paused"`
}

var (
	cfgMu sync.Mutex
	cfg   = Config{Notifications: true}
	// cfgVersion increases with every change, so pollers can skip unchanged configs.
	cfgVersion int
)

func exeDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	if r, err := filepath.EvalSymlinks(exe); err == nil {
		exe = r
	}
	return filepath.Dir(exe)
}

func configPath() string {
	p := filepath.Join(exeDir(), "AppCloser-Settings.json")
	// Migrate the settings file of earlier (German) versions.
	if _, err := os.Stat(p); os.IsNotExist(err) {
		os.Rename(filepath.Join(exeDir(), "AppCloser-Einstellungen.json"), p)
	}
	return p
}

func loadConfig() {
	data, err := os.ReadFile(configPath())
	if err != nil {
		return
	}
	var c Config
	if json.Unmarshal(data, &c) == nil {
		cfgMu.Lock()
		cfg = c
		cfgVersion++
		cfgMu.Unlock()
	}
}

var (
	lastMod   time.Time
	lastModMu sync.Mutex
)

// reloadIfChanged re-reads the config file when another process saved it.
func reloadIfChanged() {
	lastModMu.Lock()
	defer lastModMu.Unlock()
	st, err := os.Stat(configPath())
	if err != nil || st.ModTime().Equal(lastMod) {
		return
	}
	lastMod = st.ModTime()
	loadConfig()
}

func saveConfig() error {
	cfgMu.Lock()
	data, err := json.MarshalIndent(cfg, "", "  ")
	cfgMu.Unlock()
	if err != nil {
		return err
	}
	tmp := configPath() + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, configPath())
}

// snapshot returns a deep copy of the config for use outside the lock.
func snapshot() Config {
	cfgMu.Lock()
	defer cfgMu.Unlock()
	c := cfg
	c.Rules = make([]Rule, len(cfg.Rules))
	for i, r := range cfg.Rules {
		r.Triggers = append([]Program(nil), r.Triggers...)
		c.Rules[i] = r
	}
	return c
}

func updateConfig(f func(c *Config)) {
	cfgMu.Lock()
	f(&cfg)
	cfgVersion++
	cfgMu.Unlock()
	saveConfig()
}

func configVersion() int {
	cfgMu.Lock()
	defer cfgMu.Unlock()
	return cfgVersion
}
