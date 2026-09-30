//go:build !darwin && !windows

package gui

import "fyne.io/fyne/v2"

func hideFromDock() {}
func showInDock()   {}
func activateApp()  {}

func installDockReopen(func()) {}

func rememberWindowFrame(fyne.Window, string) {}

const (
	loginNotRegistered   = 0
	loginEnabled         = 1
	loginRequiresApprove = 2
	loginNotFound        = 3
	loginUnsupported     = -1
)

func loginItemState() int        { return loginUnsupported }
func setLaunchAtLogin(bool) bool { return false }
