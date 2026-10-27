// 잔불 서버 간편 설치기: finds the friend's Soulrend profile (installed by the CurseForge app), then adds our mod,
// the Korean packs, the server entry and a sensible memory setting.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const appVersion = "1.3.0"

func main() {
	cli := flag.Bool("cli", false, "no window: detect and install, printing the report")
	root := flag.String("root", "", "CurseForge Instances folder override (tests)")
	dl := flag.String("downloads", "", "Downloads folder override (tests)")
	mem := flag.Int64("memory", 0, "memory in MB for -cli (0 = recommended)")
	packPath := flag.String("pack", "", "data pack to use instead of the one next to the program")
	flag.Parse()
	env := &Env{roots: cfRoots(), downloads: downloadDirs()}
	if *dl != "" {
		env.downloads = []string{*dl}
	}
	if err := loadPack(*packPath, env.downloads); err != nil {
		logf("data pack: %v", err)
		messageBox("잔불 서버 설치", packProblem(err))
		os.Exit(2)
	}
	logf("data pack %s from %s (Emberline %s)", dataVersion, packFrom, emberlineVersion)
	if *root != "" {
		env.roots = []string{*root}
	} else if env.hasRight() {
		// 1.2.25: the Soulrend profile is in CurseForge's usual place — no need to look through every drive
		env.mu.Lock()
		env.scanned = true
		env.mu.Unlock()
	} else if *cli {
		env.scan()
	} else {
		go env.scan()
	}
	if *cli {
		os.Exit(runCLI(env, *mem))
	}
	runGUI(env)
}

// Env is where to look; kept in one place so tests can point it at fake folders.
type Env struct {
	mu        sync.Mutex
	roots     []string
	downloads []string
	scanning  bool
	scanned   bool
}

// hasRight: a profile of the version our server runs is already in the folders known so far
func (e *Env) hasRight() bool {
	for _, in := range findInstances(e.rootsNow()) {
		if in.Right {
			return true
		}
	}
	return false
}

// scan adds the CurseForge folders found away from the usual places (a moved modding folder).
func (e *Env) scan() {
	e.mu.Lock()
	e.scanning = true
	e.mu.Unlock()
	drives, home := scanStarts()
	var found []string
	if home != "" {
		found = append(found, scanRoots([]string{home}, 4, 3*time.Second)...)
	}
	if len(drives) > 0 {
		found = append(found, scanRoots(drives, 3, 5*time.Second)...)
	}
	e.mu.Lock()
	for _, f := range found {
		e.addRootLocked(f)
	}
	e.scanning = false
	e.scanned = true
	e.mu.Unlock()
	if len(found) > 0 {
		logf("found CurseForge folders: %v", found)
	}
}

func (e *Env) addRootLocked(r string) bool {
	for _, x := range e.roots {
		if strings.EqualFold(filepath.Clean(x), filepath.Clean(r)) {
			return false
		}
	}
	e.roots = append(e.roots, r)
	return true
}

// addRoot: a folder the player picked by hand. Returns how many Soulrend profiles it holds.
func (e *Env) addRoot(p string) int {
	r := rootFor(p)
	if r == "" {
		return 0
	}
	e.mu.Lock()
	e.addRootLocked(r)
	e.mu.Unlock()
	return len(findInstances([]string{r}))
}

func (e *Env) rootsNow() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]string(nil), e.roots...)
}

func (e *Env) state() map[string]any {
	roots := e.rootsNow()
	ins := findInstances(roots)
	e.mu.Lock()
	scanning := e.scanning
	e.mu.Unlock()
	ram := totalRAMGB()
	st := map[string]any{
		"version":     appVersion,
		"data":        dataVersion,
		"instances":   ins,
		"ramGB":       ram,
		"recommendMB": recommendMB(ram),
		"cfRunning":   curseforgeRunning(),
		"moru":        findMoru(e.downloads),
		"server":      strings.TrimSpace(serverName),
		"hasServer":   strings.TrimSpace(serverAddress) != "",
		"roots":       roots,
		"scanning":    scanning,
		"extraMB":     float64(extraTotal()) / 1048576,
		"extraCount":  len(extraMods()),
	}
	return st
}

func runCLI(e *Env, mem int64) int {
	st := e.state()
	b, _ := json.MarshalIndent(st, "", "  ")
	fmt.Println(string(b))
	ins := findInstances(e.rootsNow())
	if len(ins) == 0 || !ins[0].Right {
		fmt.Println("no Soulrend " + packVersion + " profile")
		return 2
	}
	if curseforgeRunning() {
		fmt.Println("CurseForge is running: close it first")
		return 3
	}
	if mem == 0 {
		mem = recommendMB(totalRAMGB())
	}
	steps := install(ins[0], findMoru(e.downloads), InstallOptions{MemoryMB: mem, AddServer: true})
	for _, s := range steps {
		mark := "OK "
		if !s.OK {
			mark = "-- "
		}
		fmt.Println(mark + s.Title + " · " + s.Note)
	}
	return 0
}

func logf(format string, a ...any) {
	dir := filepath.Join(os.TempDir(), "emberline-installer")
	_ = os.MkdirAll(dir, 0o755)
	f, err := os.OpenFile(filepath.Join(dir, "log.txt"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s %s\n", time.Now().Format("2006-01-02 15:04:05"), fmt.Sprintf(format, a...))
}
