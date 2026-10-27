package main

// 1.3.0: the data pack. Everything that belongs to one server rather than to installing — the server's address and
// name, our mod (Emberline), the Korean supplement resource pack, the mod lists — comes in 잔불데이터.pack, a zip
// next to the program, signed by the server's owner with an ed25519 key. The program itself (whose source is public)
// holds only the matching public key, so it installs only what that owner put together: a pack someone else made or
// changed is refused. Another server builds the program with its own key (tools/packtool).

import (
	"archive/zip"
	"bytes"
	"crypto/ed25519"
	"encoding/hex"
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

const (
	packName   = "잔불데이터.pack"
	packMagic  = "EMBPACK1" // the last 8 bytes; before them the 64-byte signature of everything before it
	packFormat = 1
	packMaxMB  = 256
)

// packPublicKey: the server owner's ed25519 public key (hex). Packs signed with any other key are refused.
const packPublicKey = "31b0a9465618b73f99a971ea933d399ee82ea2739bd14d1ffc37f943506ca738"

// pack.json
type packManifest struct {
	Format    int    `json:"format"`
	Data      string `json:"data"`      // the pack's own version, shown in the window
	Server    string `json:"server"`    // the name in the multiplayer list
	Address   string `json:"address"`   // host[:port]; empty = no server entry
	Emberline string `json:"emberline"` // version of emberline.jar (installed as emberline-<version>.jar)
	MinApp    string `json:"minApp,omitempty"`
}

// what the pack provides (empty until loadPack succeeds)
var (
	dataVersion      string
	emberlineVersion string
	emberlineName    string
	emberlineJar     []byte
	supplementPack   []byte
	serverAddress    string
	serverName       string
	extraModsJSON    []byte
	retiredJSON      []byte
	restoredJSON     []byte
	everycompDefault []byte
	packFrom         string
)

var packFiles = []string{"pack.json", "emberline.jar", "supplement.zip", "extra_mods.json", "retired_mods.json",
	"restored_mods.json", "everycomp-hazardous.toml"}

// openPack checks the signature and returns the zip inside.
func openPack(b []byte, pubHex string) (*zip.Reader, error) {
	n := len(b)
	tail := ed25519.SignatureSize + len(packMagic)
	if n < tail || string(b[n-len(packMagic):]) != packMagic {
		return nil, errors.New("잔불 데이터 파일이 아니에요")
	}
	pub, err := hex.DecodeString(pubHex)
	if err != nil || len(pub) != ed25519.PublicKeySize {
		return nil, errors.New("설치기에 들어 있는 공개 키가 잘못됐어요")
	}
	body, sig := b[:n-tail], b[n-tail:n-len(packMagic)]
	if !ed25519.Verify(ed25519.PublicKey(pub), body, sig) {
		return nil, errors.New("서명이 맞지 않아요. 서버 주인이 만든 데이터가 아니거나 파일이 손상됐어요")
	}
	return zip.NewReader(bytes.NewReader(body), int64(len(body)))
}

func readPack(b []byte, pubHex string) (map[string][]byte, packManifest, error) {
	var m packManifest
	zr, err := openPack(b, pubHex)
	if err != nil {
		return nil, m, err
	}
	files := map[string][]byte{}
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		if f.UncompressedSize64 > packMaxMB<<20 {
			return nil, m, fmt.Errorf("%s: 너무 커요", f.Name)
		}
		rc, err := f.Open()
		if err != nil {
			return nil, m, err
		}
		data, err := io.ReadAll(io.LimitReader(rc, packMaxMB<<20+1))
		rc.Close()
		if err != nil {
			return nil, m, fmt.Errorf("%s: %v", f.Name, err)
		}
		files[f.Name] = data
	}
	for _, name := range packFiles {
		if _, ok := files[name]; !ok {
			return nil, m, fmt.Errorf("데이터에 %s가 없어요", name)
		}
	}
	if err := json.Unmarshal(files["pack.json"], &m); err != nil {
		return nil, m, fmt.Errorf("pack.json: %v", err)
	}
	if m.Format != packFormat {
		return nil, m, fmt.Errorf("이 데이터는 형식 %d예요. 새 설치기가 필요해요", m.Format)
	}
	if m.MinApp != "" && versionLess(appVersion, m.MinApp) {
		return nil, m, fmt.Errorf("이 데이터는 설치기 %s 이상이 필요해요 (지금 %s)", m.MinApp, appVersion)
	}
	if m.Emberline == "" || strings.ContainsAny(m.Emberline, `/\:`) {
		return nil, m, errors.New("pack.json: emberline 버전이 비었거나 잘못됐어요")
	}
	return files, m, nil
}

