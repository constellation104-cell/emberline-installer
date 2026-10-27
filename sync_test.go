package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSyncConfigs(t *testing.T) {
	dir := os.Getenv("SYNC_DIR")
	if dir == "" {
		t.Skip("SYNC_DIR not set")
	}
	note, err := syncConfigs(Instance{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	t.Log(note)
	note2, err := syncConfigs(Instance{Dir: dir}) // second run changes nothing
	if err != nil || note2 != "이미 서버와 같아요" {
		t.Fatalf("second run: %q %v", note2, err)
	}
	// missing files: created
	empty := t.TempDir()
	if _, err := syncConfigs(Instance{Dir: empty}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(empty, "config", "everycomp-hazardous.toml")); err != nil {
		t.Fatal(err)
	}
}
