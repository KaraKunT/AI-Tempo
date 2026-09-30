//go:build darwin

package gui

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa -framework ServiceManagement
#import <Cocoa/Cocoa.h>
#import <ServiceManagement/ServiceManagement.h>
#import <objc/runtime.h>
#include <stdlib.h>

extern void goDockReopen(void);

// dockReopen, Dock simgesine tıklanınca çağrılır; pencere gizliyse Go tarafı gösterir.
static BOOL dockReopen(id self, SEL _cmd, NSApplication *app, BOOL hasVisible) {
	goDockReopen();
	return YES;
}

// installReopenHandler, GLFW'nin uygulama delegesine eksik olan
// applicationShouldHandleReopen:hasVisibleWindows: yöntemini ekler.
static void installReopenHandler(void) {
	id d = [NSApp delegate];
	if (d == nil) return;
	SEL sel = @selector(applicationShouldHandleReopen:hasVisibleWindows:);
	if (!class_addMethod([d class], sel, (IMP)dockReopen, "c@:@c")) {
		class_replaceMethod([d class], sel, (IMP)dockReopen, "c@:@c");
	}
}

// loginItemStatus: 0 kayıtlı değil, 1 etkin, 2 kullanıcı onayı gerekiyor,
// 3 bulunamadı (.app dışında çalışıyor), -1 desteklenmiyor (macOS < 13).
static int loginItemStatus(void) {
	if (@available(macOS 13.0, *)) {
		return (int)[SMAppService mainAppService].status;
	}
	return -1;
}

// setLoginItem, uygulamayı oturum açılış öğesi olarak kaydeder/kaldırır; 0 başarı.
static int setLoginItem(int on) {
	if (@available(macOS 13.0, *)) {
		NSError *err = nil;
		BOOL ok = on ? [[SMAppService mainAppService] registerAndReturnError:&err]
		             : [[SMAppService mainAppService] unregisterAndReturnError:&err];
		return ok ? 0 : (int)(err ? err.code : 1);
	}
	return -1;
}

static void hideFromDock(void) {
	[NSApp setActivationPolicy:NSApplicationActivationPolicyAccessory];
}

static void showInDock(void) {
	[NSApp setActivationPolicy:NSApplicationActivationPolicyRegular];
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

// showInDock, uygulamayı Dock'ta ve Cmd+Tab'da gösterir.
func showInDock() { C.showInDock() }

// onDockReopen, Dock simgesine tıklanınca çalışacak fonksiyondur.
var onDockReopen func()

//export goDockReopen
func goDockReopen() {
	if onDockReopen != nil {
		onDockReopen()
	}
}

// installDockReopen, pencere x ile gizlendikten sonra Dock simgesine
// tıklanınca pencerenin yeniden açılmasını sağlar. Açılıştan sonra çağrılmalıdır.
func installDockReopen(show func()) {
	onDockReopen = show
	C.installReopenHandler()
}

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

// Oturum açılış öğesi durumları (SMAppServiceStatus).
const (
	loginNotRegistered   = 0
	loginEnabled         = 1
	loginRequiresApprove = 2
	loginNotFound        = 3
	loginUnsupported     = -1
)

// loginItemState, uygulamanın oturum açılışında başlatılma durumunu döndürür.
func loginItemState() int { return int(C.loginItemStatus()) }

// setLaunchAtLogin, uygulamayı oturum açılışında başlatılacak şekilde kaydeder
// veya kaydını siler (macOS 13+, SMAppService). Hata olursa false döner.
func setLaunchAtLogin(on bool) bool {
	v := C.int(0)
	if on {
		v = 1
	}
	return C.setLoginItem(v) == 0
}
