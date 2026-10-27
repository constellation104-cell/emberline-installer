package main

import (
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Mods our server adds on top of Soulrend (Botania, the Let's Do farm set, Create 6.0.8 and add-ons, Macaw's
// building and furniture, Another Furniture, Decorative Blocks). 1.2.1 dropped the Valkyrien Skies ships and
// Create: Central Kitchen (see retire.go). The installer does not carry them:
// each friend's installer downloads them from Modrinth, where their authors publish them, and checks every file
// against the SHA-512 Modrinth lists for it.

// extraModsJSON comes from the data pack (pack.go).

type ExtraMod struct {
	Slug     string `json:"slug"`
	Title    string `json:"title"`
	Filename string `json:"filename"`
	URL      string `json:"url"`
	SHA512   string `json:"sha512"`
	Size     int64  `json:"size"`
	Prefix   string `json:"prefix"`
}

func extraMods() []ExtraMod {
	var l []ExtraMod
	_ = json.Unmarshal(extraModsJSON, &l)
	return l
}

func extraTotal() int64 {
	var t int64
	for _, m := range extraMods() {
		t += m.Size
	}
	return t
}

// progressHook, when set, hears download progress (bytes so far, total, the mod being fetched).
var progressHook func(done, total int64, name string)

func fileSHA512(p string) string {
	f, err := os.Open(p)
	if err != nil {
		return ""
	}
	defer f.Close()
	h := sha512.New()
	if _, err := io.Copy(h, f); err != nil {
		return ""
	}
	return hex.EncodeToString(h.Sum(nil))
}

// switchOffOtherVersions turns off every other version of an extra mod in the mods folder (newer ones too: Entity Culling
// is pinned to an older version on purpose). 1.2.19: it is done the way CurseForge does it itself — <name>.disabled, and
// the add-on marked off in minecraftinstance.json. Before, the file was only renamed to .old-installer, and CurseForge
// downloads a tracked add-on again when its file disappears: on a friend's PC Entity Culling 1.11.3 came back at the next
// start, next to 1.10.5, and the game took the higher version, so the Nightfall crash stayed. Returns the files turned off
// (old name -> new name) and the ones that could not be renamed (the game still open).
func switchOffOtherVersions(mods string, m ExtraMod, ents []os.DirEntry) (map[string]string, []string) {
	renamed := map[string]string{}
	var failed []string
	for _, e := range ents {
		n := e.Name()
		if n == m.Filename || !strings.HasPrefix(n, m.Prefix) || !strings.HasSuffix(strings.ToLower(n), ".jar") {
			continue
		}
		p := filepath.Join(mods, n)
		dis := p + ".disabled"
		if _, err := os.Stat(dis); err == nil {
			_ = os.Rename(dis, dis+".old-installer") // an earlier copy, kept rather than deleted
		}
		if err := os.Rename(p, dis); err != nil {
			failed = append(failed, n)
			continue
		}
		renamed[n] = n + ".disabled"
	}
	return renamed, failed
}

// installExtras puts each extra mod in the profile's mods folder. Files already there with the right hash are kept;
// any other version of the same mod is switched off (switchOffOtherVersions) rather than deleted.
func installExtras(in Instance) (string, error) {
	mods := filepath.Join(in.Dir, "mods")
	if err := os.MkdirAll(mods, 0o755); err != nil {
		return "", err
	}
	list := extraMods()
	total := extraTotal()
	var done int64
	fetched, kept := 0, 0
	client := &http.Client{Timeout: 10 * time.Minute}
	ents, _ := os.ReadDir(mods)
	offAll := map[string]string{}
	var offNames, failed []string
	for _, m := range list {
		dst := filepath.Join(mods, m.Filename)
		off, bad := switchOffOtherVersions(mods, m, ents)
		for from, to := range off {
			offAll[from] = to
			offNames = append(offNames, from)
		}
		failed = append(failed, bad...)
		if fileSHA512(dst) == m.SHA512 {
			kept++
			done += m.Size
			if progressHook != nil {
				progressHook(done, total, m.Title)
			}
			continue
		}
		if err := download(client, m, dst, func(n int64) {
			if progressHook != nil {
				progressHook(done+n, total, m.Title)
			}
		}); err != nil {
			return "", fmt.Errorf("%s 받기 실패: %v", m.Title, err)
		}
		done += m.Size
		fetched++
	}
	note := fmt.Sprintf("%d개 받음, %d개는 이미 있음 (%.1fMB)", fetched, kept, float64(total)/1048576)
	if len(offAll) > 0 {
		sort.Strings(offNames)
		if err := markAddons(filepath.Join(in.Dir, "minecraftinstance.json"), offAll, false); err != nil {
			return "", fmt.Errorf("다른 버전은 껐지만 CurseForge 목록을 고치지 못했어요 (다음 실행 때 다시 받아질 수 있어요): %v", err)
		}
		note += " · 다른 버전 끔: " + strings.Join(offNames, ", ")
	}
	if len(failed) > 0 {
		return "", fmt.Errorf("다른 버전을 끄지 못했어요 (게임을 끄고 다시 실행해 주세요): %s", strings.Join(failed, ", "))
	}
	return note, nil
}

// allowedURL: Modrinth, and (1.2.21) CurseForge's own file server for mods published only there (CraftBrowser).
// Either way the file must match the SHA-512 pinned in extra_mods.json.
func allowedURL(u string) bool {
	return strings.HasPrefix(u, "https://cdn.modrinth.com/") || strings.HasPrefix(u, "https://edge.forgecdn.net/files/")
}

func download(client *http.Client, m ExtraMod, dst string, progress func(int64)) error {
	if !allowedURL(m.URL) {
		return errors.New("알 수 없는 주소")
	}
	var last error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * 2 * time.Second)
		}
		last = func() error {
			req, _ := http.NewRequest("GET", m.URL, nil)
			req.Header.Set("User-Agent", "emberline-installer/"+appVersion)
			resp, err := client.Do(req)
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			if resp.StatusCode != 200 {
				return fmt.Errorf("HTTP %d", resp.StatusCode)
			}
			part := dst + ".part"
			f, err := os.Create(part)
			if err != nil {
				return err
			}
			h := sha512.New()
			var n int64
			buf := make([]byte, 64*1024)
			for {
				k, rerr := resp.Body.Read(buf)
				if k > 0 {
					if _, werr := f.Write(buf[:k]); werr != nil {
						f.Close()
						os.Remove(part)
						return werr
					}
					h.Write(buf[:k])
					n += int64(k)
					progress(n)
				}
				if rerr == io.EOF {
					break
				}
				if rerr != nil {
					f.Close()
					os.Remove(part)
					return rerr
				}
			}
			f.Close()
			if hex.EncodeToString(h.Sum(nil)) != m.SHA512 {
				os.Remove(part)
				return errors.New("파일 확인(해시)이 맞지 않아요")
			}
			_ = os.Remove(dst)
			return os.Rename(part, dst)
		}()
		if last == nil {
			return nil
		}
	}
	return last
}
