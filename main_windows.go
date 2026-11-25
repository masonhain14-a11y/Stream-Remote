//go:build windows

package main

import (
	"fmt"
	"net"
	"runtime"
	"slices"
	"syscall"
	"unsafe"

	"github.com/skip2/go-qrcode"
)

var (
	kernel32          = syscall.NewLazyDLL("kernel32.dll")
	gdi32             = syscall.NewLazyDLL("gdi32.dll")
	getModuleHandle   = kernel32.NewProc("GetModuleHandleW")
	globalAlloc       = kernel32.NewProc("GlobalAlloc")
	globalLock        = kernel32.NewProc("GlobalLock")
	globalUnlock      = kernel32.NewProc("GlobalUnlock")
	globalFree        = kernel32.NewProc("GlobalFree")
	moveMemory        = syscall.NewLazyDLL("ntdll.dll").NewProc("RtlMoveMemory")
	registerClass     = user32.NewProc("RegisterClassExW")
	createWindow      = user32.NewProc("CreateWindowExW")
	defWindowProc     = user32.NewProc("DefWindowProcW")
	destroyWindow     = user32.NewProc("DestroyWindow")
	showWindow        = user32.NewProc("ShowWindow")
	updateWindow      = user32.NewProc("UpdateWindow")
	getMessage        = user32.NewProc("GetMessageW")
	isDialogMessage   = user32.NewProc("IsDialogMessageW")
	translateMessage  = user32.NewProc("TranslateMessage")
	dispatchMessage   = user32.NewProc("DispatchMessageW")
	sendMessage       = user32.NewProc("SendMessageW")
	postQuitMessage   = user32.NewProc("PostQuitMessage")
	messageBox        = user32.NewProc("MessageBoxW")
	adjustWindowRect  = user32.NewProc("AdjustWindowRect")
	loadCursor        = user32.NewProc("LoadCursorW")
	loadIcon          = user32.NewProc("LoadIconW")
	setTimer          = user32.NewProc("SetTimer")
	invalidateRect    = user32.NewProc("InvalidateRect")
	beginPaint        = user32.NewProc("BeginPaint")
	endPaint          = user32.NewProc("EndPaint")
	fillRect          = user32.NewProc("FillRect")
	openClipboard     = user32.NewProc("OpenClipboard")
	closeClipboard    = user32.NewProc("CloseClipboard")
	emptyClipboard    = user32.NewProc("EmptyClipboard")
	setClipboardData  = user32.NewProc("SetClipboardData")
	createSolidBrush  = gdi32.NewProc("CreateSolidBrush")
	selectObject      = gdi32.NewProc("SelectObject")
	deleteObject      = gdi32.NewProc("DeleteObject")
	createFont        = gdi32.NewProc("CreateFontW")
	setTextColor      = gdi32.NewProc("SetTextColor")
	setBackgroundMode = gdi32.NewProc("SetBkMode")
	textOut           = gdi32.NewProc("TextOutW")
)

const (
	wmDestroy      = 0x0002
	wmPaint        = 0x000F
	wmEraseBkgnd   = 0x0014
	wmSetFont      = 0x0030
	wmCommand      = 0x0111
	wmTimer        = 0x0113
	cbAddString    = 0x0143
	cbGetCurSel    = 0x0147
	cbResetContent = 0x014B
	cbSetCurSel    = 0x014E
	cbnSelChange   = 1
	copyID         = 101
	resetID        = 102
	networkID      = 103
	clientWidth    = 516
	clientHeight   = 516
)

type rect struct{ left, top, right, bottom int32 }
type point struct{ x, y int32 }
type windowClass struct {
	size, style            uint32
	callback               uintptr
	classExtra, winExtra   int32
	instance, icon, cursor uintptr
	background, menu, name uintptr
	smallIcon              uintptr
}
type winMessage struct {
	window         uintptr
	message        uint32
	wParam, lParam uintptr
	time           uint32
	pos            point
	private        uint32
}
type paintData struct {
	dc              uintptr
	erase           int32
	region          rect
	restore, update int32
	reserved        [32]byte
}

type window struct {
	remote                    *remote
	handle, instance          uintptr
	network                   uintptr
	font, small, label, title uintptr
	heading, mono, codeFont   uintptr
	port, addressIndex, count int
	addresses                 []address
	pin, pairURL, info        string
	qr                        [][]bool
}

