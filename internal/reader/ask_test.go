package reader

import (
	"slices"
	"testing"

	"pain/internal/store"
)

func TestAskArgsModel(t *testing.T) {
	orig := slices.Clone(claudeArgs)
	defer func() {
		if !slices.Equal(claudeArgs, orig) {
			t.Error("askArgs modified claudeArgs")
		}
	}()
	for _, model := range []string{"", store.ModelDefault} {
		if args := askArgs(model, "sys"); slices.Contains(args, "--model") {
			t.Errorf("askArgs(%q) passes --model: %q", model, args)
		}
	}
	args := askArgs("sonnet", "sys")
	i := slices.Index(args, "--model")
	if i < 0 || i+1 >= len(args) || args[i+1] != "sonnet" {
		t.Errorf("askArgs(sonnet) = %q, want --model sonnet", args)
	}
}

func TestNewPicker(t *testing.T) {
	if p := newPicker("", true); p.ids[p.sel] != store.ModelDefault {
		t.Errorf("unset model selects %q, want default", p.ids[p.sel])
	}
	if p := newPicker("opus", false); p.ids[p.sel] != "opus" {
		t.Errorf("selects %q, want opus", p.ids[p.sel])
	}
	p := newPicker("claude-sonnet-5-5", false)
	if p.ids[p.sel] != "claude-sonnet-5-5" || len(p.ids) != len(modelChoices)+1 {
		t.Errorf("hand-set model not offered: %q sel %d", p.ids, p.sel)
	}
}
