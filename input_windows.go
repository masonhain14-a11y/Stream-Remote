//go:build windows

package main

import (
	"errors"
	"runtime"
	"syscall"
	"unicode/utf16"
	"unsafe"
)

var (
	user32          = syscall.NewLazyDLL("user32.dll")
	shell32         = syscall.NewLazyDLL("shell32.dll")
	ole32           = syscall.NewLazyDLL("ole32.dll")
	sendInput       = user32.NewProc("SendInput")
	shellExecute    = shell32.NewProc("ShellExecuteW")
	coInitialize    = ole32.NewProc("CoInitializeEx")
	coUninitialize  = ole32.NewProc("CoUninitialize")
	errWindowsInput = errors.New("Windows blocked input. Focus a regular window on your PC.")
)

const (
	inputMouse    = 0
	inputKeyboard = 1
	mouseMove     = 0x0001
	leftDown      = 0x0002
	leftUp        = 0x0004
	rightDown     = 0x0008
	rightUp       = 0x0010
	mouseWheel    = 0x0800
	keyExtended   = 0x0001
	keyUp         = 0x0002
	keyUnicode    = 0x0004
)

// The Windows INPUT union is 32 bytes and starts at offset eight on x64 and ARM64.
type winInput struct {
	kind    uint32
	_       uint32
	payload [4]uint64
}

type mousePayload struct {
	x, y  int32
	data  uint32
	flags uint32
	time  uint32
	extra uintptr
}

type keyPayload struct {
	virtual, scan uint16
	flags, time   uint32
	extra         uintptr
}

type windowsDesktop struct {
	wheelRemainder int
}

func mouseEvent(flags uint32, x, y, data int) winInput {
	event := winInput{kind: inputMouse}
	*(*mousePayload)(unsafe.Pointer(&event.payload)) = mousePayload{
		x: int32(x), y: int32(y), data: uint32(int32(data)), flags: flags,
	}
	return event
}

func keyEvent(virtual, scan uint16, flags uint32) winInput {
	event := winInput{kind: inputKeyboard}
	*(*keyPayload)(unsafe.Pointer(&event.payload)) = keyPayload{virtual: virtual, scan: scan, flags: flags}
	return event
}

func send(events ...winInput) error {
	if len(events) == 0 {
		return nil
	}
	count, _, _ := sendInput.Call(uintptr(len(events)), uintptr(unsafe.Pointer(&events[0])), unsafe.Sizeof(winInput{}))
	runtime.KeepAlive(events)
	if int(count) != len(events) {
		return errWindowsInput
	}
	return nil
}

func (*windowsDesktop) move(x, y int) error {
	return send(mouseEvent(mouseMove, x, y, 0))
}

func (d *windowsDesktop) scroll(amount int) error {
	d.wheelRemainder += amount
	delta := d.wheelRemainder / 120 * 120
	if delta == 0 {
		return nil
	}
	d.wheelRemainder -= delta
	return send(mouseEvent(mouseWheel, 0, 0, delta))
}

func (*windowsDesktop) click(button string) error {
	down, up := uint32(leftDown), uint32(leftUp)
	if button == "right" {
		down, up = rightDown, rightUp
	}
	release := mouseEvent(up, 0, 0, 0)
	if err := send(mouseEvent(down, 0, 0, 0), release); err != nil {
		_ = send(release)
		return err
	}
	return nil
}

func (*windowsDesktop) holdLeft(down bool) error {
	flags := uint32(leftUp)
	if down {
		flags = leftDown
	}
	return send(mouseEvent(flags, 0, 0, 0))
}

type virtualKey struct {
	code  uint16
	flags uint32
}

var keys = map[string]virtualKey{
	"escape":     {0x1B, 0},
	"enter":      {0x0D, 0},
	"tab":        {0x09, 0},
	"backspace":  {0x08, 0},
	"space":      {0x20, 0},
	"left":       {0x25, keyExtended},
	"up":         {0x26, keyExtended},
	"right":      {0x27, keyExtended},
	"down":       {0x28, keyExtended},
	"previous":   {0xB1, keyExtended},
	"next":       {0xB0, keyExtended},
	"playPause":  {0xB3, keyExtended},
	"mute":       {0xAD, keyExtended},
	"volumeDown": {0xAE, keyExtended},
	"volumeUp":   {0xAF, keyExtended},
	"fullscreen": {0x7A, 0},
}

func appendKey(events []winInput, key virtualKey) []winInput {
	return append(events, keyEvent(key.code, 0, key.flags), keyEvent(key.code, 0, key.flags|keyUp))
}

func sendChord(modifier, key virtualKey) error {
	release := keyEvent(modifier.code, 0, modifier.flags|keyUp)
	events := appendKey([]winInput{keyEvent(modifier.code, 0, modifier.flags)}, key)
	if err := send(append(events, release)...); err != nil {
		_ = send(release)
		return err
	}
	return nil
}

func (*windowsDesktop) key(name string) error {
	switch name {
	case "browserBack":
		return sendChord(virtualKey{0x12, 0}, keys["left"])
	case "desktop":
		return sendChord(virtualKey{0x5B, keyExtended}, virtualKey{'D', 0})
	case "selectAll":
		return sendChord(virtualKey{0x11, 0}, virtualKey{'A', 0})
	}
	key, ok := keys[name]
	if !ok {
		return errors.New("Unknown key.")
	}
	return send(appendKey(nil, key)...)
}

func (*windowsDesktop) typeText(value string, erase int) error {
	events := make([]winInput, 0, 2*(len(value)+erase))
	for i := 0; i < erase; i++ {
		events = appendKey(events, keys["backspace"])
	}
	for _, unit := range utf16.Encode([]rune(value)) {
		switch {
		case unit == '\n':
			events = appendKey(events, keys["enter"])
		case unit == '\t':
			events = appendKey(events, keys["tab"])
		case unit >= 32 && unit != 127:
			events = append(events, keyEvent(0, unit, keyUnicode), keyEvent(0, unit, keyUnicode|keyUp))
		}
	}
	return send(events...)
}

func (*windowsDesktop) open(url string) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if result, _, _ := coInitialize.Call(0, 2|4); int32(result) >= 0 {
		defer coUninitialize.Call()
	}
	result, _, _ := shellExecute.Call(0, uintptr(unsafe.Pointer(wide("open"))), uintptr(unsafe.Pointer(wide(url))), 0, 0, 1)
	if result <= 32 {
		return errors.New("Windows couldn't open the default browser.")
	}
	return nil
}
