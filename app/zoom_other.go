//go:build !darwin

package main

import "github.com/wailsapp/wails/v3/pkg/application"

// setContentZoom sets the page zoom (the app's text size).
func setContentZoom(window *application.WebviewWindow, zoom float64) {
	window.SetZoom(zoom)
}
