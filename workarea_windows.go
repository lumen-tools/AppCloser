//go:build gui

package main

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// workArea returns the usable screen area (without the taskbar) in pixels and
// the display scale (1.0 = 96 dpi).
func workArea() (w, h, scale float32) {
	user32 := windows.NewLazySystemDLL("user32.dll")
	var r struct{ Left, Top, Right, Bottom int32 }
	const spiGetWorkArea = 0x0030
	if ok, _, _ := user32.NewProc("SystemParametersInfoW").Call(spiGetWorkArea, 0, uintptr(unsafe.Pointer(&r)), 0); ok == 0 {
		return 0, 0, 1
	}
	scale = 1
	if p := user32.NewProc("GetDpiForSystem"); p.Find() == nil {
		if dpi, _, _ := p.Call(); dpi > 0 {
			scale = float32(dpi) / 96
		}
	}
	return float32(r.Right - r.Left), float32(r.Bottom - r.Top), scale
}
