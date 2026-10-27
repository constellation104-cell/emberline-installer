package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 1.2.19: Entity Culling 1.11.3 (part of the pack, tracked by CurseForge) next to the pinned 1.10.5 must end up as
// .disabled AND marked off in minecraftinstance.json, or CurseForge downloads it back and the game picks the higher one.
func TestSwitchOffOtherVersions(t *testing.T) {
	dir := t.TempDir()
	mods := filepath.Join(dir, "mods")
	_ = os.MkdirAll(mods, 0o755)
	write := func(p, s string) {
		if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(mods, "entityculling-forge-1.11.3-mc1.20.1.jar"), "new")
	write(filepath.Join(mods, "entityculling-forge-1.10.5-mc1.20.1.jar"), "pinned")
	write(filepath.Join(mods, "entityculling-forge-1.11.3-mc1.20.1.jar.old-installer"), "left by 1.2.17")
	write(filepath.Join(mods, "other-mod-1.0.jar"), "x")
	inst := `{
  "name": "Soulrend",
  "installedAddons": [
    {
      "addonID": 448233,
      "isEnabled": true,
      "installedFile": {
        "fileName": "entityculling-forge-1.11.3-mc1.20.1.jar",
        "fileNameOnDisk": "entityculling-forge-1.11.3-mc1.20.1.jar"
      },
      "fileNameOnDisk": "entityculling-forge-1.11.3-mc1.20.1.jar"
    },
    {
      "addonID": 1,
      "isEnabled": true,
      "fileNameOnDisk": "other-mod-1.0.jar"
    }
  ]
}`
	write(filepath.Join(dir, "minecraftinstance.json"), inst)
	m := ExtraMod{Filename: "entityculling-forge-1.10.5-mc1.20.1.jar", Prefix: "entityculling-forge-", Title: "Entity Culling"}
	ents, _ := os.ReadDir(mods)
	off, failed := switchOffOtherVersions(mods, m, ents)
	if len(failed) != 0 || len(off) != 1 || off["entityculling-forge-1.11.3-mc1.20.1.jar"] != "entityculling-forge-1.11.3-mc1.20.1.jar.disabled" {
		t.Fatalf("off=%v failed=%v", off, failed)
	}
	if err := markAddons(filepath.Join(dir, "minecraftinstance.json"), off, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(mods, "entityculling-forge-1.11.3-mc1.20.1.jar")); err == nil {
		t.Fatal("1.11.3 jar still there")
	}
	for _, keep := range []string{"entityculling-forge-1.10.5-mc1.20.1.jar", "entityculling-forge-1.11.3-mc1.20.1.jar.disabled", "entityculling-forge-1.11.3-mc1.20.1.jar.old-installer", "other-mod-1.0.jar"} {
		if _, err := os.Stat(filepath.Join(mods, keep)); err != nil {
			t.Fatalf("missing %s", keep)
		}
	}
	raw, _ := os.ReadFile(filepath.Join(dir, "minecraftinstance.json"))
	s := string(raw)
	if !strings.Contains(s, `"fileNameOnDisk": "entityculling-forge-1.11.3-mc1.20.1.jar.disabled"`) || strings.Count(s, `"isEnabled": false`) != 1 {
		t.Fatalf("CurseForge list not marked:\n%s", s)
	}
	if !strings.Contains(s, `"fileNameOnDisk": "other-mod-1.0.jar"`) {
		t.Fatal("other add-on changed")
	}
	// running again changes nothing
	ents, _ = os.ReadDir(mods)
	off, failed = switchOffOtherVersions(mods, m, ents)
	if len(off) != 0 || len(failed) != 0 {
		t.Fatalf("second run: off=%v failed=%v", off, failed)
	}
}

func TestAllowedURL(t *testing.T) {
	for u, want := range map[string]bool{
		"https://cdn.modrinth.com/data/x/versions/y/a.jar":               true,
		"https://edge.forgecdn.net/files/8433/11/craftbrowser-1.0.0.jar": true,
		"https://mediafilez.forgecdn.net/files/8433/11/a.jar":            false,
		"http://cdn.modrinth.com/data/a.jar":                             false,
		"https://edge.forgecdn.net.evil.example/files/a.jar":             false,
	} {
		if allowedURL(u) != want {
			t.Errorf("allowedURL(%q) = %v, want %v", u, !want, want)
		}
	}
	seen := map[string]bool{}
	for _, m := range extraMods() {
		if !allowedURL(m.URL) || len(m.SHA512) != 128 || m.Size <= 0 || m.Prefix == "" || seen[m.Slug] {
			t.Errorf("bad extra mod entry %+v", m.Slug)
		}
		seen[m.Slug] = true
	}
	if !seen["mcef"] || !seen["craftbrowser"] {
		t.Error("mcef / craftbrowser missing from extra_mods.json")
	}
}

func TestCraftBrowserKeys(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "options.txt")
	if err := os.WriteFile(p, []byte("version:3465\r\nkey_key.craftbrowser.toggle_browser:key.keyboard.g\r\nkey_key.emberline.open:key.keyboard.p\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := setOptions(dir, nil); err != nil {
		t.Fatal(err)
	}
	if err := setOptions(dir, nil); err != nil { // twice: nothing doubles
		t.Fatal(err)
	}
	b, _ := os.ReadFile(p)
	s := string(b)
	for _, want := range []string{"key_key.craftbrowser.toggle_browser:key.keyboard.g", "key_key.craftbrowser.toggle_pip:key.keyboard.f8",
		"key_key.craftbrowser.panel_lock:key.keyboard.f9", "key_key.emberline.open:key.keyboard.p", "lang:ko_kr"} {
		if strings.Count(s, want) != 1 {
			t.Errorf("want %q once in\n%s", want, s)
		}
	}
	if strings.Contains(s, "toggle_browser:key.keyboard.f6") {
		t.Error("a key the friend chose was changed")
	}
}