func wide(text string) *uint16 {
	p, _ := syscall.UTF16PtrFromString(text)
	return p
}

func rgb(hex uint32) uintptr {
	return uintptr((hex&0xff0000)>>16 | hex&0x00ff00 | (hex&0x0000ff)<<16)
}

func font(size int, weight uintptr, face string) uintptr {
	result, _, _ := createFont.Call(uintptr(-size), 0, 0, 0, weight, 0, 0, 0, 1, 0, 0, 5, 0, uintptr(unsafe.Pointer(wide(face))))
	return result
}

func drawText(dc, f uintptr, x, y int, color uint32, label string) {
	chars, _ := syscall.UTF16FromString(label)
	if len(chars) < 2 {
		return
	}
	previous, _, _ := selectObject.Call(dc, f)
	setTextColor.Call(dc, rgb(color))
	textOut.Call(dc, uintptr(x), uintptr(y), uintptr(unsafe.Pointer(&chars[0])), uintptr(len(chars)-1))
	runtime.KeepAlive(chars)
	selectObject.Call(dc, previous)
}

func fill(dc uintptr, left, top, right, bottom int, brush uintptr) {
	r := rect{int32(left), int32(top), int32(right), int32(bottom)}
	fillRect.Call(dc, uintptr(unsafe.Pointer(&r)), brush)
}

func block(dc uintptr, left, top, right, bottom int, color uint32) {
	brush, _, _ := createSolidBrush.Call(rgb(color))
	fill(dc, left, top, right, bottom, brush)
	deleteObject.Call(brush)
}

func (w *window) paint() {
	var paint paintData
	dc, _, _ := beginPaint.Call(w.handle, uintptr(unsafe.Pointer(&paint)))
	defer endPaint.Call(w.handle, uintptr(unsafe.Pointer(&paint)))
	setBackgroundMode.Call(dc, 1)
	block(dc, 0, 0, clientWidth, clientHeight, 0xf3f3ef)
	block(dc, 0, 0, clientWidth, 92, 0x171b27)
	block(dc, 25, 22, 70, 67, 0xd6f477)
	drawText(dc, w.heading, 33, 28, 0x192324, "SR")
	drawText(dc, w.title, 83, 17, 0xffffff, "Stream Remote")
	drawText(dc, w.font, 85, 53, 0xc4c8d5, "Use your phone to control this PC.")

	drawText(dc, w.heading, 24, 113, 0x191d29, "Connect your phone")
	drawText(dc, w.font, 24, 145, 0x555c68, "Put both devices on the same Wi-Fi or local network.")

	block(dc, 24, 177, 229, 382, 0xffffff)
	if len(w.qr) > 0 {
		pixel := 205 / len(w.qr)
		margin := (205 - pixel*len(w.qr)) / 2
		brush, _, _ := createSolidBrush.Call(rgb(0x171b27))
		for y, row := range w.qr {
			for x, dark := range row {
				if dark {
					left, top := 24+margin+x*pixel, 177+margin+y*pixel
					fill(dc, left, top, left+pixel, top+pixel, brush)
				}
			}
		}
		deleteObject.Call(brush)
	} else {
		drawText(dc, w.font, 44, 266, 0x555c68, "Waiting for a network…")
	}
	drawText(dc, w.label, 24, 389, 0x555c68, "Scan with your phone’s camera")

	drawText(dc, w.label, 248, 184, 0x555c68, "Browser address")
	drawText(dc, w.mono, 248, 209, 0x191d29, w.pairURL)
	drawText(dc, w.label, 248, 292, 0x555c68, "Manual pairing code")
	block(dc, 248, 317, 411, 366, 0xe4efcc)
	drawText(dc, w.codeFont, 259, 320, 0x26391c, w.pin)
	drawText(dc, w.small, 248, 374, 0x555c68, "Enter this code in your phone browser.")

	drawText(dc, w.label, 24, 433, 0x555c68, "Network")
	block(dc, 0, 475, clientWidth, 476, 0xe0e1d8)
	status, color := "Waiting for a phone", uint32(0x7b8190)
	if len(w.addresses) == 0 {
		status = "Waiting for a network"
	}
	if w.count != 0 {
		color = 0x548045
		status = fmt.Sprintf("%d browsers connected", w.count)
		if w.count == 1 {
			status = "1 browser connected"
		}
	}
	if w.info != "" {
		status = w.info
	}
	block(dc, 25, 494, 34, 503, color)
	drawText(dc, w.label, 41, 489, 0x555c68, status)
}

