package main

import (
	"os"
	"path/filepath"
	"testing"
)

// The tests use the mod lists in testdata/ (copies of the ones in the data pack; EMBERLINE_PACK_DIR points at another
// folder with the same files).
func TestMain(m *testing.M) {
	dir := os.Getenv("EMBERLINE_PACK_DIR")
	if dir == "" {
		dir = "testdata"
	}
	read := func(n string) []byte {
		b, err := os.ReadFile(filepath.Join(dir, n))
		if err != nil {
			panic(err)
		}
		return b
	}
	extraModsJSON = read("extra_mods.json")
	retiredJSON = read("retired_mods.json")
	restoredJSON = read("restored_mods.json")
	everycompDefault = read("everycomp-hazardous.toml")
	emberlineVersion = "0.0.0-test"
	emberlineName = "emberline-0.0.0-test.jar"
	emberlineJar = []byte("test jar")
	supplementPack = []byte("test zip")
	os.Exit(m.Run())
}
