package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestScanRoots(t *testing.T) {
	d := t.TempDir()
	mk := func(p string) {
		if err := os.MkdirAll(filepath.Join(d, p), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	mk("Games/CF/Instances/Soulrend")
	mk("Games/CF/Install")
	mk("a/b/minecraft/Instances/x")
	mk("Windows/curseforge/minecraft/Instances") // skipped
	mk("deep/1/2/3/4/minecraft/Instances")       // too deep
	got := scanRoots([]string{d}, 3, 5*time.Second)
	want := map[string]bool{filepath.Join(d, "Games/CF/Instances"): true, filepath.Join(d, "a/b/minecraft/Instances"): true}
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for _, g := range got {
		if !want[g] {
			t.Fatalf("unexpected %s in %v", g, got)
		}
	}
	if r := rootFor(filepath.Join(d, "Games/CF")); r != filepath.Join(d, "Games/CF/Instances") {
		t.Fatalf("rootFor modding folder: %s", r)
	}
	os.WriteFile(filepath.Join(d, "Games/CF/Instances/Soulrend/minecraftinstance.json"), []byte("{}"), 0o644)
	if r := rootFor(filepath.Join(d, "Games/CF/Instances/Soulrend")); r != filepath.Join(d, "Games/CF/Instances") {
		t.Fatalf("rootFor profile: %s", r)
	}
}
