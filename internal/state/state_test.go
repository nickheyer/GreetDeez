package state

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveLoadRoundtrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	t.Setenv("GREETDEEZ_STATE_FILE", path)

	if err := Save(State{LastUser: "nick", LastSession: "sway"}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got := Load()
	if got.LastUser != "nick" || got.LastSession != "sway" {
		t.Fatalf("roundtrip mismatch: %+v", got)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("state file perms = %v want 0600", info.Mode().Perm())
	}
}

func TestLoadMissingGivesEmpty(t *testing.T) {
	t.Setenv("GREETDEEZ_STATE_FILE", filepath.Join(t.TempDir(), "nope.json"))
	if got := Load(); got != (State{}) {
		t.Fatalf("want empty state got %+v", got)
	}
}

func TestLoadCorruptGivesEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	t.Setenv("GREETDEEZ_STATE_FILE", path)
	if err := os.WriteFile(path, []byte("{not json"), 0600); err != nil {
		t.Fatal(err)
	}
	if got := Load(); got != (State{}) {
		t.Fatalf("want empty state got %+v", got)
	}
}
