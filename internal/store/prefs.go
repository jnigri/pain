package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

type Prefs struct {
	// CaseSensitive makes "a" and "A" different keys.
	CaseSensitive bool `json:"case_sensitive"`
	// SkipPunct lets you type past punctuation and spaces: typing the next
	// letter or digit fills in everything before it.
	SkipPunct bool `json:"skip_punctuation"`
	// SplitAsk shows questions and answers below the text instead of full screen.
	SplitAsk bool `json:"ask_split"`
	// Model answers questions, passed to claude as --model: an alias such as
	// "sonnet" or a full model name. Empty until chosen; ModelDefault leaves
	// it to Claude Code's own settings.
	Model string `json:"model,omitempty"`
	// Explain makes a checkpoint ask for the section in your own words
	// instead of asking questions about it.
	Explain bool `json:"checkpoint_explain"`
	// Active lists the folders and files read actively: a checkpoint ends
	// each section of the files under them.
	Active []string `json:"active_reading,omitempty"`
}

// covers tells whether path is dir or sits under it.
func covers(dir, path string) bool {
	sep := string(filepath.Separator)
	return path == dir || strings.HasPrefix(path, strings.TrimSuffix(dir, sep)+sep)
}

// IsActive tells whether path, or a folder above it, is read actively.
func (p Prefs) IsActive(path string) bool {
	return slices.ContainsFunc(p.Active, func(a string) bool { return covers(a, path) })
}

// ToggleActive turns active reading on for scope when path is not read
// actively, and otherwise off for everything that covers path. It returns the
// new state.
func (p *Prefs) ToggleActive(scope, path string) bool {
	if p.IsActive(path) {
		p.Active = slices.DeleteFunc(p.Active, func(a string) bool { return covers(a, path) })
		return false
	}
	p.Active = append(p.Active, scope)
	return true
}

// ModelDefault is the Model setting that passes no --model flag.
const ModelDefault = "default"

var defaultPrefs = Prefs{CaseSensitive: false, SkipPunct: true}

// PrefsPath follows XDG: $XDG_CONFIG_HOME/pain/config.json, else ~/.config/pain/config.json.
func PrefsPath() string {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "pain", "config.json")
}

func LoadPrefs() Prefs {
	p := defaultPrefs
	if data, err := os.ReadFile(PrefsPath()); err == nil {
		_ = json.Unmarshal(data, &p)
	}
	return p
}

func (p Prefs) Save() error {
	f := PrefsPath()
	if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(f, append(data, '\n'), 0o644)
}
