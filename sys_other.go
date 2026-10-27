//go:build !windows

package main

import (
	"fmt"
	"os"
	"strings"
)

// Linux stand-ins so the logic and the page can be tested away from Windows.

func cfRoots() []string {
	if r := os.Getenv("INSTALLER_ROOT"); r != "" {
		return []string{r}
	}
	return nil
}

func scanStarts() (drives []string, home string) {
	if d := os.Getenv("INSTALLER_SCAN"); d != "" {
		return []string{d}, ""
	}
	return nil, ""
}

func pickFolder() string { return os.Getenv("INSTALLER_PICK") }

func downloadDirs() []string {
	if d := os.Getenv("INSTALLER_DOWNLOADS"); d != "" {
		return []string{d}
	}
	return nil
}

func totalRAMGB() float64 {
	b, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 16
	}
	var kb float64
	for _, l := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(l, "MemTotal:") {
			fmt.Sscanf(strings.TrimSpace(strings.TrimPrefix(l, "MemTotal:")), "%f", &kb)
		}
	}
	if v := os.Getenv("INSTALLER_RAM_GB"); v != "" {
		var g float64
		fmt.Sscanf(v, "%f", &g)
		return g
	}
	return kb / (1 << 20)
}

func openURL(u string)         { fmt.Println("open url:", u) }
func openPath(p string)        { fmt.Println("open path:", p) }
func curseforgeRunning() bool  { return os.Getenv("INSTALLER_CF_RUNNING") == "1" }
func openCurseForge() bool     { fmt.Println("open curseforge"); return true }
func messageBox(t, text string) { fmt.Println(t + ": " + text) }
