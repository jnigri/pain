package store

import (
	"encoding/json"
	"os"
	"path/filepath"
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
