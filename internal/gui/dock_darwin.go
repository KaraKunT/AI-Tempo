//go:build darwin

package gui

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa
#import <Cocoa/Cocoa.h>

static void hideFromDock(void) {
	[NSApp setActivationPolicy:NSApplicationActivationPolicyAccessory];
}

static void activateApp(void) {
	[NSApp activateIgnoringOtherApps:YES];
}
*/
import "C"

// hideFromDock, uygulamayı Dock'tan ve Cmd+Tab'dan gizler. Info.plist'teki
// LSUIElement tek başına yetmiyor: GLFW açılışta etkinleştirme politikasını
// "Regular"a çeviriyor, bu yüzden açılıştan sonra tekrar "Accessory" yapılır.
func hideFromDock() { C.hideFromDock() }

// activateApp, Dock'ta olmayan uygulamanın penceresini öne getirir.
func activateApp() { C.activateApp() }
