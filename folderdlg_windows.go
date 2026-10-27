//go:build windows

package main

import (
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// 1.2.25: the folder picker is Windows' own folder window (IFileOpenDialog), opened directly. Up to 1.2.24 it went
// through a hidden PowerShell, which works but is exactly what antivirus programs watch for in unknown programs.

var (
	pCoCreateInstance = windows.NewLazySystemDLL("ole32.dll").NewProc("CoCreateInstance")

	clsidFileOpenDialog = windows.GUID{Data1: 0xDC1C5A9C, Data2: 0xE88A, Data3: 0x4DDE, Data4: [8]byte{0xA5, 0xA1, 0x60, 0xF8, 0x2A, 0x20, 0xAE, 0xF7}}
	iidIFileOpenDialog  = windows.GUID{Data1: 0xD57C7288, Data2: 0xD4AD, Data3: 0x4768, Data4: [8]byte{0xBE, 0x02, 0x9D, 0x96, 0x95, 0x32, 0xD9, 0x60}}
)

const (
	clsctxInprocServer = 0x1
	fosNoChangeDir     = 0x8
	fosPickFolders     = 0x20
	fosForceFileSystem = 0x40
	fosPathMustExist   = 0x800
	sigdnFileSysPath   = 0x80058000
	hrCancelled        = 0x800704C7 // HRESULT_FROM_WIN32(ERROR_CANCELLED): the player closed the window

	// vtable slots: IUnknown 0–2, IModalWindow 3, IFileDialog 4–26 (IFileOpenDialog adds 27–28)
	vRelease    = 2
	vShow       = 3
	vSetOptions = 9
	vGetOptions = 10
	vSetTitle   = 17
	vGetResult  = 20
	// IShellItem
	vGetDisplayName = 5
)

// vt: method i of a COM object
func vt(obj uintptr, i int) uintptr {
	tbl := *(*uintptr)(unsafe.Pointer(obj))
	return *(*uintptr)(unsafe.Pointer(tbl + uintptr(i)*unsafe.Sizeof(uintptr(0))))
}

// folderDialog runs on a thread with COM set up for windows (the window's own thread).
func folderDialog(owner uintptr, title string) string {
	var dlg uintptr
	hr, _, _ := syscall.SyscallN(pCoCreateInstance.Addr(), uintptr(unsafe.Pointer(&clsidFileOpenDialog)), 0, clsctxInprocServer,
		uintptr(unsafe.Pointer(&iidIFileOpenDialog)), uintptr(unsafe.Pointer(&dlg)))
	if int32(hr) < 0 || dlg == 0 {
		logf("folder dialog: CoCreateInstance %08x", uint32(hr))
		return ""
	}
	defer syscall.SyscallN(vt(dlg, vRelease), dlg)
	var opts uint32
	if r, _, _ := syscall.SyscallN(vt(dlg, vGetOptions), dlg, uintptr(unsafe.Pointer(&opts))); int32(r) < 0 {
		opts = 0
	}
	syscall.SyscallN(vt(dlg, vSetOptions), dlg, uintptr(opts|fosPickFolders|fosForceFileSystem|fosPathMustExist|fosNoChangeDir))
	if t, err := windows.UTF16PtrFromString(title); err == nil {
		syscall.SyscallN(vt(dlg, vSetTitle), dlg, uintptr(unsafe.Pointer(t)))
		runtime.KeepAlive(t)
	}
	hr, _, _ = syscall.SyscallN(vt(dlg, vShow), dlg, owner)
	if int32(hr) < 0 {
		if uint32(hr) != hrCancelled {
			logf("folder dialog: Show %08x", uint32(hr))
		}
		return ""
	}
	var item uintptr
	if hr, _, _ = syscall.SyscallN(vt(dlg, vGetResult), dlg, uintptr(unsafe.Pointer(&item))); int32(hr) < 0 || item == 0 {
		logf("folder dialog: GetResult %08x", uint32(hr))
		return ""
	}
	defer syscall.SyscallN(vt(item, vRelease), item)
	var name *uint16
	if hr, _, _ = syscall.SyscallN(vt(item, vGetDisplayName), item, sigdnFileSysPath, uintptr(unsafe.Pointer(&name))); int32(hr) < 0 || name == nil {
		logf("folder dialog: GetDisplayName %08x", uint32(hr))
		return ""
	}
	defer windows.CoTaskMemFree(unsafe.Pointer(name))
	return windows.UTF16PtrToString(name)
}

// pickFolder asks for a folder. In the window it runs on the window's own thread (COM is ready there and the folder
// window stays in front of ours); without a window (tests) on a thread of its own.
func pickFolder() string {
	const title = "Soulrend 프로필 폴더나 CurseForge 모드 폴더를 골라 주세요"
	ch := make(chan string, 1)
	if w := mainWin; w != nil && w.hwnd != 0 {
		w.dispatch(func() { ch <- folderDialog(w.hwnd, title) })
		return <-ch
	}
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		if err := windows.CoInitializeEx(0, windows.COINIT_APARTMENTTHREADED); err == nil {
			defer windows.CoUninitialize()
		}
		ch <- folderDialog(0, title)
	}()
	return <-ch
}
