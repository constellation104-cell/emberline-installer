package main

import (
	"archive/zip"
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testPack(t *testing.T, priv ed25519.PrivateKey, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for n, c := range files {
		w, err := zw.Create(n)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write([]byte(c))
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	body := buf.Bytes()
	return append(append(body, ed25519.Sign(priv, body)...), packMagic...)
}

func fullFiles() map[string]string {
	return map[string]string{
		"pack.json":                `{"format":1,"data":"t1","server":"테스트","address":"example.invalid:25565","emberline":"9.9.9"}`,
		"emberline.jar":            "jar",
		"supplement.zip":           "zip",
		"extra_mods.json":          "[]",
		"retired_mods.json":        "[]",
		"restored_mods.json":       "[]",
		"everycomp-hazardous.toml": "x",
	}
}

func TestPackSignature(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	pubHex := hex.EncodeToString(pub)
	b := testPack(t, priv, fullFiles())

	files, m, err := readPack(b, pubHex)
	if err != nil {
		t.Fatalf("good pack refused: %v", err)
	}
	if m.Emberline != "9.9.9" || m.Server != "테스트" || string(files["emberline.jar"]) != "jar" {
		t.Fatalf("wrong contents: %+v", m)
	}

	// one byte changed anywhere before the signature
	bad := append([]byte(nil), b...)
	bad[10] ^= 1
	if _, _, err := readPack(bad, pubHex); err == nil {
		t.Fatal("changed pack accepted")
	}

	// signed by someone else
	_, other, _ := ed25519.GenerateKey(rand.Reader)
	if _, _, err := readPack(testPack(t, other, fullFiles()), pubHex); err == nil || !strings.Contains(err.Error(), "서명") {
		t.Fatalf("pack signed by another key: %v", err)
	}

	// not a pack at all
	if _, _, err := readPack([]byte("PK\x03\x04 hello"), pubHex); err == nil {
		t.Fatal("random bytes accepted")
	}

	// a file missing
	f := fullFiles()
	delete(f, "supplement.zip")
	if _, _, err := readPack(testPack(t, priv, f), pubHex); err == nil {
		t.Fatal("pack without supplement.zip accepted")
	}

	// a newer format, or a newer installer needed
	f = fullFiles()
	f["pack.json"] = `{"format":2,"emberline":"1"}`
	if _, _, err := readPack(testPack(t, priv, f), pubHex); err == nil {
		t.Fatal("format 2 accepted")
	}
	f["pack.json"] = `{"format":1,"emberline":"1","minApp":"99.0.0"}`
	if _, _, err := readPack(testPack(t, priv, f), pubHex); err == nil {
		t.Fatal("minApp 99 accepted")
	}
	// a version that would make a path
	f["pack.json"] = `{"format":1,"emberline":"../../x"}`
	if _, _, err := readPack(testPack(t, priv, f), pubHex); err == nil {
		t.Fatal("emberline version with a path accepted")
	}
}

func TestInstallerZipsNewestFirst(t *testing.T) {
	d := t.TempDir()
	for i, n := range []string{"잔불서버설치_v1.2.9.zip", "잔불서버설치_v1.3.0.zip", "other.zip"} {
		p := filepath.Join(d, n)
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		tm := testTime(i)
		_ = os.Chtimes(p, tm, tm)
	}
	l := installerZips([]string{d})
	if len(l) != 2 || filepath.Base(l[0]) != "잔불서버설치_v1.3.0.zip" {
		t.Fatalf("got %v", l)
	}
}

func TestVersionLess(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want bool
	}{{"1.2.9", "1.2.10", true}, {"1.3.0", "1.3.0", false}, {"1.3", "1.3.0", false}, {"2.0.0", "1.9.9", false}, {"1.2.25", "1.3.0", true}} {
		if got := versionLess(c.a, c.b); got != c.want {
			t.Errorf("versionLess(%q,%q)=%v", c.a, c.b, got)
		}
	}
}

func testTime(i int) time.Time { return time.Date(2026, 1, 1+i, 0, 0, 0, 0, time.UTC) }