func (w *window) refreshPairing() {
	pin, secret, count := w.remote.status()
	w.pin, w.count = pin, count
	w.pairURL = ""
	w.qr = nil
	if len(w.addresses) != 0 {
		w.pairURL = fmt.Sprintf("http://%s:%d", w.addresses[w.addressIndex].prefix.Addr().String(), w.port)
		code, err := qrcode.New(w.pairURL+"/#pair="+secret, qrcode.Medium)
		if err == nil {
			w.qr = code.Bitmap()
		}
	}
	invalidateRect.Call(w.handle, 0, 0)
}

func (w *window) refreshNetworks() {
	list := addresses()
	if slices.Equal(w.addresses, list) {
		return
	}
	var chosen string
	if len(w.addresses) > 0 {
		chosen = w.addresses[w.addressIndex].prefix.Addr().String()
	}
	w.addresses, w.addressIndex = list, 0
	sendMessage.Call(w.network, cbResetContent, 0, 0)
	for i, item := range list {
		sendMessage.Call(w.network, cbAddString, 0, uintptr(unsafe.Pointer(wide(item.name+"  ("+item.prefix.Addr().String()+")"))))
		if item.prefix.Addr().String() == chosen {
			w.addressIndex = i
		}
	}
	if len(list) == 0 {
		sendMessage.Call(w.network, cbAddString, 0, uintptr(unsafe.Pointer(wide("No network address found"))))
	}
	sendMessage.Call(w.network, cbSetCurSel, uintptr(w.addressIndex), 0)
	w.refreshPairing()
}

func child(w *window, kind, title string, style uintptr, x, y, width, height, id int) uintptr {
	const visibleChild = 0x50000000
	handle, _, _ := createWindow.Call(0, uintptr(unsafe.Pointer(wide(kind))), uintptr(unsafe.Pointer(wide(title))), visibleChild|style,
		uintptr(x), uintptr(y), uintptr(width), uintptr(height), w.handle, uintptr(id), w.instance, 0)
	sendMessage.Call(handle, wmSetFont, w.font, 1)
	return handle
}

func copyText(handle uintptr, value string) bool {
	data, err := syscall.UTF16FromString(value)
	if err != nil {
		return false
	}
	bytes := len(data) * 2
	memory, _, _ := globalAlloc.Call(0x0002, uintptr(bytes))
	if memory == 0 {
		return false
	}
	defer func() {
		if memory != 0 {
			globalFree.Call(memory)
		}
	}()
	dest, _, _ := globalLock.Call(memory)
	if dest == 0 {
		return false
	}
	moveMemory.Call(dest, uintptr(unsafe.Pointer(&data[0])), uintptr(bytes))
	globalUnlock.Call(memory)
	ok, _, _ := openClipboard.Call(handle)
	if ok == 0 {
		return false
	}
	defer closeClipboard.Call()
	ok, _, _ = emptyClipboard.Call()
	if ok == 0 {
		return false
	}
	ok, _, _ = setClipboardData.Call(13, memory)
	if ok == 0 {
		return false
	}
	memory = 0
	return true
}

func (w *window) procedure(handle uintptr, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case wmEraseBkgnd:
		return 1
	case wmPaint:
		if w.handle != 0 {
			w.paint()
			return 0
		}
	case wmTimer:
		w.refreshNetworks()
		_, _, count := w.remote.status()
		if count != w.count {
			w.count = count
			w.info = ""
			invalidateRect.Call(handle, 0, 0)
		}
		return 0
	case wmCommand:
		w.info = ""
		switch id, notice := wParam&0xffff, (wParam>>16)&0xffff; {
		case id == copyID && notice == 0:
			w.info = "No network address to copy"
			if w.pairURL != "" {
				w.info = "Couldn't copy the address"
				if copyText(handle, w.pairURL) {
					w.info = "Browser address copied"
				}
			}
			invalidateRect.Call(handle, 0, 0)
		case id == resetID && notice == 0:
			w.remote.reset()
			w.info = "Pairing code refreshed"
			w.refreshPairing()
		case id == networkID && notice == cbnSelChange:
			i, _, _ := sendMessage.Call(w.network, cbGetCurSel, 0, 0)
			if i < uintptr(len(w.addresses)) {
				w.addressIndex = int(i)
				w.refreshPairing()
			}
		}
		return 0
	case wmDestroy:
		postQuitMessage.Call(0)
		return 0
	}
	result, _, _ := defWindowProc.Call(handle, uintptr(msg), wParam, lParam)
	return result
}

