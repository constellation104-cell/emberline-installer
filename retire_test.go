package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// CurseForge writes minecraftinstance.json minified; the root and the modpack also have isEnabled/fileNameOnDisk.
const cfJSON = `{"baseModLoader":{"name":"forge-47.4.26"},"installedModpack":{"fileNameOnDisk":"Soulrend-v8.5.2.zip","isEnabled":true},` +
	`"isEnabled":true,"allocatedMemory":10240,"installedAddons":[` +
	`{"addonID":1,"name":"Create","installedFile":{"fileName":"create-1.20.1-6.0.8.jar","isAvailable":true},"fileNameOnDisk":"create-1.20.1-6.0.8.jar","isEnabled":true},` +
	`{"addonID":2,"name":"Valkyrien Skies (Forge/Fabric)","installedFile":{"fileName":"valkyrienskies-120-2.4.11.jar","modules":[{"name":"x","isEnabled":true}]},"fileNameOnDisk":"valkyrienskies-120-2.4.11.jar","dateInstalled":"2026-10-25T09:20:02Z","isEnabled":true,"isLocked":false},` +
	`{"addonID":3,"name":"VS: Clockwork","installedFile":{"fileName":"clockwork-0.5.6.jar"},"fileNameOnDisk":"clockwork-0.5.6.jar.disabled","isEnabled":false},` +
	`{"addonID":4,"name":"Create: Central Kitchen","isEnabled":true,"installedFile":{"fileName":"create_central_kitchen-1.20.1-for-create-6.0.8-1.5.1.jar"},"fileNameOnDisk":"create_central_kitchen-1.20.1-for-create-6.0.8-1.5.1.jar"}` +
	`],"gameVersion":"1.20.1"}`

func TestRetire(t *testing.T) {
	dir := t.TempDir()
	mods := filepath.Join(dir, "mods")
	os.MkdirAll(mods, 0o755)
	for _, f := range []string{"create-1.20.1-6.0.8.jar", "valkyrienskies-120-2.4.11.jar", "clockwork-0.5.6.jar.disabled", "eureka-1201-1.6.3.jar",
		"create_central_kitchen-1.20.1-for-create-6.0.8-1.5.1.jar", "create_central_kitchen-1.20.1-for-create-6.0.8-1.5.1.jar.disabled"} {
		os.WriteFile(filepath.Join(mods, f), []byte(f), 0o644)
	}
	os.WriteFile(filepath.Join(dir, "minecraftinstance.json"), []byte(cfJSON), 0o644)
	note, err := retireMods(Instance{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	t.Log(note)
	for _, f := range []string{"valkyrienskies-120-2.4.11.jar", "eureka-1201-1.6.3.jar", "create_central_kitchen-1.20.1-for-create-6.0.8-1.5.1.jar"} {
		if _, err := os.Stat(filepath.Join(mods, f)); err == nil {
			t.Errorf("%s still enabled", f)
		}
		if _, err := os.Stat(filepath.Join(mods, f+".disabled")); err != nil {
			t.Errorf("%s.disabled missing", f)
		}
	}
	if _, err := os.Stat(filepath.Join(mods, "create_central_kitchen-1.20.1-for-create-6.0.8-1.5.1.jar.disabled.old-installer")); err != nil {
		t.Error("the earlier .disabled copy was not kept")
	}
	if _, err := os.Stat(filepath.Join(mods, "create-1.20.1-6.0.8.jar")); err != nil {
		t.Error("create was touched")
	}
	out, _ := os.ReadFile(filepath.Join(dir, "minecraftinstance.json"))
	var d map[string]any
	if err := json.Unmarshal(out, &d); err != nil {
		t.Fatal(err)
	}
	if d["isEnabled"] != true || d["installedModpack"].(map[string]any)["isEnabled"] != true {
		t.Error("root or modpack isEnabled changed")
	}
	got := map[string][2]any{}
	for _, a := range d["installedAddons"].([]any) {
		m := a.(map[string]any)
		got[m["name"].(string)] = [2]any{m["fileNameOnDisk"], m["isEnabled"]}
		if m["name"] == "Valkyrien Skies (Forge/Fabric)" {
			mod := m["installedFile"].(map[string]any)["modules"].([]any)[0].(map[string]any)
			if mod["isEnabled"] != true {
				t.Error("nested isEnabled changed")
			}
		}
	}
	want := map[string][2]any{
		"Create":                         {"create-1.20.1-6.0.8.jar", true},
		"Valkyrien Skies (Forge/Fabric)": {"valkyrienskies-120-2.4.11.jar.disabled", false},
		"VS: Clockwork":                  {"clockwork-0.5.6.jar.disabled", false},
		"Create: Central Kitchen":        {"create_central_kitchen-1.20.1-for-create-6.0.8-1.5.1.jar.disabled", false},
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s: got %v want %v", k, got[k], v)
		}
	}
	// nothing else moved: undo the four value changes and compare
	back := strings.NewReplacer(`"valkyrienskies-120-2.4.11.jar.disabled","dateInstalled":"2026-10-25T09:20:02Z","isEnabled":false`,
		`"valkyrienskies-120-2.4.11.jar","dateInstalled":"2026-10-25T09:20:02Z","isEnabled":true`,
		`"isEnabled":false,"installedFile":{"fileName":"create_central_kitchen-1.20.1-for-create-6.0.8-1.5.1.jar"},"fileNameOnDisk":"create_central_kitchen-1.20.1-for-create-6.0.8-1.5.1.jar.disabled"`,
		`"isEnabled":true,"installedFile":{"fileName":"create_central_kitchen-1.20.1-for-create-6.0.8-1.5.1.jar"},"fileNameOnDisk":"create_central_kitchen-1.20.1-for-create-6.0.8-1.5.1.jar"`).Replace(string(out))
	if back != cfJSON {
		t.Errorf("other bytes changed:\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(dir, "minecraftinstance.json.bak-retire")); err != nil {
		t.Error("no backup")
	}
	// second run: nothing left to do
	note2, err := retireMods(Instance{Dir: dir})
	if err != nil || note2 != "해당 없음" {
		t.Errorf("second run: %q %v", note2, err)
	}
}
