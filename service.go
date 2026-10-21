package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/wailsapp/wails/v3/pkg/application"
	wailevents "github.com/wailsapp/wails/v3/pkg/events"
	"github.com/wailsapp/wails/v3/pkg/services/notifications"
)

const googleLoginURL = "https://accounts.google.com/AccountChooser?continue=https://messages.google.com/web/config"

// Messages is the service the UI calls. Protocol commands go through Call,
// with the same names and shapes as gogo-messages' sidecar protocol.
type Messages struct {
	app      *application.App
	window   *application.WebviewWindow
	notifier *notifications.NotificationService
	notifyOK bool
	backend  backend
	links    *links
	dir      string
	demo     bool

	mu         sync.Mutex
	signIn     *application.WebviewWindow
	signingIn  bool
	settings   Settings
	saveTimer  *time.Timer
	settingsMu sync.Mutex
	// httpEvents carries events in --serve mode, where there is no Wails app.
	httpEvents *eventHub
}

func newMessages(demo bool) (*Messages, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("find config directory: %w", err)
	}
	dir := filepath.Join(base, "GoMessages")
	// Builds before 0.4.0 were "GoMessages Next" and kept their data in
	// GoMessagesNext; move it over once so the pairing carries over.
	if _, err := os.Stat(dir); errors.Is(err, os.ErrNotExist) {
		_ = os.Rename(filepath.Join(base, "GoMessagesNext"), dir)
	}
	if demo {
		dir = filepath.Join(dir, "demo")
	}
	if err := os.MkdirAll(filepath.Join(dir, "media"), 0o700); err != nil {
		return nil, err
	}
	// The first Wails build saved its pairing as auth.json; the sidecar code
	// calls it session.json. Same libgm data, so an existing pairing carries over.
	old, current := filepath.Join(dir, "auth.json"), filepath.Join(dir, "session.json")
	if _, err := os.Stat(current); errors.Is(err, os.ErrNotExist) {
		_ = os.Rename(old, current)
	}
	m := &Messages{dir: dir, demo: demo, notifier: notifications.New()}
	m.settings = loadSettings(m.settingsPath())
	o := &out{emit: m.emit}
	if demo {
		m.backend = newDemo(o, dir)
	} else {
		level := zerolog.WarnLevel
		if os.Getenv("GOGO_DEBUG") != "" {
			level = zerolog.DebugLevel
		}
		log := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.TimeOnly}).Level(level).With().Timestamp().Logger()
		m.backend = newDaemon(o, dir, log)
		m.links = newLinks(dir)
	}
	return m, nil
}

func (m *Messages) emit(name string, data any) {
	payload := map[string]any{"event": name, "data": data}
	switch {
	case m.httpEvents != nil:
		m.httpEvents.publish("gm", payload)
	case m.app != nil:
		m.app.Event.Emit("gm", payload)
	}
}

// ServiceStartup starts the backend once the app (and its event bus) runs.
func (m *Messages) ServiceStartup(ctx context.Context, opts application.ServiceOptions) error {
	// Started here rather than as its own service: outside an .app bundle
	// (just run) it fails, and that should cost notifications, not the app.
	if err := m.notifier.ServiceStartup(ctx, opts); err != nil {
		log.Printf("notifications off: %v", err)
	} else {
		m.notifyOK = true
	}
	go m.backend.start(context.Background())
	return nil
}

func (m *Messages) ServiceShutdown() error {
	if d, ok := m.backend.(*daemon); ok {
		d.shutdown()
	}
	if m.notifyOK {
		_ = m.notifier.ServiceShutdown()
	}
	m.flushSettings()
	return nil
}

// Call runs one protocol command (conversations, messages, send, download,
// avatars, contacts, sims, new_conversation, mark_read, react, typing,
// logout, cancel_pair, state…), plus link_preview.
func (m *Messages) Call(cmd string, args map[string]any) (any, error) {
	raw, err := json.Marshal(args)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	// Link previews are web pages, not the phone: Go fetches them itself.
	// The demo answers from its own samples, so it never goes online.
	if cmd == "link_preview" && m.links != nil {
		var a struct {
			URL string `json:"url"`
		}
		_ = json.Unmarshal(raw, &a)
		return m.links.preview(ctx, a.URL)
	}
	return m.backend.handle(ctx, cmd, raw)
}

// IsDemo tells the UI it runs on sample data.
func (m *Messages) IsDemo() bool { return m.demo }

