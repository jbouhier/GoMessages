//go:build windows || linux

package main

import (
	"reflect"
	"unsafe"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// webviewHandle reads the web view handle out of Wails' platform window by
// field names (path), since Wails v3 has no public accessor for it. The
// names match the Wails version pinned in go.mod; nil if they ever change,
// so sign-in fails with a message instead of crashing.
func webviewHandle(w *application.WebviewWindow, path ...string) unsafe.Pointer {
	v := reflect.ValueOf(w)
	for _, name := range append([]string{"impl"}, path...) {
		for v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface {
			if v.IsNil() {
				return nil
			}
			v = v.Elem()
		}
		if v.Kind() != reflect.Struct {
			return nil
		}
		if v = v.FieldByName(name); !v.IsValid() {
			return nil
		}
	}
	if v.Kind() != reflect.Pointer && v.Kind() != reflect.UnsafePointer {
		return nil
	}
	return v.UnsafePointer()
}
