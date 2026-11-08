// Command mkicon writes winres/AppCloser.ico for the Windows resource and
// docs/logo.png. With -iconset DIR it writes a macOS iconset instead (for
// iconutil), with the margin macOS app icons use.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"

	"appcloser/logo"
)

func main() {
	iconset := flag.String("iconset", "", "write a macOS .iconset into this directory")
	flag.Parse()
	if *iconset != "" {
		writeIconset(*iconset)
		return
	}
	os.WriteFile("winres/AppCloser.ico", logo.ICO(logo.Blue, 16, 24, 32, 48, 64, 256), 0o644)
	os.WriteFile("docs/logo.png", logo.PNG(256, logo.Blue), 0o644)
}

// macIcon places the logo on a transparent canvas: macOS icons keep about 10 %
// free space on each side (824 of 1024 pixels).
func macIcon(size int) []byte {
	inner := size * 824 / 1024
	canvas := image.NewNRGBA(image.Rect(0, 0, size, size))
	off := (size - inner) / 2
	draw.Draw(canvas, image.Rect(off, off, off+inner, off+inner), logo.Draw(inner, logo.Blue), image.Point{}, draw.Over)
	var buf bytes.Buffer
	png.Encode(&buf, canvas)
	return buf.Bytes()
}

func writeIconset(dir string) {
	os.MkdirAll(dir, 0o755)
	for _, s := range []int{16, 32, 128, 256, 512} {
		os.WriteFile(filepath.Join(dir, fmt.Sprintf("icon_%dx%d.png", s, s)), macIcon(s), 0o644)
		os.WriteFile(filepath.Join(dir, fmt.Sprintf("icon_%dx%d@2x.png", s, s)), macIcon(2*s), 0o644)
	}
}
