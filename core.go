package main

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// What this installer adds to a friend's own Soulrend instance. Nothing here is someone else's file:
// the modpack comes from the CurseForge app, the Korean patch from moru.gg (the friend downloads it),
// and only our own mod and translation pack are inside the exe.

const (
	packProjectID = 1229700
	packFileID    = 9272906
	packVersion   = "v8.5.2"
	installURL    = "https://www.curseforge.com/minecraft/modpacks/soulrend/install/9272906"
	packPageURL   = "https://www.curseforge.com/minecraft/modpacks/soulrend"
	moruURL       = "https://moru.gg/ko/modpack/soulrend"
	supplementZip = "Soulrend_8.5.2_한글_보충팩.zip"
)


// Instance is one CurseForge profile that has Soulrend installed.
type Instance struct {
	Dir      string `json:"dir"`
	Name     string `json:"name"`
	Version  string `json:"version"`
	FileID   int64  `json:"fileId"`
	MemoryMB int64  `json:"memoryMB"`
	Right    bool   `json:"right"` // the version our server runs
}

func readJSONMap(p string) (map[string]any, error) {
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	b = []byte(strings.TrimPrefix(string(b), "\ufeff"))
	d := json.NewDecoder(strings.NewReader(string(b)))
	d.UseNumber()
	var m map[string]any
	if err := d.Decode(&m); err != nil {
		return nil, err
	}
	return m, nil
}

func num(v any) int64 {
	switch x := v.(type) {
	case json.Number:
		n, _ := x.Int64()
		return n
	case float64:
		return int64(x)
	}
	return 0
}

// findInstances lists every Soulrend profile under the given CurseForge "Instances" folders, newest version first.
func findInstances(roots []string) []Instance {
	var out []Instance
	seen := map[string]bool{}
	for _, root := range roots {
		ents, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, e := range ents {
			if !e.IsDir() {
				continue
			}
			dir := filepath.Join(root, e.Name())
			if seen[strings.ToLower(dir)] {
				continue
			}
			m, err := readJSONMap(filepath.Join(dir, "minecraftinstance.json"))
			if err != nil {
				continue
			}
			mp, _ := m["installedModpack"].(map[string]any)
			if mp == nil || num(mp["addonID"]) != packProjectID {
				continue
			}
			seen[strings.ToLower(dir)] = true
			in := Instance{Dir: dir, Name: e.Name(), MemoryMB: num(m["allocatedMemory"])}
			if f, ok := mp["installedFile"].(map[string]any); ok {
				in.FileID = num(f["id"])
				in.Version = versionOf(f)
			}
			in.Right = in.FileID == packFileID || in.Version == packVersion
			out = append(out, in)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Right != out[j].Right {
			return out[i].Right
		}
		return out[i].FileID > out[j].FileID
	})
	return out
}

var verRe = regexp.MustCompile(`v?\d+\.\d+(\.\d+)?`)

func versionOf(f map[string]any) string {
	for _, k := range []string{"displayName", "fileName"} {
		if s, ok := f[k].(string); ok {
			if v := verRe.FindString(s); v != "" {
				if !strings.HasPrefix(v, "v") {
					v = "v" + v
				}
				return v
			}
		}
	}
	return ""
}

// recommendMB picks the memory for Minecraft from the PC's RAM (Soulrend asks for about 10GB).
func recommendMB(totalGB float64) int64 {
	switch {
	case totalGB >= 30:
		return 12288
	case totalGB >= 15:
		return 10240
	case totalGB >= 11:
		return 8192
	default:
		return 6144
	}
}

// setMemory writes the profile's own memory setting; CurseForge must be closed or it writes its copy back.
func setMemory(in Instance, mb int64) error {
	p := filepath.Join(in.Dir, "minecraftinstance.json")
	raw, err := os.ReadFile(p)
	if err != nil {
		return err
	}
	// change just the one number in the text so nothing else in CurseForge's file moves
	re := regexp.MustCompile(`("allocatedMemory"\s*:\s*)-?\d+`)
	if !re.Match(raw) {
		return errors.New("메모리 설정 항목을 찾지 못했어요")
	}
	if err := os.WriteFile(p+".bak-installer", raw, 0o644); err != nil {
		return err
	}
	out := re.ReplaceAll(raw, []byte("${1}"+strconv.FormatInt(mb, 10)))
	return writeAtomic(p, out)
}

