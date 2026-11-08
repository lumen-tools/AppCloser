//go:build gui

package main

/*
#cgo CFLAGS: -x objective-c -mmacosx-version-min=11.0
#cgo LDFLAGS: -framework AppKit
#import <AppKit/AppKit.h>

// Usable area of the main screen (without menu bar and Dock), in points.
static void ac_work_area(double *w, double *h) {
	NSRect r = [[NSScreen mainScreen] visibleFrame];
	*w = r.size.width;
	*h = r.size.height;
}
*/
import "C"

// workArea returns the free screen area in points; window sizes on macOS are
// in points as well, so the scale is 1.
func workArea() (w, h, scale float32) {
	var cw, ch C.double
	C.ac_work_area(&cw, &ch)
	return float32(cw), float32(ch), 1
}
