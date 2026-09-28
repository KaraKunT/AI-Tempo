//go:build !darwin

package gui

import "fyne.io/fyne/v2"

func hideFromDock() {}
func activateApp()  {}

func rememberWindowFrame(fyne.Window, string) {}