// usePack fills the variables the installer works from.
func usePack(files map[string][]byte, m packManifest, from string) {
	dataVersion = m.Data
	emberlineVersion = m.Emberline
	emberlineName = "emberline-" + m.Emberline + ".jar"
	emberlineJar = files["emberline.jar"]
	supplementPack = files["supplement.zip"]
	serverAddress = strings.TrimSpace(m.Address)
	serverName = strings.TrimSpace(m.Server)
	extraModsJSON = files["extra_mods.json"]
	retiredJSON = files["retired_mods.json"]
	restoredJSON = files["restored_mods.json"]
	everycompDefault = files["everycomp-hazardous.toml"]
	packFrom = from
}

// loadPack finds the pack: the path given, next to the program, in the current folder, and — for a program started
// straight from inside the zip it came in (Windows then copies only the .exe out) — inside the newest
// 잔불서버설치_v*.zip in the downloads folders.
func loadPack(explicit string, downloads []string) error {
	var tried []string
	try := func(p string) (bool, error) {
		b, err := os.ReadFile(p)
		if err != nil {
			return false, nil
		}
		files, m, err := readPack(b, packPublicKey)
		if err != nil {
			return true, fmt.Errorf("%s: %v", p, err)
		}
		usePack(files, m, p)
		return true, nil
	}
	if explicit != "" {
		if ok, err := try(explicit); ok || err != nil {
			return err
		}
		return fmt.Errorf("%s를 열 수 없어요", explicit)
	}
	if exe, err := os.Executable(); err == nil {
		tried = append(tried, filepath.Join(filepath.Dir(exe), packName))
	}
	if wd, err := os.Getwd(); err == nil {
		tried = append(tried, filepath.Join(wd, packName))
	}
	for _, p := range tried {
		if ok, err := try(p); ok {
			return err
		}
	}
	for _, z := range installerZips(downloads) {
		zr, err := zip.OpenReader(z)
		if err != nil {
			continue
		}
		for _, f := range zr.File {
			if filepath.Base(f.Name) != packName || f.UncompressedSize64 > packMaxMB<<20 {
				continue
			}
			rc, err := f.Open()
			if err != nil {
				continue
			}
			b, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				continue
			}
			if files, m, err := readPack(b, packPublicKey); err == nil {
				zr.Close()
				usePack(files, m, z)
				return nil
			}
		}
		zr.Close()
	}
	return errPackMissing
}

var errPackMissing = errors.New("missing")

// installerZips: 잔불서버설치_v*.zip in the downloads folders, newest first
func installerZips(dirs []string) []string {
	type z struct {
		p string
		t int64
	}
	var l []z
	for _, d := range dirs {
		ents, err := os.ReadDir(d)
		if err != nil {
			continue
		}
		for _, e := range ents {
			n := e.Name()
			if e.IsDir() || !strings.HasPrefix(n, "잔불서버설치_v") || !strings.HasSuffix(strings.ToLower(n), ".zip") {
				continue
			}
			if info, err := e.Info(); err == nil {
				l = append(l, z{filepath.Join(d, n), info.ModTime().UnixNano()})
			}
		}
	}
	sort.Slice(l, func(i, j int) bool { return l[i].t > l[j].t })
	out := make([]string, 0, len(l))
	for _, x := range l {
		out = append(out, x.p)
	}
	return out
}

// versionLess compares dotted versions numerically ("1.2.9" < "1.2.10").
func versionLess(a, b string) bool {
	pa, pb := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(pa) || i < len(pb); i++ {
		var x, y int
		if i < len(pa) {
			x, _ = strconv.Atoi(pa[i])
		}
		if i < len(pb) {
			y, _ = strconv.Atoi(pb[i])
		}
		if x != y {
			return x < y
		}
	}
	return false
}

func packProblem(err error) string {
	if errors.Is(err, errPackMissing) {
		return "설치 데이터(" + packName + ")를 찾지 못했어요.\n\n받은 zip 파일을 먼저 압축 해제한 다음, 풀린 폴더 안의 잔불서버설치.exe를 실행해 주세요."
	}
	return "설치 데이터를 쓸 수 없어요.\n\n" + err.Error() + "\n\n서버 공지에서 설치기를 다시 받아 주세요."
}
