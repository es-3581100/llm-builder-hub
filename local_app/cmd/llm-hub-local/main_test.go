package main

import (
	"path/filepath"
	"testing"
)

func TestDefaultStatePathUsesExternalStateHome(t *testing.T) {
	stateHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateHome)
	got := defaultStatePath()
	want := filepath.Join(stateHome, "llm-hub", "workstation-state.json")
	if got != want {
		t.Fatalf("defaultStatePath()=%q want %q", got, want)
	}
}
