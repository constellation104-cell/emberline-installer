package main

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// 1.2.16 — Sinytra Connector keeps Forge-ready copies of the pack's Fabric mods (Origins and its parts) in mods/.connector.
// Building that copy sometimes goes wrong: some names stay in Fabric's own form ("method_8115", "class_1263") instead of
// being turned into Forge's, and then joining the server fails (AbstractMethodError in apoli's recipe). A good copy has no
// such names left at all (checked on a working install: 0 in 20 jars). So the installer looks inside the copy and only
// has the broken jars built again; a good copy stays, which also spares the slow first start (and the one-time Epic Fight
// crash that a full rebuild can hit).

// intermediary names as they appear inside class files: "method_8115", "net/minecraft/class_1263", "field_1234"
var intermediaryName = regexp.MustCompile(`(^|[^A-Za-z0-9_$])(method|field|class|comp)_[0-9]{2,6}([^A-Za-z0-9_]|$)`)

// mixin classes keep Fabric-form names in their annotations on purpose (Connector maps those through the refmap)
var mixinMarker = []byte("Lorg/spongepowered/asm/mixin/Mixin;")

// brokenConnectorJar says why one cached jar is broken, or "" when it is fine
func brokenConnectorJar(path string) string {
	z, err := zip.OpenReader(path)
	if err != nil {
		return "열 수 없음"
	}
	defer z.Close()
	for _, f := range z.File {
		if !strings.HasSuffix(f.Name, ".class") {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return "읽을 수 없음"
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return "읽을 수 없음"
		}
		if bytes.Contains(data, mixinMarker) {
			continue
		}
		if name := leftoverName(data); name != "" {
			return strings.TrimSuffix(f.Name, ".class") + " 안의 " + name
		}
	}
	return ""
}

// leftoverName returns the first Fabric-form name in the class file's constant pool, or ""
func leftoverName(b []byte) string {
	if len(b) < 10 || binary.BigEndian.Uint32(b) != 0xCAFEBABE {
		return ""
	}
	n := int(binary.BigEndian.Uint16(b[8:10]))
	p := 10
	for i := 1; i < n; i++ {
		if p >= len(b) {
			return ""
		}
		tag := b[p]
		p++
		switch tag {
		case 1: // Utf8
			if p+2 > len(b) {
				return ""
			}
			l := int(binary.BigEndian.Uint16(b[p:]))
			p += 2
			if p+l > len(b) {
				return ""
			}
			if m := intermediaryName.Find(b[p : p+l]); m != nil {
				return strings.Trim(string(m), "/;.()[$ ")
			}
			p += l
		case 3, 4: // Integer, Float
			p += 4
		case 5, 6: // Long, Double take two slots
			p += 8
			i++
		case 7, 8, 16, 19, 20: // Class, String, MethodType, Module, Package
			p += 2
		case 9, 10, 11, 12, 17, 18: // refs, NameAndType, Dynamic, InvokeDynamic
			p += 4
		case 15: // MethodHandle
			p += 3
		default:
			return "" // not a class file we understand: say nothing rather than guess
		}
	}
	return ""
}

// checkConnectorCache keeps a good cache and marks broken jars so the game builds just those again
func checkConnectorCache(in Instance) (string, error) {
	dir := filepath.Join(in.Dir, "mods", ".connector")
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		return "해당 없음", nil
	}
	jars, _ := filepath.Glob(filepath.Join(dir, "*_mapped_srg_*.jar"))
	sort.Strings(jars)
	if len(jars) == 0 {
		return "아직 만들어지지 않았어요 (게임을 켜면 만들어요)", nil
	}
	var bad []string
	why := ""
	for _, j := range jars {
		w := brokenConnectorJar(j)
		if w == "" {
			continue
		}
		bad = append(bad, filepath.Base(j))
		if why == "" {
			why = w
		}
		// Connector compares this note with the original jar (version, side, sha256) and builds the jar again when it no
		// longer matches (ConnectorUtil.getCached), so only the broken jars are made again, not the whole cache
		if err := os.WriteFile(j+".input", []byte("rebuild requested by the 잔불 installer: "+w), 0o644); err != nil {
			return "", fmt.Errorf("깨진 캐시를 다시 만들게 표시하지 못했어요 (게임이 켜져 있나요?): %v", err)
		}
	}
	if len(bad) == 0 {
		return fmt.Sprintf("정상이에요 (%d개 확인) · 그대로 둬요", len(jars)), nil
	}
	return fmt.Sprintf("깨진 캐시를 찾았어요 (%d개, 예: %s) · 다음 실행 때 그것만 다시 만들어요 (첫 실행이 조금 더 걸리고, 한 번 꺼지면 다시 켜 주세요)", len(bad), why), nil
}
