// Command mkicon writes winres/AppCloser.ico for the Windows resource and
// logo.png for documentation.
package main

import (
	"os"

	"appcloser/logo"
)

func main() {
	os.WriteFile("winres/AppCloser.ico", logo.ICO(logo.Blue, 16, 24, 32, 48, 64, 256), 0o644)
	os.WriteFile("logo.png", logo.PNG(256, logo.Blue), 0o644)
}