func writeAtomic(p string, b []byte) error {
	tmp := p + ".tmp-installer"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

// Moru are the Korean patch files the friend downloaded from moru.gg.
type Moru struct {
	Pack      string `json:"pack"`      // resource pack zip
	Overrides string `json:"overrides"` // overrides zip (we only take kubejs/assets from it)
}

var moruPack = regexp.MustCompile(`(?i)soulrend.*ko[_-]?kr.*resource[_-]?pack.*\.zip$`)
var moruOver = regexp.MustCompile(`(?i)soulrend.*ko[_-]?kr.*overrides.*\.zip$`)

func findMoru(dirs []string) Moru {
	var m Moru
	var pt, ot time.Time
	for _, d := range dirs {
		ents, err := os.ReadDir(d)
		if err != nil {
			continue
		}
		for _, e := range ents {
			if e.IsDir() {
				continue
			}
			info, err := e.Info()
			if err != nil || info.Size() < 1000 {
				continue
			}
			p := filepath.Join(d, e.Name())
			switch {
			case moruPack.MatchString(e.Name()) && info.ModTime().After(pt):
				m.Pack, pt = p, info.ModTime()
			case moruOver.MatchString(e.Name()) && info.ModTime().After(ot):
				m.Overrides, ot = p, info.ModTime()
			}
		}
	}
	return m
}

// Step is one line of the install report.
type Step struct {
	Title string `json:"title"`
	OK    bool   `json:"ok"`
	Note  string `json:"note"`
}

type InstallOptions struct {
	MemoryMB  int64 `json:"memoryMB"`
	SkipMoru  bool  `json:"skipMoru"`
	AddServer bool  `json:"addServer"`
}

// install puts everything in place. Each step is independent; a failed step is reported and the rest still run.
func install(in Instance, moru Moru, o InstallOptions) []Step {
	var steps []Step
	add := func(title string, err error, okNote string) {
		s := Step{Title: title, OK: err == nil, Note: okNote}
		if err != nil {
			s.Note = err.Error()
		}
		steps = append(steps, s)
	}

	// 1. our mod
	add("잔불 모드 넣기", installJar(in), emberlineName)

	// 1a. mods 1.2.0 added that the server no longer has (ships, Central Kitchen): off, as CurseForge itself would
	rnote, rerr := retireMods(in)
	add("안 쓰는 모드 끄기", rerr, rnote)
	// 1a'. Xaero's maps: the server needs them on every client (1.2.2 and 1.2.3 had switched them off)
	xnote, xerr2 := restoreMods(in)
	add("꺼 둔 모드 다시 켜기", xerr2, xnote)
	// 1a''. 1.2.25: the 잔불 minimap instead of Xaero's (Xaero stays installed; only its minimap display goes off, once)
	mnote, merr := hideXaeroMinimap(in)
	add("미니맵을 잔불 미니맵으로", merr, mnote)

	// 1b. the mods our server adds (Botania, Let's Do, Create add-ons, building and furniture), from Modrinth
	note, xerr := installExtras(in)
	add("추가 모드 받기 ("+strconv.Itoa(len(extraMods()))+"개)", xerr, note)

	// 1b'. Sinytra Connector keeps Forge-ready copies of the pack's Fabric mods (Origins and its parts) in mods/.connector.
	// A copy can come out broken, and then joining fails with an AbstractMethodError from apoli's recipe. 1.2.16: look
	// inside it and move it aside only when it is broken (connector.go); a good copy stays, so no slow, crash-prone rebuild.
	cc, ccErr := checkConnectorCache(in)
	add("Connector 캐시 점검", ccErr, cc)

	// 1c. settings that must match the server (Every Compat blocks; Radium's POI setting, kept as the server has it)
	cnote, cerr := syncConfigs(in)
	add("설정을 서버와 맞추기", cerr, cnote)

	// 2. Korean: moru's pack (if the friend downloaded it) and our supplement
	rpDir := filepath.Join(in.Dir, "resourcepacks")
	_ = os.MkdirAll(rpDir, 0o755)
	var packs []string
	if !o.SkipMoru && moru.Pack != "" {
		name := filepath.Base(moru.Pack)
		err := copyFile(moru.Pack, filepath.Join(rpDir, name))
		add("모루 한글패치 넣기", err, name)
		if err == nil {
			packs = append(packs, "file/"+name)
		}
	} else {
		add("모루 한글패치 넣기", errors.New("건너뜀: 받은 파일이 없어요"), "")
	}
	if !o.SkipMoru && moru.Overrides != "" {
		n, err := extractKubeAssets(moru.Overrides, in.Dir)
		add("모루 추가 번역(kubejs) 넣기", err, fmt.Sprintf("파일 %d개", n))
	}
	err := writeAtomic(filepath.Join(rpDir, supplementZip), supplementPack)
	add("한글 보충팩 넣기", err, supplementZip)
	if err == nil {
		packs = append(packs, "file/"+supplementZip)
	}

	// 3. language and resource pack order
	add("한국어와 리소스팩 켜기", setOptions(in.Dir, packs), "언어 한국어, 리소스팩 "+strconv.Itoa(len(packs))+"개")

	// 4. server list
	if o.AddServer && strings.TrimSpace(serverAddress) != "" {
		add("서버 목록에 잔불 서버 추가", addServer(filepath.Join(in.Dir, "servers.dat"), strings.TrimSpace(serverName), strings.TrimSpace(serverAddress)), "멀티플레이 목록 맨 위")
	}

	// 4a. Soulrend's official realm picker and official server list: their own options set so Multiplayer opens our list
	snote, serr := skipRealms(in, strings.TrimSpace(serverName))
	add("공식 서버 선택 화면 끄기", serr, snote)

	// 5. memory
	if o.MemoryMB > 0 {
		add("메모리 "+strconv.FormatInt(o.MemoryMB/1024, 10)+"GB로 설정", setMemory(in, o.MemoryMB), "")
	}
	return steps
}

func installJar(in Instance) error {
	mods := filepath.Join(in.Dir, "mods")
	if err := os.MkdirAll(mods, 0o755); err != nil {
		return err
	}
	ents, _ := os.ReadDir(mods)
	for _, e := range ents { // an older Emberline goes away; nothing else is touched
		n := strings.ToLower(e.Name())
		if strings.HasPrefix(n, "emberline-") && strings.HasSuffix(n, ".jar") && e.Name() != emberlineName {
			_ = os.Remove(filepath.Join(mods, e.Name()))
		}
	}
	return writeAtomic(filepath.Join(mods, emberlineName), emberlineJar)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp := dst + ".tmp-installer"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, dst)
}

