package main

import "github.com/wailsapp/wails/v3/pkg/application"

// minWindowWidth is the narrowest the main window gets, in points.
const minWindowWidth = 520

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
