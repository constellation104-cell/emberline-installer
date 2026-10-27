//go:build windows

package main

import (
	"math"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/wailsapp/go-webview2/pkg/edge"
	"github.com/wailsapp/go-webview2/webviewloader"
	"golang.org/x/sys/windows"
)

// A plain Win32 window hosting WebView2. The WebView2 loader used here is the pure-Go one
// (no DLL embedded in the exe and no in-memory module loading).

var (
	user32              = windows.NewLazySystemDLL("user32.dll")
	pRegisterClassExW   = user32.NewProc("RegisterClassExW")
	pCreateWindowExW    = user32.NewProc("CreateWindowExW")
	pDefWindowProcW     = user32.NewProc("DefWindowProcW")
	pShowWindow         = user32.NewProc("ShowWindow")
	pUpdateWindow       = user32.NewProc("UpdateWindow")
	pGetMessageW        = user32.NewProc("GetMessageW")
	pTranslateMessage   = user32.NewProc("TranslateMessage")
	pDispatchMessageW   = user32.NewProc("DispatchMessageW")
	pPostQuitMessage    = user32.NewProc("PostQuitMessage")
	pPostMessageW       = user32.NewProc("PostMessageW")
	pDestroyWindow      = user32.NewProc("DestroyWindow")
	pLoadImageW         = user32.NewProc("LoadImageW")
	pLoadCursorW        = user32.NewProc("LoadCursorW")
	pGetSystemMetrics   = user32.NewProc("GetSystemMetrics")
	pSetWindowPos       = user32.NewProc("SetWindowPos")
	pGetDpiForSystem    = user32.NewProc("GetDpiForSystem")
	pGetDpiForWindow    = user32.NewProc("GetDpiForWindow")
	pSystemParamsInfoW  = user32.NewProc("SystemParametersInfoW")
	pFindWindowW        = user32.NewProc("FindWindowW")
	pSetForegroundWin   = user32.NewProc("SetForegroundWindow")
)

const (
	wmDestroy        = 0x0002
	wmMove           = 0x0003
	wmSize           = 0x0005
	wmActivate       = 0x0006
	wmClose          = 0x0010
	wmQuit           = 0x0012
	wmGetMinMaxInfo  = 0x0024
	wmMoving         = 0x0216
	wmDpiChanged     = 0x02E0
	wmApp            = 0x8000
	wsOverlappedWin  = 0x00CF0000
	swShow           = 5
	imageIcon        = 1
	lrDefaultSize    = 0x0040
	lrShared         = 0x8000
	idcArrow         = 32512
	smCxScreen       = 0
	smCyScreen       = 1
	swpNoZOrder      = 0x0004
	swpNoActivate    = 0x0010
	baseW, baseH     = 860, 700
	minW, minH       = 720, 560
)

type wndClassExW struct {
	Size       uint32
	Style      uint32
	WndProc    uintptr
	ClsExtra   int32
	WndExtra   int32
	Instance   windows.Handle
	Icon       windows.Handle
	Cursor     windows.Handle
	Background windows.Handle
	MenuName   *uint16
	ClassName  *uint16
	IconSm     windows.Handle
}

type point struct{ X, Y int32 }

type msgW struct {
	Hwnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      point
	Private uint32
}

type minMaxInfo struct{ Reserved, MaxSize, MaxPosition, MinTrackSize, MaxTrackSize point }

type rect struct{ Left, Top, Right, Bottom int32 }

type window struct {
	hwnd     uintptr
	chromium *edge.Chromium
	ready    atomic.Bool
	scale    float64
	mu       sync.Mutex
	queue    []func()
}

var mainWin *window