// SignIn opens Google's sign-in in its own window; once it reaches Messages,
// the cookies start emoji pairing (pair_emoji / paired / pair_error events).
func (m *Messages) SignIn() error {
	if m.demo {
		_, err := m.Call("pair_google", map[string]any{"cookies": map[string]string{}})
		return err
	}
	if m.app == nil {
		return errors.New("Google sign-in needs the desktop app")
	}
	m.mu.Lock()
	if m.signIn != nil {
		m.signIn.Focus()
		m.mu.Unlock()
		return nil
	}
	m.signingIn = false
	window := m.app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:     "Sign in to Google",
		Width:     520,
		Height:    700,
		MinWidth:  420,
		MinHeight: 520,
		URL:       "about:blank",
		Hidden:    true,
		Mac:       application.MacWindow{EnableFraudulentWebsiteWarnings: true},
	})
	m.signIn = window
	m.mu.Unlock()
	// Each platform reports page loads with its own event; the others never fire.
	for _, loaded := range []wailevents.WindowEventType{
		wailevents.Mac.WebViewDidCommitNavigation,
		wailevents.Windows.WebViewNavigationCompleted,
		wailevents.Linux.WindowLoadCommitted,
	} {
		window.OnWindowEvent(loaded, func(_ *application.WindowEvent) {
			go func() {
				if isMessagesConfigURL(readWebviewURL(window)) {
					m.finishSignIn(window)
				}
			}()
		})
	}
	window.OnWindowEvent(wailevents.Common.WindowClosing, func(_ *application.WindowEvent) {
		m.mu.Lock()
		closedEarly := m.signIn == window && !m.signingIn
		if m.signIn == window {
			m.signIn = nil
		}
		m.mu.Unlock()
		if closedEarly {
			m.emit("signin_cancelled", nil)
		}
	})
	window.SetURL(googleLoginURL)
	window.Show()
	return nil
}

func isMessagesConfigURL(raw string) bool {
	parsed, err := url.Parse(raw)
	return err == nil && parsed.Scheme == "https" && parsed.Hostname() == "messages.google.com" && parsed.Path == "/web/config"
}

func (m *Messages) finishSignIn(window *application.WebviewWindow) {
	m.mu.Lock()
	if m.signIn != window || m.signingIn {
		m.mu.Unlock()
		return
	}
	m.signingIn = true
	m.mu.Unlock()
	window.Hide()
	var cookies map[string]string
	var err error
	for attempt := 0; attempt < 20; attempt++ {
		if cookies, err = readWebviewGoogleCookies(window); err == nil {
			break
		}
		time.Sleep(250 * time.Millisecond)
	}
	window.Close()
	if err != nil {
		m.emit("pair_error", map[string]string{"message": "Google sign-in could not be completed: " + err.Error()})
		return
	}
	if _, err := m.Call("pair_google", map[string]any{"cookies": cookies}); err != nil {
		// pair_error carries the readable message.
		return
	}
}

// PickFile asks for an attachment; "" when cancelled.
func (m *Messages) PickFile() (string, error) {
	return m.app.Dialog.OpenFile().SetTitle("Attach a file").PromptForSingleSelection()
}

// SaveAs copies a downloaded file where the user chooses.
func (m *Messages) SaveAs(path, name string) error {
	dest, err := m.app.Dialog.SaveFile().SetFilename(name).PromptForSingleSelection()
	if err != nil || dest == "" {
		return err
	}
	src, err := os.Open(m.mediaFile(path))
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer dst.Close()
	_, err = io.Copy(dst, src)
	return err
}

// OpenPath opens a downloaded file in its default app.
func (m *Messages) OpenPath(path string) error {
	p := m.mediaFile(path)
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", p).Start()
	case "windows":
		return exec.Command("cmd", "/C", "start", "", p).Start()
	default:
		return exec.Command("xdg-open", p).Start()
	}
}

// mediaFile keeps file access inside our media folder.
func (m *Messages) mediaFile(path string) string {
	return filepath.Join(m.dir, "media", filepath.Base(path))
}

// Notify shows a desktop notification for a new message.
func (m *Messages) Notify(title, body string, sound bool) error {
	if !m.notifyOK {
		return nil
	}
	opts := notifications.NotificationOptions{ID: fmt.Sprint(time.Now().UnixNano()), Title: title, Body: body}
	if !sound {
		opts.Sound = &notifications.NotificationSound{Silent: true}
	}
	return m.notifier.SendNotification(opts)
}

// RequestNotifications asks for permission once, at first use.
func (m *Messages) RequestNotifications() bool {
	if !m.notifyOK {
		return false
	}
	ok, _ := m.notifier.RequestNotificationAuthorization()
	return ok
}

// OpenDataFolder shows where the session, settings, and media live.
func (m *Messages) OpenDataFolder() error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", m.dir).Start()
	case "windows":
		return exec.Command("explorer", m.dir).Start()
	default:
		return exec.Command("xdg-open", m.dir).Start()
	}
}

// BlurSupported says whether the window can blur what's behind it.
func (m *Messages) BlurSupported() bool { return m.window != nil && blurSupported() }

// SetAppearance tints the window light or dark ("" follows the system).
func (m *Messages) SetAppearance(mode string) {
	if m.window != nil {
		setAppearance(m.window, mode)
	}
}

// SetBlur shows the blurred desktop behind the window, for transparency.
func (m *Messages) SetBlur(on bool) {
	if m.window != nil {
		setWindowBlur(m.window, on)
	}
}

// SetZoom applies the page zoom (text size) and remembers it.
func (m *Messages) SetZoom(zoom float64) {
	if m.window != nil {
		setContentZoom(m.window, zoom)
	}
	m.settingsMu.Lock()
	m.settings.Zoom = zoom
	m.settingsMu.Unlock()
	m.scheduleSave()
}
