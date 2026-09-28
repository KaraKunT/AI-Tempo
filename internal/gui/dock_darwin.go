//go:build darwin

package gui

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa
#import <Cocoa/Cocoa.h>
#include <stdlib.h>

static void hideFromDock(void) {
	[NSApp setActivationPolicy:NSApplicationActivationPolicyAccessory];
}

static void activateApp(void) {
	[NSApp activateIgnoringOtherApps:YES];
}

static void autosaveFrame(uintptr_t win, const char *name) {
	NSWindow *w = (__bridge NSWindow *)(void *)win;
	NSString *n = [NSString stringWithUTF8String:name];
	[w setFrameUsingName:n];
	[w setFrameAutosaveName:n];
}
*/
import "C"

import (
	"unsafe"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver"
)

// hideFromDock, uygulamayı Dock'tan ve Cmd+Tab'dan gizler. Info.plist'teki
// LSUIElement tek başına yetmiyor: GLFW açılışta etkinleştirme politikasını
// "Regular"a çeviriyor, bu yüzden açılıştan sonra tekrar "Accessory" yapılır.
func hideFromDock() { C.hideFromDock() }

// activateApp, Dock'ta olmayan uygulamanın penceresini öne getirir.
func activateApp() { C.activateApp() }

// rememberWindowFrame, pencerenin konumunu ve boyutunu macOS'un kendi
// mekanizmasıyla (NSWindow frame autosave) saklar; kayıtlı bir konum varsa
// pencere oraya taşınır. Pencere gösterildikten sonra çağrılmalıdır.
func rememberWindowFrame(w fyne.Window, name string) {
	nw, ok := w.(driver.NativeWindow)
	if !ok {
		return
	}
	nw.RunNative(func(ctx any) {
		mac, ok := ctx.(driver.MacWindowContext)
		if !ok || mac.NSWindow == 0 {
			return
		}
		cname := C.CString(name)
		defer C.free(unsafe.Pointer(cname))
		C.autosaveFrame(C.uintptr_t(mac.NSWindow), cname)
	})
}