func wndProc(hwnd, msg, wp, lp uintptr) uintptr {
	w := mainWin
	if w != nil && w.hwnd == hwnd {
		switch msg {
		case wmApp: // work queued from other goroutines (window messages also arrive during move/resize loops)
			w.drain()
			return 0
		case wmMove, wmMoving:
			if w.ready.Load() {
				_ = w.chromium.NotifyParentWindowPositionChanged()
			}
		case wmSize:
			if w.ready.Load() {
				w.chromium.Resize()
			}
		case wmGetMinMaxInfo:
			mmi := (*minMaxInfo)(unsafe.Pointer(lp))
			mw, mh := int32(float64(minW)*w.scale), int32(float64(minH)*w.scale)
			if wa := workArea(); wa.Right > wa.Left {
				if mw > wa.Right-wa.Left {
					mw = wa.Right - wa.Left
				}
				if mh > wa.Bottom-wa.Top {
					mh = wa.Bottom - wa.Top
				}
			}
			mmi.MinTrackSize = point{mw, mh}
			return 0
		case wmDpiChanged:
			r := (*rect)(unsafe.Pointer(lp))
			w.scale = float64(uint16(wp>>16)) / 96
			_, _, _ = pSetWindowPos.Call(hwnd, 0, uintptr(r.Left), uintptr(r.Top), uintptr(r.Right-r.Left), uintptr(r.Bottom-r.Top), swpNoZOrder|swpNoActivate)
			if w.ready.Load() {
				w.applyScale()
			}
			return 0
		case wmClose:
			if !w.ready.Load() { // WebView2 still starting: leaving Embed's own message pump early would crash
				os.Exit(0)
			}
			_, _, _ = pDestroyWindow.Call(hwnd)
			return 0
		case wmDestroy:
			_, _, _ = pPostQuitMessage.Call(0)
			return 0
		}
	}
	r, _, _ := pDefWindowProcW.Call(hwnd, msg, wp, lp)
	return r
}

func (w *window) dispatch(f func()) {
	w.mu.Lock()
	w.queue = append(w.queue, f)
	w.mu.Unlock()
	_, _, _ = pPostMessageW.Call(w.hwnd, wmApp, 0, 0)
}

func (w *window) drain() {
	w.mu.Lock()
	q := w.queue
	w.queue = nil
	w.mu.Unlock()
	for _, f := range q {
		f()
	}
}

// the controller works in raw pixels and does not follow monitor changes by itself, so the page zoom is set here
func (w *window) applyScale() {
	if c3 := w.chromium.GetController().GetICoreWebView2Controller3(); c3 != nil {
		// put_RasterizationScale takes the double by value (the fork's wrapper passes a pointer)
		_, _, _ = c3.Vtbl.PutRasterizationScale.Call(uintptr(unsafe.Pointer(c3)), uintptr(math.Float64bits(w.scale)))
		_, _, _ = c3.Vtbl.Release.Call(uintptr(unsafe.Pointer(c3)))
	}
	w.chromium.Resize()
}

func workArea() rect {
	var r rect
	_, _, _ = pSystemParamsInfoW.Call(0x0030, 0, uintptr(unsafe.Pointer(&r)), 0) // SPI_GETWORKAREA
	return r
}

func windowScale(hwnd uintptr) float64 {
	if pGetDpiForWindow.Find() == nil {
		if d, _, _ := pGetDpiForWindow.Call(hwnd); d >= 96 {
			return float64(d) / 96
		}
	}
	return systemScale()
}

func systemScale() float64 {
	if pGetDpiForSystem.Find() == nil {
		if d, _, _ := pGetDpiForSystem.Call(); d >= 96 {
			return float64(d) / 96
		}
	}
	return 1
}

var instanceMutex windows.Handle

// singleInstance keeps one installer window; a second start just brings the first one to the front.
func singleInstance() bool {
	name, _ := windows.UTF16PtrFromString(`Local\EmberlineInstaller`)
	h, err := windows.CreateMutex(nil, false, name)
	if err == windows.ERROR_ALREADY_EXISTS {
		cls, _ := windows.UTF16PtrFromString("EmberlineInstallerWindow")
		if hw, _, _ := pFindWindowW.Call(uintptr(unsafe.Pointer(cls)), 0); hw != 0 {
			_, _, _ = pShowWindow.Call(hw, 9) // SW_RESTORE
			_, _, _ = pSetForegroundWin.Call(hw)
		}
		return false
	}
	instanceMutex = h
	return true
}

