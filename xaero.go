package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// 1.2.25: the 잔불 minimap is the server's minimap. Xaero's Minimap stays installed (the server needs it, see retire.go), but
// its own minimap display is switched off once — in Xaero's profile files, the same switch as Xaero's settings (미니맵 표시),
// so a friend who wants it back turns it on there and the installer leaves it alone from then on (a marker in config/emberline).
var xaeroDisplayLine = regexp.MustCompile(`(?m)^display_minimap\s*=\s*true([ \t]*\r?)$`)

func hideXaeroMinimap(in Instance) (string, error) {
	marker := filepath.Join(in.Dir, "config", "emberline", "xaero_minimap_off.done")
	if _, err := os.Stat(marker); err == nil {
		return "이미 잔불 미니맵으로 바꿨어요", nil
	}
	if !hasXaeroMinimap(in) {
		return "Xaero 미니맵이 없어요", nil
	}
	dir := filepath.Join(in.Dir, "config", "xaero", "minimap", "profiles")
	ents, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		// Xaero has not written its settings yet (never started): try again next time
		return "Xaero 설정이 아직 없어요 · 게임을 한 번 켠 뒤 다시 실행하면 바꿔요", nil
	}
	if err != nil {
		return "", err
	}
	n := 0
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".cfg") {
			continue
		}
		p := filepath.Join(dir, e.Name())
		b, err := os.ReadFile(p)
		if err != nil {
			return "", err
		}
		if !xaeroDisplayLine.Match(b) {
			continue
		}
		if err := os.WriteFile(p+".bak_emberline", b, 0o644); err != nil {
			return "", err
		}
		if err := os.WriteFile(p, xaeroDisplayLine.ReplaceAll(b, []byte("display_minimap = false$1")), 0o644); err != nil {
			return "", err
		}
		n++
	}
	if err := os.MkdirAll(filepath.Dir(marker), 0o755); err != nil {
		return "", err
	}
	_ = os.WriteFile(marker, []byte("Xaero's minimap display was switched off by the 잔불 installer "+appVersion+"\n"), 0o644)
	if n == 0 {
		return "Xaero 미니맵은 이미 꺼져 있어요", nil
	}
	return fmt.Sprintf("Xaero 미니맵 표시 끔 (설정 %d개) · 잔불 미니맵이 보여요", n), nil
}

func hasXaeroMinimap(in Instance) bool {
	ents, err := os.ReadDir(filepath.Join(in.Dir, "mods"))
	if err != nil {
		return false
	}
	for _, e := range ents {
		n := strings.ToLower(e.Name())
		if strings.HasPrefix(n, "xaerominimap") && strings.HasSuffix(n, ".jar") {
			return true
		}
	}
	return false
}
