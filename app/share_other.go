//go:build !darwin

package main

import "github.com/wailsapp/wails/v3/pkg/application"

func shareSupported() bool { return false }

func showShare(window *application.WebviewWindow, path string, x, y, w, h float64) {}