func runGUI(env *Env) {
	if !singleInstance() {
		return
	}
	if v, err := webviewloader.GetAvailableCoreWebView2BrowserVersionString(""); err != nil || v == "" {
		logf("webview2 runtime missing: %q %v", v, err)
		messageBox("잔불 서버 설치", "이 PC에는 Microsoft Edge WebView2 런타임이 없어서 설치 화면을 열 수 없어요.\n\n확인을 누르면 Microsoft 공식 다운로드 페이지가 열려요. 설치한 뒤 다시 실행해 주세요.")
		openURL("https://developer.microsoft.com/microsoft-edge/webview2/")
		return
	}

	w := &window{scale: systemScale()}
	mainWin = w

	var inst windows.Handle
	_ = windows.GetModuleHandleEx(0, nil, &inst)
	icon, _, _ := pLoadImageW.Call(uintptr(inst), 1, imageIcon, 0, 0, lrDefaultSize|lrShared)
	cursor, _, _ := pLoadCursorW.Call(0, idcArrow)
	cls, _ := windows.UTF16PtrFromString("EmberlineInstallerWindow")
	wc := wndClassExW{
		WndProc:   windows.NewCallback(wndProc),
		Instance:  inst,
		Icon:      windows.Handle(icon),
		IconSm:    windows.Handle(icon),
		Cursor:    windows.Handle(cursor),
		ClassName: cls,
	}
	wc.Size = uint32(unsafe.Sizeof(wc))
	_, _, _ = pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))

	width, height := int(float64(baseW)*w.scale), int(float64(baseH)*w.scale)
	wa := workArea()
	if wa.Right <= wa.Left {
		sw, _, _ := pGetSystemMetrics.Call(smCxScreen)
		sh, _, _ := pGetSystemMetrics.Call(smCyScreen)
		wa = rect{0, 0, int32(sw), int32(sh)}
	}
	ww, wh := int(wa.Right-wa.Left), int(wa.Bottom-wa.Top)
	if width > ww-24 {
		width = ww - 24
	}
	if height > wh-24 {
		height = wh - 24
	}
	x, y := int(wa.Left)+(ww-width)/2, int(wa.Top)+(wh-height)/2
	title, _ := windows.UTF16PtrFromString("잔불 서버 설치")
	w.hwnd, _, _ = pCreateWindowExW.Call(0, uintptr(unsafe.Pointer(cls)), uintptr(unsafe.Pointer(title)), wsOverlappedWin,
		uintptr(x), uintptr(y), uintptr(width), uintptr(height), 0, 0, uintptr(inst), 0)
	if w.hwnd == 0 {
		messageBox("잔불 서버 설치", "창을 만들지 못했어요.")
		return
	}
	_, _, _ = pShowWindow.Call(w.hwnd, swShow)
	_, _, _ = pUpdateWindow.Call(w.hwnd)

	c := edge.NewChromium()
	w.chromium = c
	c.DataPath = filepath.Join(os.TempDir(), "emberline-installer", "webview")
	c.SetErrorCallback(func(err error) {
		logf("webview2 error: %v", err)
		messageBox("잔불 서버 설치", "화면 표시 중 문제가 생겨 설치기를 닫아요.\n다시 실행해 주세요.\n\n"+err.Error())
		os.Exit(1)
	})
	ui := &UI{env: env}
	ui.eval = func(js string) {
		w.dispatch(func() {
			if w.ready.Load() {
				c.Eval(js)
			}
		})
	}
	ui.quit = func() { _, _, _ = pPostMessageW.Call(w.hwnd, wmClose, 0, 0) }
	c.MessageCallback = func(message string, _ *edge.ICoreWebView2, _ *edge.ICoreWebView2WebMessageReceivedEventArgs) {
		ui.handle(message)
	}
	if !c.Embed(w.hwnd) {
		messageBox("잔불 서버 설치", "화면을 준비하지 못했어요.")
		return
	}
	debug := os.Getenv("EMBERLINE_DEBUG") == "1"
	if s, err := c.GetSettings(); err == nil {
		_ = s.PutAreDefaultContextMenusEnabled(debug)
		_ = s.PutAreDevToolsEnabled(debug)
		_ = s.PutIsStatusBarEnabled(false)
		_ = s.PutIsZoomControlEnabled(false)
	}
	c.SetBackgroundColour(20, 21, 25, 255)
	_ = c.AllowExternalDrag(false)
	w.ready.Store(true)
	w.scale = windowScale(w.hwnd)
	w.applyScale()
	c.NavigateToString(indexHTML)

	var m msgW
	for {
		r, _, _ := pGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		_, _, _ = pTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		_, _, _ = pDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
	w.ready.Store(false)
	c.ShuttingDown()
}