// extractKubeAssets copies only kubejs/assets/** (language files) from moru's overrides zip; quests and data
// in that zip belong to an older pack version and the server decides those anyway.
func extractKubeAssets(zipPath, dst string) (int, error) {
	z, err := zip.OpenReader(zipPath)
	if err != nil {
		return 0, err
	}
	defer z.Close()
	n := 0
	for _, f := range z.File {
		name := strings.ReplaceAll(f.Name, "\\", "/")
		name = strings.TrimPrefix(name, "overrides/")
		if !strings.HasPrefix(name, "kubejs/assets/") || strings.HasSuffix(name, "/") || strings.Contains(name, "..") {
			continue
		}
		target := filepath.Join(dst, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return n, err
		}
		r, err := f.Open()
		if err != nil {
			return n, err
		}
		b, err := io.ReadAll(r)
		r.Close()
		if err != nil {
			return n, err
		}
		if err := writeAtomic(target, b); err != nil {
			return n, err
		}
		n++
	}
	if n == 0 {
		return 0, errors.New("압축 파일 안에 번역 파일이 없어요")
	}
	return n, nil
}

// setOptions turns on Korean and appends our packs (moru, then the supplement) on top of the pack's own list.
// If the game never ran yet, the pack's default options are copied first so its key bindings etc. still apply.
func setOptions(dir string, packs []string) error {
	p := filepath.Join(dir, "options.txt")
	if _, err := os.Stat(p); err != nil {
		def := filepath.Join(dir, "config", "defaultoptions", "options.txt")
		if b, err := os.ReadFile(def); err == nil {
			if err := os.WriteFile(p, b, 0o644); err != nil {
				return err
			}
		} else if err := os.WriteFile(p, nil, 0o644); err != nil {
			return err
		}
	}
	raw, err := os.ReadFile(p)
	if err != nil {
		return err
	}
	_ = os.WriteFile(p+".bak-installer", raw, 0o644)
	lines := strings.Split(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n")
	hasLang, hasPacks := false, false
	for i, l := range lines {
		switch {
		case strings.HasPrefix(l, "lang:"):
			lines[i] = "lang:ko_kr"
			hasLang = true
		case strings.HasPrefix(l, "resourcePacks:"):
			var cur []string
			_ = json.Unmarshal([]byte(strings.TrimPrefix(l, "resourcePacks:")), &cur)
			lines[i] = "resourcePacks:" + mergePacks(cur, packs)
			hasPacks = true
		}
	}
	// 1.2.21: CraftBrowser's own keys (B, P, H, arrows) are taken in this pack (backpacks, the 잔불 hub, Familiars,
	// Shoulder Surfing). Free keys are set only where the friend has no binding for them yet, so a choice made in
	// Options > Controls is never changed.
	for _, kb := range craftBrowserKeys {
		has := false
		for _, l := range lines {
			if strings.HasPrefix(l, kb[0]+":") {
				has = true
				break
			}
		}
		if !has {
			lines = insertBeforeEmptyTail(lines, kb[0]+":"+kb[1])
		}
	}
	if !hasLang {
		lines = insertBeforeEmptyTail(lines, "lang:ko_kr")
	}
	if !hasPacks {
		lines = insertBeforeEmptyTail(lines, "resourcePacks:"+mergePacks([]string{"vanilla"}, packs))
	}
	return writeAtomic(p, []byte(strings.Join(lines, "\n")))
}

var craftBrowserKeys = [][2]string{
	{"key_key.craftbrowser.toggle_browser", "key.keyboard.f6"},
	{"key_key.craftbrowser.toggle_pip", "key.keyboard.f8"},
	{"key_key.craftbrowser.panel_lock", "key.keyboard.f9"},
	{"key_key.craftbrowser.scroll_up", "key.keyboard.unknown"},
	{"key_key.craftbrowser.scroll_down", "key.keyboard.unknown"},
	{"key_key.craftbrowser.pip_grow", "key.keyboard.equal"},
	{"key_key.craftbrowser.pip_shrink", "key.keyboard.minus"},
}

func insertBeforeEmptyTail(lines []string, l string) []string {
	if n := len(lines); n > 0 && lines[n-1] == "" {
		return append(lines[:n-1], l, "")
	}
	return append(lines, l)
}

func mergePacks(cur, add []string) string {
	var out []string
	drop := map[string]bool{}
	for _, a := range add {
		drop[a] = true
	}
	for _, c := range cur {
		// remove older copies of our packs and of moru's Soulrend patch so the new ones go on top
		lc := strings.ToLower(c)
		if drop[c] || strings.Contains(lc, "한글_보충팩") || moruPack.MatchString(strings.TrimPrefix(c, "file/")) {
			continue
		}
		out = append(out, c)
	}
	out = append(out, add...)
	b, _ := json.Marshal(out)
	return string(b)
}

