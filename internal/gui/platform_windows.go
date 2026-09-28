//go:build windows

package gui

import (
	"fmt"
	"os"
	"strings"
	"time"
	"unsafe"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// Windows'ta Dock yoktur; uygulama sistem tepsisinde (saatin yanı) çalışır.
func hideFromDock() {}
func showInDock()   {}
func activateApp()  {}

// Oturum açılış öğesi durumları (macOS sürümüyle aynı değerler).
const (
	loginNotRegistered   = 0
	loginEnabled         = 1
	loginRequiresApprove = 2
	loginNotFound        = 3
	loginUnsupported     = -1
)

const (
	runKeyPath   = `Software\Microsoft\Windows\CurrentVersion\Run`
	runValueName = "AI Tempo"
)

// loginItemState, HKCU\...\Run altında bu exe'yi gösteren bir kayıt olup olmadığına bakar.
func loginItemState() int {
	exe, err := os.Executable()
	if err != nil {
		return loginNotFound
	}
	k, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.QUERY_VALUE)
	if err != nil {
		return loginNotRegistered
	}
	defer k.Close()
	v, _, err := k.GetStringValue(runValueName)
	if err != nil || !strings.EqualFold(strings.Trim(v, `"`), exe) {
		return loginNotRegistered // kayıt yok ya da exe taşınmış
	}
	return loginEnabled
}

// setLaunchAtLogin, uygulamayı oturum açılışında başlatılacak şekilde kaydeder veya siler.
func setLaunchAtLogin(on bool) bool {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, runKeyPath, registry.SET_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	if !on {
		err = k.DeleteValue(runValueName)
		return err == nil || err == registry.ErrNotExist
	}
	exe, err := os.Executable()
	if err != nil {
		return false
	}
	return k.SetStringValue(runValueName, `"`+exe+`"`) == nil
}

var (
	user32            = windows.NewLazySystemDLL("user32.dll")
	procGetWindowRect = user32.NewProc("GetWindowRect")
	procSetWindowPos  = user32.NewProc("SetWindowPos")
	procIsWindow      = user32.NewProc("IsWindow")
	procIsIconic      = user32.NewProc("IsIconic")
)

type rect struct{ Left, Top, Right, Bottom int32 }

const (
	swpNoZOrder   = 0x0004
	swpNoActivate = 0x0010
)

// rememberWindowFrame, pencerenin konum ve boyutunu uygulama tercihlerinde
// saklar; kayıtlı bir konum varsa pencere oraya taşınır. Pencere gösterildikten
// sonra çağrılmalıdır. (Fyne'de konum API'si olmadığı için Win32 kullanılır.)
func rememberWindowFrame(w fyne.Window, name string) {
	nw, ok := w.(driver.NativeWindow)
	if !ok {
		return
	}
	prefs := fyne.CurrentApp().Preferences()
	key := "frame." + name
	nw.RunNative(func(ctx any) {
		wc, ok := ctx.(driver.WindowsWindowContext)
		if !ok || wc.HWND == 0 {
			return
		}
		hwnd := wc.HWND
		var x, y, cw, ch int32
		if _, err := fmt.Sscanf(prefs.String(key), "%d %d %d %d", &x, &y, &cw, &ch); err == nil && cw > 100 && ch > 100 {
			procSetWindowPos.Call(hwnd, 0, uintptr(x), uintptr(y), uintptr(cw), uintptr(ch), swpNoZOrder|swpNoActivate)
		}
		// Konum değiştikçe kaydet; pencere kapanınca döngü biter.
		go func() {
			last := ""
			for range time.Tick(2 * time.Second) {
				if ok, _, _ := procIsWindow.Call(hwnd); ok == 0 {
					return
				}
				if min, _, _ := procIsIconic.Call(hwnd); min != 0 {
					continue // simge durumundayken konum anlamsız
				}
				var r rect
				if ok, _, _ := procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&r))); ok == 0 {
					continue
				}
				cur := fmt.Sprintf("%d %d %d %d", r.Left, r.Top, r.Right-r.Left, r.Bottom-r.Top)
				if cur != last {
					last = cur
					prefs.SetString(key, cur)
				}
			}
		}()
	})
}
