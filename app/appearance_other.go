//go:build !darwin

package main

import "github.com/wailsapp/wails/v3/pkg/application"

// setAppearance is macOS-only; elsewhere the theme alone sets the look.
func setAppearance(*application.WebviewWindow, string) {}

// setWindowBlur is macOS-only; Windows gets Acrylic from the window options.
func setWindowBlur(*application.WebviewWindow, bool) {}
