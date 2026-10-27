package main

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

// CurseForge lets players move its "modding folder" anywhere (Settings → Minecraft). That folder holds "Instances" next
// to "Install"; the default one sits at <home>\curseforge\minecraft. scanRoots walks a few levels below each start folder
// looking for either shape and returns the Instances folders it finds. It stops at the time budget so the window opens fast.
var scanSkip = map[string]bool{
	"windows": true, "program files": true, "program files (x86)": true, "programdata": true, "$recycle.bin": true,
	"system volume information": true, "appdata": true, "recovery": true, "perflogs": true, "msocache": true,
	"node_modules": true, "steamapps": true, "windowsapps": true, "$windows.~bt": true, "$windows.~ws": true,
}

func scanRoots(starts []string, maxDepth int, budget time.Duration) []string {
	deadline := time.Now().Add(budget)
	type item struct {
		dir   string
		depth int
	}
	var queue []item
	for _, s := range starts {
		queue = append(queue, item{s, 0})
	}
	var out []string
	seen := map[string]bool{}
	visited := 0
	for len(queue) > 0 && visited < 60000 && time.Now().Before(deadline) {
		it := queue[0]
		queue = queue[1:]
		ents, err := os.ReadDir(it.dir)
		if err != nil {
			continue
		}
		visited++
		inst, install := "", false
		for _, e := range ents {
			if !e.IsDir() {
				continue
			}
			switch strings.ToLower(e.Name()) {
			case "instances":
				inst = filepath.Join(it.dir, e.Name())
			case "install":
				install = true
			}
		}
		if inst != "" && (install || strings.EqualFold(filepath.Base(it.dir), "minecraft")) {
			if k := strings.ToLower(filepath.Clean(inst)); !seen[k] {
				seen[k] = true
				out = append(out, inst)
			}
			continue // nothing more to find inside a CurseForge folder
		}
		if it.depth >= maxDepth {
			continue
		}
		for _, e := range ents {
			if !e.IsDir() {
				continue
			}
			n := strings.ToLower(e.Name())
			if scanSkip[n] || strings.HasPrefix(n, ".") {
				continue
			}
			queue = append(queue, item{filepath.Join(it.dir, e.Name()), it.depth + 1})
		}
	}
	return out
}

// rootFor turns a folder the player picked into an Instances folder: the Soulrend profile itself, CurseForge's modding
// folder (with Instances inside), or the Instances folder.
func rootFor(p string) string {
	p = filepath.Clean(strings.TrimSpace(strings.Trim(p, `"`)))
	if p == "" || p == "." {
		return ""
	}
	if st, err := os.Stat(filepath.Join(p, "minecraftinstance.json")); err == nil && !st.IsDir() {
		return filepath.Dir(p)
	}
	for _, sub := range []string{"Instances", filepath.Join("minecraft", "Instances")} {
		if st, err := os.Stat(filepath.Join(p, sub)); err == nil && st.IsDir() {
			return filepath.Join(p, sub)
		}
	}
	if st, err := os.Stat(p); err == nil && st.IsDir() {
		return p
	}
	return ""
}
