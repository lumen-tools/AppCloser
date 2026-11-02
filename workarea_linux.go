//go:build gui

package main

/*
#cgo LDFLAGS: -lX11
#include <X11/Xlib.h>
#include <X11/Xatom.h>

// Usable area of the desktop (without panels) from _NET_WORKAREA, or the
// screen size if the window manager does not publish it. 0 without X11.
static void work_area(int *w, int *h) {
	*w = 0; *h = 0;
	Display *d = XOpenDisplay(NULL);
	if (!d) return;
	Window root = DefaultRootWindow(d);
	*w = DisplayWidth(d, DefaultScreen(d));
	*h = DisplayHeight(d, DefaultScreen(d));
	Atom a = XInternAtom(d, "_NET_WORKAREA", True);
	Atom type; int fmt; unsigned long n, after; unsigned char *data = NULL;
	if (a != None && XGetWindowProperty(d, root, a, 0, 4, False, XA_CARDINAL, &type, &fmt, &n, &after, &data) == Success && data) {
		if (n >= 4 && fmt == 32) {
			long *v = (long *)data;
			if (v[2] > 0 && v[3] > 0) { *w = (int)v[2]; *h = (int)v[3]; }
		}
		XFree(data);
	}
	XCloseDisplay(d);
}
*/
import "C"

// workArea returns the usable screen area in pixels and an estimate of the
// display scale; the window's real scale corrects it after showing.
func workArea() (w, h, scale float32) {
	var cw, ch C.int
	C.work_area(&cw, &ch)
	return float32(cw), float32(ch), 1
}
