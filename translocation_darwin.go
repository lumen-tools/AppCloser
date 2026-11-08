package main

/*
#cgo LDFLAGS: -framework CoreFoundation
#include <CoreFoundation/CoreFoundation.h>
#include <dlfcn.h>
#include <limits.h>
#include <stdlib.h>
#include <string.h>

typedef CFURLRef (*ac_original_fn)(CFURLRef, CFErrorRef *);

// Asks macOS where a translocated app really is (SecTranslocate, part of the
// Security framework since macOS 10.12). Returns NULL if unknown.
static char *ac_original_path(const char *path) {
	void *h = dlopen("/System/Library/Frameworks/Security.framework/Security", RTLD_LAZY);
	if (!h) return NULL;
	ac_original_fn f = (ac_original_fn)dlsym(h, "SecTranslocateCreateOriginalPathForURL");
	if (!f) return NULL;
	CFURLRef in = CFURLCreateFromFileSystemRepresentation(NULL, (const UInt8 *)path, strlen(path), true);
	if (!in) return NULL;
	CFURLRef out = f(in, NULL);
	CFRelease(in);
	if (!out) return NULL;
	char buf[PATH_MAX];
	Boolean ok = CFURLGetFileSystemRepresentation(out, true, (UInt8 *)buf, sizeof buf);
	CFRelease(out);
	return ok ? strdup(buf) : NULL;
}
*/
import "C"

import (
	"strings"
	"unsafe"
)

// originalAppPath returns where AppCloser.app really is when macOS runs it
// from a hidden copy ("App Translocation" of downloaded apps), else "".
func originalAppPath(app string) string {
	if !strings.Contains(app, "/AppTranslocation/") {
		return ""
	}
	p := C.CString(app)
	defer C.free(unsafe.Pointer(p))
	r := C.ac_original_path(p)
	if r == nil {
		return ""
	}
	defer C.free(unsafe.Pointer(r))
	return C.GoString(r)
}
