package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestSkipRealms(t *testing.T) {
	dir := t.TempDir()
	q := filepath.Join(dir, "config", "quasar_client", "config.json")
	_ = os.MkdirAll(filepath.Dir(q), 0o755)
	orig := `{
	"modPackConfig": {"clusterName": "SOULDREND", "serversTitle": "Soulrend Servers", "welcomeTitle": "Use Code: Apoth", "welcomeMessage": "10% Rank Discounts", "servers": []},
	"chatConfig": {"useQChat": true},
	"tabListConfig": {"showNetwork": true, "size": 5},
	"collapsedTabs": ["a"]
}`
	if err := os.WriteFile(q, []byte(orig), 0o644); err != nil {
		t.Fatal(err)
	}
	note, err := skipRealms(Instance{Dir: dir}, "잔불 서버")
	if err != nil {
		t.Fatal(err)
	}
	t.Log(note)
	var pref map[string]any
	b, _ := os.ReadFile(filepath.Join(dir, "config", "cdu_soulrend", "preferences.json"))
	if json.Unmarshal(b, &pref) != nil || pref["skipSelector"] != true {
		t.Fatalf("preferences: %s", b)
	}
	var root map[string]any
	b, _ = os.ReadFile(q)
	if err := json.Unmarshal(b, &root); err != nil {
		t.Fatal(err)
	}
	mp := root["modPackConfig"].(map[string]any)
	if mp["clusterName"] != "" || mp["serversTitle"] != "잔불 서버" || mp["welcomeTitle"] != "Use Code: Apoth" {
		t.Fatalf("quasar: %s", b)
	}
	if root["chatConfig"].(map[string]any)["useQChat"] != true || root["tabListConfig"].(map[string]any)["size"].(float64) != 5 || len(root["collapsedTabs"].([]any)) != 1 {
		t.Fatalf("other settings changed: %s", b)
	}
	if bk, _ := os.ReadFile(q + ".bak-installer"); string(bk) != orig {
		t.Fatal("no backup of the original")
	}
	// second run: nothing to do, the backup stays the original
	note2, err := skipRealms(Instance{Dir: dir}, "잔불 서버")
	if err != nil {
		t.Fatal(err)
	}
	t.Log(note2)
	if bk, _ := os.ReadFile(q + ".bak-installer"); string(bk) != orig {
		t.Fatal("backup overwritten")
	}
	// a profile without Quasar settings
	if _, err := skipRealms(Instance{Dir: t.TempDir()}, "x"); err != nil {
		t.Fatal(err)
	}
}
