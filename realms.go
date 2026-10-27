package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// Soulrend asks, every time Multiplayer is pressed, which of Craft Down Under's official realms to join (Soulrend
// Companion), and Quasar Companion lists their official servers above the player's own. Both mods are required by
// soulrend_core, so they stay on; installer 1.2.3 only sets their own options, the way a player could by hand:
//   - config/cdu_soulrend/preferences.json  skipSelector: true  — the realm screen's own "Don't show again"
//   - config/quasar_client/config.json      clusterName "" (no official server list is fetched), serversTitle = our name
// Nothing else in those files changes, and the first change keeps a .bak-installer copy.

func skipRealms(in Instance, title string) (string, error) {
	var notes []string

	pref := filepath.Join(in.Dir, "config", "cdu_soulrend", "preferences.json")
	m := map[string]any{}
	if b, err := os.ReadFile(pref); err == nil {
		_ = json.Unmarshal(b, &m) // a broken file is replaced, as the mod itself would
		if m == nil {
			m = map[string]any{}
		}
	}
	if m["skipSelector"] == true {
		notes = append(notes, "공식 서버 선택 화면은 이미 꺼져 있어요")
	} else {
		m["skipSelector"] = true
		if err := writeJSON(pref, m, "  "); err != nil {
			return "", err
		}
		notes = append(notes, "공식 서버 선택 화면 끔")
	}

	q := filepath.Join(in.Dir, "config", "quasar_client", "config.json")
	b, err := os.ReadFile(q)
	if err != nil {
		if os.IsNotExist(err) {
			return strings.Join(notes, " · "), nil // no Quasar settings in this profile: nothing to hide
		}
		return "", err
	}
	var root map[string]any
	if err := json.Unmarshal(b, &root); err != nil || root == nil {
		notes = append(notes, "Quasar 설정을 읽지 못해 공식 서버 목록은 그대로예요")
		return strings.Join(notes, " · "), nil
	}
	mp, _ := root["modPackConfig"].(map[string]any)
	if mp == nil {
		return strings.Join(notes, " · "), nil
	}
	changed := false
	if s, _ := mp["clusterName"].(string); s != "" {
		mp["clusterName"] = ""
		changed = true
	}
	if s, _ := mp["serversTitle"].(string); title != "" && s != title {
		mp["serversTitle"] = title
		changed = true
	}
	if !changed {
		notes = append(notes, "공식 서버 목록은 이미 숨겨져 있어요")
		return strings.Join(notes, " · "), nil
	}
	if _, err := os.Stat(q + ".bak-installer"); err != nil {
		_ = os.WriteFile(q+".bak-installer", b, 0o644)
	}
	if err := writeJSON(q, root, "\t"); err != nil {
		return "", err
	}
	notes = append(notes, "멀티플레이 목록에서 공식 서버 숨김")
	return strings.Join(notes, " · "), nil
}

func writeJSON(p string, v any, indent string) error {
	out, err := json.MarshalIndent(v, "", indent)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return writeAtomic(p, out)
}
