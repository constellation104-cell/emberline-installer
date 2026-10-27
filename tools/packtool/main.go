// packtool makes the installer's data pack (잔불데이터.pack): a zip of one folder, signed with the server owner's
// ed25519 key. The installer only uses a pack whose signature matches the public key it was built with.
//
//	go run ./tools/packtool keygen <private-key-file>        new key pair: writes the private key, prints the public key
//	go run ./tools/packtool pub    <private-key-file>        prints the public key of an existing private key
//	go run ./tools/packtool build  <folder> <out.pack> <private-key-file>
//	go run ./tools/packtool verify <file.pack> <public-key-hex>
//
// The folder holds pack.json, emberline.jar, supplement.zip, extra_mods.json, retired_mods.json, restored_mods.json
// and everycomp-hazardous.toml (see pack.go). Keep the private key file to yourself; it never goes into the repository.
package main

import (
	"archive/zip"
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const magic = "EMBPACK1"

func die(f string, a ...any) {
	fmt.Fprintf(os.Stderr, f+"\n", a...)
	os.Exit(1)
}

func readKey(p string) ed25519.PrivateKey {
	b, err := os.ReadFile(p)
	if err != nil {
		die("private key: %v", err)
	}
	seed, err := hex.DecodeString(strings.TrimSpace(string(b)))
	if err != nil || len(seed) != ed25519.SeedSize {
		die("private key: not a %d-byte hex seed", ed25519.SeedSize)
	}
	return ed25519.NewKeyFromSeed(seed)
}

func main() {
	if len(os.Args) < 2 {
		die("usage: packtool keygen|pub|build|verify ...")
	}
	switch os.Args[1] {
	case "keygen":
		if len(os.Args) != 3 {
			die("usage: packtool keygen <private-key-file>")
		}
		if _, err := os.Stat(os.Args[2]); err == nil {
			die("%s already exists", os.Args[2])
		}
		pub, priv, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			die("%v", err)
		}
		if err := os.WriteFile(os.Args[2], []byte(hex.EncodeToString(priv.Seed())+"\n"), 0o600); err != nil {
			die("%v", err)
		}
		fmt.Println(hex.EncodeToString(pub))
	case "pub":
		if len(os.Args) != 3 {
			die("usage: packtool pub <private-key-file>")
		}
		fmt.Println(hex.EncodeToString(readKey(os.Args[2]).Public().(ed25519.PublicKey)))
	case "build":
		if len(os.Args) != 5 {
			die("usage: packtool build <folder> <out.pack> <private-key-file>")
		}
		key := readKey(os.Args[4])
		body := zipFolder(os.Args[2])
		sig := ed25519.Sign(key, body)
		out := append(append(body, sig...), magic...)
		if err := os.WriteFile(os.Args[3], out, 0o644); err != nil {
			die("%v", err)
		}
		fmt.Printf("%s: %d bytes, signed by %s\n", os.Args[3], len(out), hex.EncodeToString(key.Public().(ed25519.PublicKey)))
	case "verify":
		if len(os.Args) != 4 {
			die("usage: packtool verify <file.pack> <public-key-hex>")
		}
		b, err := os.ReadFile(os.Args[2])
		if err != nil {
			die("%v", err)
		}
		pub, err := hex.DecodeString(os.Args[3])
		if err != nil || len(pub) != ed25519.PublicKeySize {
			die("public key: not a %d-byte hex key", ed25519.PublicKeySize)
		}
		n, tail := len(b), ed25519.SignatureSize+len(magic)
		if n < tail || string(b[n-len(magic):]) != magic {
			die("not a data pack")
		}
		if !ed25519.Verify(pub, b[:n-tail], b[n-tail:n-len(magic)]) {
			die("signature does NOT match")
		}
		zr, err := zip.NewReader(bytes.NewReader(b[:n-tail]), int64(n-tail))
		if err != nil {
			die("%v", err)
		}
		for _, f := range zr.File {
			fmt.Printf("%10d  %s\n", f.UncompressedSize64, f.Name)
		}
		fmt.Println("signature OK")
	default:
		die("unknown command %q", os.Args[1])
	}
}

// zipFolder: the folder's files (not subfolders), sorted, with a fixed date, so the same files give the same pack.
func zipFolder(dir string) []byte {
	ents, err := os.ReadDir(dir)
	if err != nil {
		die("%v", err)
	}
	var names []string
	for _, e := range ents {
		if !e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	when := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, n := range names {
		data, err := os.ReadFile(filepath.Join(dir, n))
		if err != nil {
			die("%v", err)
		}
		method := zip.Deflate
		if strings.HasSuffix(n, ".jar") || strings.HasSuffix(n, ".zip") {
			method = zip.Store // already compressed
		}
		w, err := zw.CreateHeader(&zip.FileHeader{Name: n, Method: method, Modified: when})
		if err != nil {
			die("%v", err)
		}
		if _, err := w.Write(data); err != nil {
			die("%v", err)
		}
	}
	if err := zw.Close(); err != nil {
		die("%v", err)
	}
	return buf.Bytes()
}
