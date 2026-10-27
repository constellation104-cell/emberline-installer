package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Mods installer 1.2.0 added that the server no longer has (2026-10-25): Valkyrien Skies 2 clashes with Epic Fight in
// the client's renderer (the game stops while starting), Clockwork and Eureka need it, and Create: Central Kitchen
// needs a newer Farmer's Delight than Soulrend's. They are switched off the way CurseForge does it itself — the file
// becomes <name>.disabled and the profile's minecraftinstance.json marks the add-on as off — because CurseForge
// downloads a tracked add-on again when only its file disappears.
// 1.2.4: Xaero's Minimap and World Map are NOT retired any more. 1.2.2 and 1.2.3 switched them off, but both register mob
// effects on the server, and Forge refuses a client that lacks a synced registry entry ("Failed to synchronize registry
// data"). Since the server keeps them, restoreMods turns them back on wherever an earlier installer switched them off.

// retiredJSON and restoredJSON come from the data pack (pack.go).

type RetiredMod struct {
	Filename string `json:"filename"`
	Title    string `json:"title"`
}

func retiredMods() []RetiredMod {
	var l []RetiredMod
	_ = json.Unmarshal(retiredJSON, &l)
	return l
}

func restoredMods() []RetiredMod {
	var l []RetiredMod
	_ = json.Unmarshal(restoredJSON, &l)
	return l
}

// restoreMods undoes an earlier installer's retirement: <name>.disabled back to <name>, and on in CurseForge's list.
// Only for the mods in restored_mods.json, and only when the plain jar is not already there.
func restoreMods(in Instance) (string, error) {
	mods := filepath.Join(in.Dir, "mods")
	var on []string
	rename := map[string]string{}
	for _, r := range restoredMods() {
		p := filepath.Join(mods, r.Filename)
		if _, err := os.Stat(p); err == nil {
			continue // already on (CurseForge or the player turned it back on)
		}
		// turned off by an earlier installer: as <name>.disabled (retired), or as <name>.old-installer (an extra mod
		// replaced it with another version, e.g. AzureLib 3.1.24 for 3.1.15 in 1.2.8 and 1.2.9)
		dis := p + ".disabled"
		suffix := ".disabled"
		if _, err := os.Stat(dis); err != nil {
			dis = p + ".old-installer"
			suffix = ".old-installer"
			if _, err := os.Stat(dis); err != nil {
				continue
			}
		}
		if err := os.Rename(dis, p); err != nil {
			return "", fmt.Errorf("%s 켜기 실패: %v", r.Title, err)
		}
		on = append(on, r.Title)
		if suffix == ".disabled" {
			rename[r.Filename+".disabled"] = r.Filename
		}
	}
	if len(on) == 0 {
		return "해당 없음", nil
	}
	if err := markAddons(filepath.Join(in.Dir, "minecraftinstance.json"), rename, true); err != nil {
		return "", fmt.Errorf("CurseForge 목록 고치기 실패 (파일은 켰어요): %v", err)
	}
	return strings.Join(on, ", ") + " 다시 켬", nil
}

func retireMods(in Instance) (string, error) {
	mods := filepath.Join(in.Dir, "mods")
	var off []string
	var files []string
	for _, r := range retiredMods() {
		p := filepath.Join(mods, r.Filename)
		if _, err := os.Stat(p); err != nil {
			continue // not there (never installed, or already off)
		}
		dis := p + ".disabled"
		if _, err := os.Stat(dis); err == nil {
			_ = os.Rename(dis, dis+".old-installer") // an earlier copy, kept rather than deleted
		}
		if err := os.Rename(p, dis); err != nil {
			return "", fmt.Errorf("%s 끄기 실패: %v", r.Title, err)
		}
		off = append(off, r.Title)
		files = append(files, r.Filename)
	}
	if len(files) == 0 {
		return "해당 없음", nil
	}
	rename := map[string]string{}
	for _, f := range files {
		rename[f] = f + ".disabled"
	}
	if err := markAddons(filepath.Join(in.Dir, "minecraftinstance.json"), rename, false); err != nil {
		return "", fmt.Errorf("CurseForge 목록 고치기 실패 (파일은 껐어요): %v", err)
	}
	return strings.Join(off, ", ") + " 끔", nil
}

