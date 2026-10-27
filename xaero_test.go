package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHideXaeroMinimap(t *testing.T) {
	d := t.TempDir()
	in := Instance{Dir: d}
	// no Xaero: nothing happens
	if note, err := hideXaeroMinimap(in); err != nil || !strings.Contains(note, "없어요") {
		t.Fatalf("no xaero: %q %v", note, err)
	}
	_ = os.MkdirAll(filepath.Join(d, "mods"), 0o755)
	_ = os.WriteFile(filepath.Join(d, "mods", "xaerominimap-forge-1.20.1-26.6.0.jar"), []byte("x"), 0o644)
	prof := filepath.Join(d, "config", "xaero", "minimap", "profiles")
	// Xaero never started: try again later, no marker
	if note, _ := hideXaeroMinimap(in); !strings.Contains(note, "아직") {
		t.Fatalf("no profile: %q", note)
	}
	_ = os.MkdirAll(prof, 0o755)
	orig := "profile_name = Default\r\ndisplay_minimap = true\r\nminimap_item = -\r\n"
	_ = os.WriteFile(filepath.Join(prof, "default.cfg"), []byte(orig), 0o644)
	_ = os.WriteFile(filepath.Join(prof, "pvp.cfg"), []byte("display_minimap = false\n"), 0o644)
	note, err := hideXaeroMinimap(in)
	if err != nil || !strings.Contains(note, "1개") {
		t.Fatalf("first run: %q %v", note, err)
	}
	b, _ := os.ReadFile(filepath.Join(prof, "default.cfg"))
	if string(b) != "profile_name = Default\r\ndisplay_minimap = false\r\nminimap_item = -\r\n" {
		t.Fatalf("rewritten: %q", b)
	}
	if bk, _ := os.ReadFile(filepath.Join(prof, "default.cfg.bak_emberline")); string(bk) != orig {
		t.Fatalf("backup: %q", bk)
	}
	// the friend turns it back on in Xaero: the installer leaves it alone
	_ = os.WriteFile(filepath.Join(prof, "default.cfg"), []byte(orig), 0o644)
	if note, _ := hideXaeroMinimap(in); !strings.Contains(note, "이미") {
		t.Fatalf("second run: %q", note)
	}
	if b, _ := os.ReadFile(filepath.Join(prof, "default.cfg")); string(b) != orig {
		t.Fatalf("second run changed the file: %q", b)
	}
}
