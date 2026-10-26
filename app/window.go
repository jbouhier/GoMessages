package main

import (
	"os"
	"runtime"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// minWindowWidth is the narrowest the main window gets, in points.
const minWindowWidth = 520

// customTitleBar: on Windows and Linux the window has no system title bar.
// The UI draws its own, in the theme's colors with the window buttons on
// the right, the way macOS shows our headers under the traffic lights.
func customTitleBar() bool { return runtime.GOOS != "darwin" }

// onScreen: enough of the window shows on some screen to grab it. A window
// saved on a monitor that's gone isn't.
func onScreen(app *application.App, b application.Rect) bool {
	screens := app.Screen.GetAll()
	if len(screens) == 0 {
		return true
	}
	for _, s := range screens {
		if in := s.WorkArea.Intersect(b); in.Width >= 100 && in.Height >= 40 {
			return true
		}
	}
	return false
}

// droppableFiles keeps the files among what was dropped on the window, in
// order and without repeats: a folder, or a file gone since, can't be sent.
func droppableFiles(paths []string) []string {
	var files []string
	seen := map[string]bool{}
	for _, p := range paths {
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		if info, err := os.Stat(p); err == nil && info.Mode().IsRegular() {
			files = append(files, p)
		}
	}
	return files
}
