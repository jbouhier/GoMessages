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
	"github.com/wailsapp/wails/v3/pkg/events"
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
		// Windows and Linux: no system title bar, the UI draws one in the
		// theme's colors. Windows 11 keeps the shadow and round corners.
		Frameless: customTitleBar(),
		// Files dragged from the desktop onto a conversation attach to it.
		EnableFileDrop: true,
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
	window.OnWindowEvent(events.Common.WindowFilesDropped, func(e *application.WindowEvent) {
		messages.filesDropped(e.Context().DroppedFiles())
	})
	menu, shortcuts := buildMenu(app)
	if customTitleBar() {
		// No menu bar to carry the shortcuts: the window catches them.
		for _, s := range windowKeys(shortcuts) {
			window.RegisterKeyBinding(s.accel, func(application.Window) { s.run() })
		}
		window.RegisterKeyBinding("Ctrl+Q", func(application.Window) { app.Quit() })
		window.RegisterKeyBinding("F11", func(w application.Window) { w.ToggleFullscreen() })
	} else {
		app.Menu.Set(menu)
	}

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

// blurSupported: macOS and Windows blur what's behind a window; Linux
// compositors vary too much to rely on it.
func blurSupported() bool { return runtime.GOOS == "darwin" || runtime.GOOS == "windows" }

// mediaMiddleware serves downloaded photos, files, and contact photos from
// our media folder at /media/<file>, and the files the user attached at
// /attachment?path=<path> for their previews; nothing else.
func mediaMiddleware(m *Messages) application.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/attachment" {
				if p := r.URL.Query().Get("path"); m.previewable(p) {
					http.ServeFile(w, r, p)
				} else {
					http.NotFound(w, r)
				}
				return
			}
			if name, ok := strings.CutPrefix(r.URL.Path, "/media/"); ok && name != "" && !strings.Contains(name, "/") {
				http.ServeFile(w, r, filepath.Join(m.dir, "media", filepath.Base(name)))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// shortcut is a menu item's key and what it does, for windows without a
// menu bar.
type shortcut struct {
	accel string
	run   func()
}

// windowKeys are the shortcuts a window without a menu bar catches itself.
// Alt+arrows stay with the UI, which handles them on every platform: caught
// here too, they'd move twice.
func windowKeys(all []shortcut) []shortcut {
	var keys []shortcut
	for _, s := range all {
		if !strings.HasPrefix(s.accel, "Alt+") {
			keys = append(keys, s)
		}
	}
	return keys
}

// buildMenu is the menu bar, and the shortcuts in it. Each item carries its
// shortcut (which is also how people discover them) and hands the action to
// the UI.
func buildMenu(app *application.App) (*application.Menu, []shortcut) {
	var shortcuts []shortcut
	// item adds a menu item that hands name to the UI, and keeps its shortcut.
	item := func(m *application.Menu, label, accel, name string) {
		run := func() { app.Event.Emit("action", name) }
		i := m.Add(label).OnClick(func(*application.Context) { run() })
		if accel != "" {
			i.SetAccelerator(accel)
			shortcuts = append(shortcuts, shortcut{accel, run})
		}
	}
	menu := application.NewMenu()

	appMenu := menu.AddSubmenu("GoMessages")
	appMenu.AddRole(application.About)
	appMenu.AddSeparator()
	item(appMenu, "Settings…", "CmdOrCtrl+,", "settings")
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
	item(view, "Show/Hide Conversation List", "CmdOrCtrl+B", "toggle-list")
	item(view, "Photos-Only List", "CmdOrCtrl+Shift+\\", "photos-only")
	item(view, "Show/Hide Conversation Info", "CmdOrCtrl+I", "info")
	view.AddSeparator()
	item(view, "Light / Dark", "CmdOrCtrl+Shift+L", "toggle-dark")
	view.AddSeparator()
	item(view, "Actual Size", "CmdOrCtrl+0", "zoom-reset")
	item(view, "Zoom In", "CmdOrCtrl+plus", "zoom-in")
	item(view, "Zoom Out", "CmdOrCtrl+-", "zoom-out")
	view.AddSeparator()
	item(view, "Refresh Conversations", "CmdOrCtrl+R", "refresh")
	view.AddSeparator()
	view.AddRole(application.ToggleFullscreen)

	goMenu := menu.AddSubmenu("Go")
	item(goMenu, "New Chat", "CmdOrCtrl+N", "new-chat")
	item(goMenu, "Switch Conversation…", "CmdOrCtrl+K", "switch")
	item(goMenu, "Search Conversations", "CmdOrCtrl+F", "search")
	goMenu.AddSeparator()
	item(goMenu, "Next Conversation", "Alt+Down", "next")
	item(goMenu, "Previous Conversation", "Alt+Up", "previous")
	item(goMenu, "Next Unread", "Alt+Shift+Down", "next-unread")
	item(goMenu, "Previous Unread", "Alt+Shift+Up", "previous-unread")
	goMenu.AddSeparator()
	jump := goMenu.AddSubmenu("Go to Conversation")
	for n := 1; n <= 9; n++ {
		item(jump, "Conversation "+strconv.Itoa(n), "CmdOrCtrl+"+strconv.Itoa(n), "jump-"+strconv.Itoa(n-1))
	}
	item(goMenu, "Archived Conversations", "", "archived")
	goMenu.AddSeparator()
	item(goMenu, "Attach File…", "CmdOrCtrl+O", "attach")

	menu.AddRole(application.WindowMenu)

	help := menu.AddSubmenu("Help")
	item(help, "Keyboard Shortcuts", "CmdOrCtrl+/", "shortcuts")
	return menu, shortcuts
}
