package main

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

// CONNECTOR_SAMPLE = a copy of a working mods/.connector (with temp/ holding the original Fabric jars)
func TestConnectorCacheCheck(t *testing.T) {
	src := os.Getenv("CONNECTOR_SAMPLE")
	if src == "" {
		t.Skip("no sample")
	}
	good, _ := filepath.Glob(filepath.Join(src, "*_mapped_srg_*.jar"))
	if len(good) == 0 {
		t.Fatal("no sample jars")
	}
	for _, j := range good {
		if w := brokenConnectorJar(j); w != "" {
			t.Errorf("good jar flagged: %s: %s", filepath.Base(j), w)
		}
	}
	raw, _ := filepath.Glob(filepath.Join(src, "temp", "*.jar"))
	for _, j := range raw {
		if w := brokenConnectorJar(j); w == "" {
			t.Errorf("Fabric-form jar not flagged: %s", filepath.Base(j))
		} else {
			t.Logf("flagged as expected: %s: %s", filepath.Base(j), w)
		}
	}
	// a whole instance: good cache stays, broken cache is moved aside
	for _, broken := range []bool{false, true} {
		inst := t.TempDir()
		dir := filepath.Join(inst, "mods", ".connector")
		os.MkdirAll(dir, 0o755)
		for _, j := range good[:3] {
			copyT(t, j, filepath.Join(dir, filepath.Base(j)))
		}
		if broken {
			copyT(t, raw[0], filepath.Join(dir, "apoli_mapped_srg_1.20.1.jar"))
		}
		msg, err := checkConnectorCache(Instance{Dir: inst})
		note, _ := os.ReadFile(filepath.Join(dir, "apoli_mapped_srg_1.20.1.jar.input"))
		t.Logf("broken=%v -> %s (err %v) note=%q", broken, msg, err, note)
		if err != nil || broken != (len(note) > 0) {
			t.Errorf("broken=%v: note written=%v", broken, len(note) > 0)
		}
		if _, e := os.Stat(dir); e != nil {
			t.Errorf("cache folder must stay")
		}
	}
}

func copyT(t *testing.T, a, b string) {
	in, err := os.Open(a)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()
	out, err := os.Create(b)
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	io.Copy(out, in)
}
