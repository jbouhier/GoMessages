package main

import (
	"embed"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var appIcon []byte

// dockIcon is the icon for the Dock and windows when not running from an
// .app bundle (just run, wails3 dev, Linux, Windows). A bundle has its own
// .icns, sized the way macOS expects, and keeps it.
func dockIcon() []byte {
	if exe, err := os.Executable(); err == nil && strings.Contains(exe, ".app/Contents/MacOS/") {
		return nil
	}
	return appIcon
}

func main() {
	messages, err := newMessages(slices.Contains(os.Args[1:], "--demo"))
	if err != nil {
		log.Fatal(err)
	}
	// --serve 127.0.0.1:9250: the UI in a browser instead of a window.
	if i := slices.Index(os.Args, "--serve"); i > 0 && i+1 < len(os.Args) {
		log.Fatal(messages.serve(os.Args[i+1]))
	}
	app := application.New(application.Options{
		Name:        "GoMessages",
		Description: "Google Messages for the desktop",
		Icon:        dockIcon(),
		Services: []application.Service{
			application.NewService(messages),
		},
		Assets: application.AssetOptions{
			Handler:    application.AssetFileServerFS(assets),
			Middleware: mediaMiddleware(messages),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})
	messages.app = app

	options := application.WebviewWindowOptions{
		Title:     "Messages",
		Width:     1120,
		Height:    740,
		MinWidth:  minWindowWidth,
		MinHeight: 400,
		URL:       "/",
		Mac: application.MacWindow{
			// Our own header sits under the traffic lights, like gogo's.
			TitleBar:                application.MacTitleBarHiddenInset,
			InvisibleTitleBarHeight: 52,
			// A clear window: with transparency the (blurred) desktop shows
			// through; otherwise the page paints over it. See setWindowBlur.
			Backdrop: application.MacBackdropTransparent,
		},
		Windows: application.WindowsWindow{BackdropType: application.Acrylic},
	}
	if blurSupported() {
		options.BackgroundType = application.BackgroundTypeTranslucent
		options.BackgroundColour = application.NewRGBA(0, 0, 0, 0)
	} else {
		options.Mac.Backdrop = application.MacBackdropNormal
		options.BackgroundColour = application.NewRGB(255, 255, 255)
	}
	messages.restoreWindow(&options)
	window := app.Window.NewWithOptions(options)
	messages.trackWindow(window)
	app.Menu.Set(buildMenu(app))

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

// blurSupported: macOS and Windows blur what's behind a window; Linux
// compositors vary too much to rely on it.
func blurSupported() bool { return runtime.GOOS == "darwin" || runtime.GOOS == "windows" }

// mediaMiddleware serves downloaded photos, files, and contact photos from
// our media folder at /media/<file>; nothing outside it.
func mediaMiddleware(m *Messages) application.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if name, ok := strings.CutPrefix(r.URL.Path, "/media/"); ok && name != "" && !strings.Contains(name, "/") {
				http.ServeFile(w, r, filepath.Join(m.dir, "media", filepath.Base(name)))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// buildMenu is the menu bar. Each item carries its shortcut (which is also
// how people discover them) and hands the action to the UI.
func buildMenu(app *application.App) *application.Menu {
	action := func(name string) func(*application.Context) {
		return func(*application.Context) { app.Event.Emit("action", name) }
	}
	menu := application.NewMenu()

	appMenu := menu.AddSubmenu("GoMessages")
	appMenu.AddRole(application.About)
	appMenu.AddSeparator()
	appMenu.Add("Settings…").SetAccelerator("CmdOrCtrl+,").OnClick(action("settings"))
	appMenu.AddSeparator()
	appMenu.AddRole(application.ServicesMenu)
	appMenu.AddSeparator()
	appMenu.AddRole(application.Hide)
	appMenu.AddRole(application.HideOthers)
	appMenu.AddRole(application.ShowAll)
	appMenu.AddSeparator()
	appMenu.AddRole(application.Quit)

	// Copy, paste, undo… for the text fields: web views need the Edit menu.
	menu.AddRole(application.EditMenu)

	view := menu.AddSubmenu("View")
	view.Add("Show/Hide Conversation List").SetAccelerator("CmdOrCtrl+B").OnClick(action("toggle-list"))
	view.Add("Photos-Only List").SetAccelerator("CmdOrCtrl+Shift+\\").OnClick(action("photos-only"))
	view.Add("Show/Hide Conversation Info").SetAccelerator("CmdOrCtrl+I").OnClick(action("info"))
	view.AddSeparator()
	view.Add("Light / Dark").SetAccelerator("CmdOrCtrl+Shift+L").OnClick(action("toggle-dark"))
	view.AddSeparator()
	view.Add("Actual Size").SetAccelerator("CmdOrCtrl+0").OnClick(action("zoom-reset"))
	view.Add("Zoom In").SetAccelerator("CmdOrCtrl+plus").OnClick(action("zoom-in"))
	view.Add("Zoom Out").SetAccelerator("CmdOrCtrl+-").OnClick(action("zoom-out"))
	view.AddSeparator()
	view.Add("Refresh Conversations").SetAccelerator("CmdOrCtrl+R").OnClick(action("refresh"))
	view.AddSeparator()
	view.AddRole(application.ToggleFullscreen)

	goMenu := menu.AddSubmenu("Go")
	goMenu.Add("New Chat").SetAccelerator("CmdOrCtrl+N").OnClick(action("new-chat"))
	goMenu.Add("Switch Conversation…").SetAccelerator("CmdOrCtrl+K").OnClick(action("switch"))
	goMenu.Add("Search Conversations").SetAccelerator("CmdOrCtrl+F").OnClick(action("search"))
	goMenu.AddSeparator()
	goMenu.Add("Next Conversation").SetAccelerator("Alt+Down").OnClick(action("next"))
	goMenu.Add("Previous Conversation").SetAccelerator("Alt+Up").OnClick(action("previous"))
	goMenu.Add("Next Unread").SetAccelerator("Alt+Shift+Down").OnClick(action("next-unread"))
	goMenu.Add("Previous Unread").SetAccelerator("Alt+Shift+Up").OnClick(action("previous-unread"))
	goMenu.AddSeparator()
	jump := goMenu.AddSubmenu("Go to Conversation")
	for n := 1; n <= 9; n++ {
		jump.Add("Conversation " + strconv.Itoa(n)).
			SetAccelerator("CmdOrCtrl+" + strconv.Itoa(n)).
			OnClick(action("jump-" + strconv.Itoa(n-1)))
	}
	goMenu.Add("Archived Conversations").OnClick(action("archived"))
	goMenu.AddSeparator()
	goMenu.Add("Attach File…").SetAccelerator("CmdOrCtrl+O").OnClick(action("attach"))

	menu.AddRole(application.WindowMenu)

	help := menu.AddSubmenu("Help")
	help.Add("Keyboard Shortcuts").SetAccelerator("CmdOrCtrl+/").OnClick(action("shortcuts"))
	return menu
}