// markAddons renames add-ons in CurseForge's profile file (fileNameOnDisk old -> new) and sets their isEnabled. Only those
// two values change, in place, so the rest of the file stays byte for byte what CurseForge wrote.
func markAddons(path string, rename map[string]string, enabled bool) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	edits, err := addonEdits(raw, rename, enabled)
	if err != nil {
		return err
	}
	if len(edits) == 0 {
		return nil // CurseForge does not track them in this profile
	}
	sort.Slice(edits, func(i, j int) bool { return edits[i].start > edits[j].start })
	out := append([]byte(nil), raw...)
	for _, e := range edits {
		out = append(out[:e.start], append([]byte(e.text), out[e.end:]...)...)
	}
	var check any
	if err := json.Unmarshal(out, &check); err != nil {
		return errors.New("고친 파일이 JSON이 아니에요: " + err.Error())
	}
	bak := path + ".bak-retire"
	if _, err := os.Stat(bak); err != nil {
		_ = os.WriteFile(bak, raw, 0o644)
	}
	return writeAtomic(path, out)
}

type edit struct {
	start, end int
	text       string
}

// addonEdits walks the JSON once and, for every element of the top-level "installedAddons" array whose
// "fileNameOnDisk" is one of `want`, returns replacements for its "isEnabled" and "fileNameOnDisk" values.
func addonEdits(raw []byte, rename map[string]string, enabled bool) ([]edit, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	// value start: skip whitespace, ':' and ',' after the previous token
	startAt := func(off int64) int {
		i := int(off)
		for i < len(raw) && (raw[i] == ' ' || raw[i] == '\n' || raw[i] == '\r' || raw[i] == '\t' || raw[i] == ':' || raw[i] == ',') {
			i++
		}
		return i
	}
	type addon struct {
		disk                string
		diskS, diskE        int
		enS, enE            int
		hasDisk, hasEnabled bool
	}
	var out []edit
	depth := 0
	inAddons := false
	addonsDepth := 0
	var cur *addon
	expectKey := false
	var key string
	keyDepth := 0
	// stack of containers: true = object, false = array
	var stack []bool
	finish := func(a *addon) {
		if a == nil || !a.hasDisk {
			return
		}
		to, ok := rename[a.disk]
		if !ok {
			return
		}
		out = append(out, edit{a.diskS, a.diskE, jsonString(to)})
		if a.hasEnabled {
			out = append(out, edit{a.enS, a.enE, strconv.FormatBool(enabled)})
		}
	}
	for {
		before := dec.InputOffset()
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		after := int(dec.InputOffset())
		inObj := len(stack) > 0 && stack[len(stack)-1]
		switch t := tok.(type) {
		case json.Delim:
			switch t {
			case '{', '[':
				if inObj && !expectKey {
					// a container value for `key`
					if depth == 1 && key == "installedAddons" && t == '[' {
						inAddons = true
						addonsDepth = depth + 1
					}
				}
				stack = append(stack, t == '{')
				depth++
				if inAddons && depth == addonsDepth+1 && t == '{' {
					cur = &addon{}
				}
				expectKey = t == '{'
			case '}', ']':
				if inAddons && depth == addonsDepth+1 && t == '}' {
					finish(cur)
					cur = nil
				}
				if inAddons && depth == addonsDepth && t == ']' {
					inAddons = false
				}
				stack = stack[:len(stack)-1]
				depth--
				expectKey = len(stack) > 0 && stack[len(stack)-1]
			}
			continue
		}
		if inObj && expectKey {
			key, _ = tok.(string)
			keyDepth = depth
			expectKey = false
			continue
		}
		// a scalar value
		if cur != nil && inObj && keyDepth == addonsDepth+1 && depth == addonsDepth+1 {
			s := startAt(before)
			switch key {
			case "fileNameOnDisk":
				if v, ok := tok.(string); ok {
					cur.disk, cur.diskS, cur.diskE, cur.hasDisk = v, s, after, true
				}
			case "isEnabled":
				if _, ok := tok.(bool); ok {
					cur.enS, cur.enE, cur.hasEnabled = s, after, true
				}
			}
		}
		if inObj {
			expectKey = true
		}
	}
	return out, nil
}

func jsonString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
