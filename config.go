package main

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Two settings have to match the server:
//   - Every Compat's module blacklist decides which blocks exist; if the client and server differ, joining fails.
//     The server turns off the Macaw's wood variants (they made the game slow to load).
//   - Radium's POI mixin stalls Valkyrien Skies ships, so the server switches it off; the client does the same.

// everycompDefault (the server's Every Compat file) comes from the data pack (pack.go).

const mcwBlacklist = `["mcwbridges", "mcwdoors", "mcwfences", "mcwfurnitures", "mcwlights", "mcwpaths", "mcwroofs", "mcwtrpdoors", "mcwwindows", "mcwstairs"]`

// backupOnce keeps the friend's original file the first time the installer changes it.
func backupOnce(p string) {
	b := p + ".bak-installer"
	if _, err := os.Stat(b); err == nil {
		return
	}
	if data, err := os.ReadFile(p); err == nil {
		_ = os.WriteFile(b, data, 0o644)
	}
}

var poiLine = regexp.MustCompile(`^\s*mixin\.ai\.poi\s*=`)
var blacklistLine = regexp.MustCompile(`^(\s*blacklist\s*=\s*)(.*)$`)

func syncConfigs(in Instance) (string, error) {
	dir := filepath.Join(in.Dir, "config")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	changed := 0

	// Radium (lithium.properties): mixin.ai.poi=false
	lp := filepath.Join(dir, "lithium.properties")
	cur, _ := os.ReadFile(lp)
	s := string(cur)
	nl := "\n"
	if strings.Contains(s, "\r\n") || s == "" {
		nl = "\r\n"
	}
	lines := strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
	have := false
	var out []string
	for _, l := range lines {
		if poiLine.MatchString(l) {
			if strings.TrimSpace(strings.SplitN(l, "=", 2)[1]) == "false" && !have {
				have = true
				out = append(out, l)
			}
			continue // a different value, or a duplicate, goes away
		}
		out = append(out, l)
	}
	if !have {
		for len(out) > 0 && out[len(out)-1] == "" {
			out = out[:len(out)-1]
		}
		out = append(out, "mixin.ai.poi=false", "")
		backupOnce(lp)
		if err := writeAtomic(lp, []byte(strings.Join(out, nl))); err != nil {
			return "", err
		}
		changed++
	}

	// Shoulder Surfing (shouldersurfing-client.toml): it hides or fades your own character when the camera gets close to
	// it, and small origins (the pixie is 1/6 or 1/2 size) always count as close, so they could not see themselves.
	if n, err := fixShoulderSurfing(filepath.Join(dir, "shouldersurfing-client.toml")); err != nil {
		return "", err
	} else {
		changed += n
	}

	// Every Compat (everycomp-hazardous.toml): [module] blacklist
	ep := filepath.Join(dir, "everycomp-hazardous.toml")
	cur2, err := os.ReadFile(ep)
	if err != nil { // the game has not made it yet: the server's own copy
		if err := writeAtomic(ep, everycompDefault); err != nil {
			return "", err
		}
		return "설정을 서버와 맞춤 (블록 · 카메라)", nil
	}
	t := string(cur2)
	nl = "\n"
	if strings.Contains(t, "\r\n") {
		nl = "\r\n"
	}
	tl := strings.Split(strings.ReplaceAll(t, "\r\n", "\n"), "\n")
	inModule, done := false, false
	for i, l := range tl {
		trim := strings.TrimSpace(l)
		if strings.HasPrefix(trim, "[") && strings.HasSuffix(trim, "]") && !strings.HasPrefix(trim, "#") && !strings.Contains(trim, "=") {
			inModule = trim == "[module]"
			continue
		}
		if !inModule || strings.HasPrefix(trim, "#") {
			continue
		}
		if m := blacklistLine.FindStringSubmatch(l); m != nil {
			v := strings.TrimSpace(m[2])
			if !strings.HasPrefix(v, "[") || !strings.HasSuffix(v, "]") {
				return "", errors.New("Every Compat 설정이 여러 줄이라 못 고쳤어요. 관리자에게 알려 주세요")
			}
			if v != mcwBlacklist {
				tl[i] = m[1] + mcwBlacklist
				backupOnce(ep)
				if err := writeAtomic(ep, []byte(strings.Join(tl, nl))); err != nil {
					return "", err
				}
				changed++
			}
			done = true
			break
		}
	}
	if !done {
		return "", errors.New("Every Compat 설정에서 [module] 항목을 찾지 못했어요. 관리자에게 알려 주세요")
	}
	if changed == 0 {
		return "이미 서버와 같아요", nil
	}
	return "설정을 서버와 맞춤 (블록 · 카메라)", nil
}

var ssrKeep = regexp.MustCompile(`^(\s*keep_camera_out_of_head_distance_multiplier\s*=\s*)(\S+)\s*$`)
var ssrFade = regexp.MustCompile(`^(\s*adjust_transparency\s*=\s*)(\S+)\s*$`)

// fixShoulderSurfing sets the two values in place (everything else in the file stays as the player has it). A game that
// never ran has no file yet: a small one with just these two goes in, and the mod fills in the rest when it starts.
func fixShoulderSurfing(p string) (int, error) {
	cur, err := os.ReadFile(p)
	if err != nil {
		if !os.IsNotExist(err) {
			return 0, err
		}
		body := "[camera]\n\tkeep_camera_out_of_head_distance_multiplier = 0.0\n\n[player]\n\n\t[player.transparency]\n\t\tadjust_transparency = false\n"
		return 1, writeAtomic(p, []byte(body))
	}
	t := string(cur)
	nl := "\n"
	if strings.Contains(t, "\r\n") {
		nl = "\r\n"
	}
	lines := strings.Split(strings.ReplaceAll(t, "\r\n", "\n"), "\n")
	edits := 0
	for i, l := range lines {
		if m := ssrKeep.FindStringSubmatch(l); m != nil && m[2] != "0.0" {
			lines[i] = m[1] + "0.0"
			edits++
		} else if m := ssrFade.FindStringSubmatch(l); m != nil && m[2] != "false" {
			lines[i] = m[1] + "false"
			edits++
		}
	}
	if edits == 0 {
		return 0, nil
	}
	backupOnce(p)
	return 1, writeAtomic(p, []byte(strings.Join(lines, nl)))
}
