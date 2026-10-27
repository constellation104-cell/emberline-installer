//go:build windows

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

// cfRoots: CurseForge keeps profiles in <home>\curseforge\minecraft\Instances unless moved; other drives are checked too.
func cfRoots() []string {
	var out []string
	if h, err := os.UserHomeDir(); err == nil {
		out = append(out, filepath.Join(h, "curseforge", "minecraft", "Instances"))
	}
	for _, d := range "CDEFGH" {
		for _, sub := range []string{`curseforge\minecraft\Instances`, `CurseForge\minecraft\Instances`, `Games\curseforge\minecraft\Instances`} {
			p := string(d) + `:\` + sub
			if st, err := os.Stat(p); err == nil && st.IsDir() {
				out = append(out, p)
			}
		}
	}
	return out
}

// scanStarts: where to look for a moved CurseForge folder — every fixed drive, and the home folder (deeper there).
func scanStarts() (drives []string, home string) {
	for _, d := range "CDEFGHIJKLMNOPQRSTUVWXYZ" {
		root := string(d) + `:\`
		p, err := windows.UTF16PtrFromString(root)
		if err != nil {
			continue
		}
		if windows.GetDriveType(p) == windows.DRIVE_FIXED {
			drives = append(drives, root)
		}
	}
	home, _ = os.UserHomeDir()
	return drives, home
}

func downloadDirs() []string {
	var out []string
	if p, err := windows.KnownFolderPath(windows.FOLDERID_Downloads, 0); err == nil {
		out = append(out, p)
	}
	if h, err := os.UserHomeDir(); err == nil {
		out = append(out, filepath.Join(h, "Downloads"), filepath.Join(h, "Desktop"))
	}
	return out
}

type memoryStatusEx struct {
	Length               uint32
	MemoryLoad           uint32
	TotalPhys            uint64
	AvailPhys            uint64
	TotalPageFile        uint64
	AvailPageFile        uint64
	TotalVirtual         uint64
	AvailVirtual         uint64
	AvailExtendedVirtual uint64
}

var procGlobalMemoryStatusEx = windows.NewLazySystemDLL("kernel32.dll").NewProc("GlobalMemoryStatusEx")

func totalRAMGB() float64 {
	var m memoryStatusEx
	m.Length = uint32(unsafe.Sizeof(m))
	if r, _, _ := procGlobalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&m))); r == 0 {
		return 0
	}
	return float64(m.TotalPhys) / (1 << 30)
}

func shellOpen(target string) error {
	verb, _ := windows.UTF16PtrFromString("open")
	file, err := windows.UTF16PtrFromString(target)
	if err != nil {
		return err
	}
	return windows.ShellExecute(0, verb, file, nil, nil, windows.SW_SHOWNORMAL)
}

func openURL(u string) {
	if strings.HasPrefix(u, "https://") {
		_ = shellOpen(u)
	}
}

func openPath(p string) { _ = shellOpen(p) }

// curseforgeRunning: the CurseForge app (an Overwolf/Electron app) runs as CurseForge.exe
func curseforgeRunning() bool {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return false
	}
	defer windows.CloseHandle(snap)
	var pe windows.ProcessEntry32
	pe.Size = uint32(unsafe.Sizeof(pe))
	for err = windows.Process32First(snap, &pe); err == nil; err = windows.Process32Next(snap, &pe) {
		if strings.EqualFold(windows.UTF16ToString(pe.ExeFile[:]), "CurseForge.exe") {
			return true
		}
	}
	return false
}

func openCurseForge() bool {
	if la := os.Getenv("LOCALAPPDATA"); la != "" {
		p := filepath.Join(la, "Programs", "CurseForge Windows", "CurseForge.exe")
		if _, err := os.Stat(p); err == nil {
			return exec.Command(p).Start() == nil
		}
	}
	return false
}

func messageBox(title, text string) {
	t, _ := windows.UTF16PtrFromString(title)
	m, _ := windows.UTF16PtrFromString(text)
	_, _ = windows.MessageBox(0, m, t, windows.MB_OK|windows.MB_ICONINFORMATION)
}