func (w *window) run() error {
	instance, _, _ := getModuleHandle.Call(0)
	w.instance = instance
	w.font = font(16, 400, "Segoe UI")
	w.small = font(14, 400, "Segoe UI")
	w.label = font(15, 600, "Segoe UI")
	w.title = font(30, 700, "Segoe UI")
	w.heading = font(23, 700, "Segoe UI")
	w.mono = font(15, 400, "Consolas")
	w.codeFont = font(36, 700, "Consolas")
	defer func() {
		for _, handle := range []uintptr{w.font, w.small, w.label, w.title, w.heading, w.mono, w.codeFont} {
			deleteObject.Call(handle)
		}
	}()
	className, _ := syscall.UTF16PtrFromString("StreamRemoteWindow")
	icon, _, _ := loadIcon.Call(instance, 1)
	if icon == 0 {
		icon, _, _ = loadIcon.Call(0, 32512)
	}
	cursor, _, _ := loadCursor.Call(0, 32512)
	class := windowClass{
		size: uint32(unsafe.Sizeof(windowClass{})), callback: syscall.NewCallback(w.procedure),
		instance: instance, icon: icon, cursor: cursor, name: uintptr(unsafe.Pointer(className)), smallIcon: icon,
	}
	if result, _, err := registerClass.Call(uintptr(unsafe.Pointer(&class))); result == 0 {
		return err
	}
	const style = 0x00CA0000
	size := rect{0, 0, clientWidth, clientHeight}
	adjustWindowRect.Call(uintptr(unsafe.Pointer(&size)), style, 0)
	h, _, err := createWindow.Call(0, uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(wide("Stream Remote"))), style,
		0x80000000, 0x80000000, uintptr(size.right-size.left), uintptr(size.bottom-size.top),
		0, 0, instance, 0)
	runtime.KeepAlive(className)
	if h == 0 {
		return err
	}
	w.handle = h
	_ = child(w, "BUTTON", "Copy address", 0x00010000, 248, 241, 130, 32, copyID)
	_ = child(w, "BUTTON", "Reset pairing", 0x00010000, 356, 481, 136, 31, resetID)
	w.network = child(w, "COMBOBOX", "", 0x00210003, 99, 427, 392, 190, networkID)
	w.refreshPairing()
	w.refreshNetworks()
	if len(w.addresses) == 0 {
		sendMessage.Call(w.network, cbAddString, 0, uintptr(unsafe.Pointer(wide("No network address found"))))
		sendMessage.Call(w.network, cbSetCurSel, 0, 0)
	}
	setTimer.Call(w.handle, 1, 1800, 0)
	showWindow.Call(w.handle, 1)
	updateWindow.Call(w.handle)
	var msg winMessage
	for {
		status, _, err := getMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(status) < 0 {
			destroyWindow.Call(w.handle)
			return err
		}
		if status == 0 {
			return nil
		}
		if handled, _, _ := isDialogMessage.Call(w.handle, uintptr(unsafe.Pointer(&msg))); handled == 0 {
			translateMessage.Call(uintptr(unsafe.Pointer(&msg)))
			dispatchMessage.Call(uintptr(unsafe.Pointer(&msg)))
		}
	}
}

func main() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	h := newRemote(&windowsDesktop{})
	server, listener, err := startServer(h, "0.0.0.0:8765")
	if err != nil {
		messageBox.Call(0, uintptr(unsafe.Pointer(wide("Couldn't start the local server. "+err.Error()))), uintptr(unsafe.Pointer(wide("Stream Remote"))), 0x10)
		return
	}
	defer server.Close()
	defer h.reset()
	go func() { _ = server.Serve(listener) }()
	w := window{remote: h, port: listener.Addr().(*net.TCPAddr).Port}
	if err := w.run(); err != nil {
		messageBox.Call(0, uintptr(unsafe.Pointer(wide("Couldn't open Stream Remote. "+err.Error()))), uintptr(unsafe.Pointer(wide("Stream Remote"))), 0x10)
	}
}
