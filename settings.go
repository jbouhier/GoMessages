package main

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	wailevents "github.com/wailsapp/wails/v3/pkg/events"
)

// Settings mirrors gogo-messages' settings: preferences the UI edits, plus
// the window's place, which Go tracks itself.
type Settings struct {
	Mode                string    `json:"mode"`         // system, light, or dark: which half of the theme is in use
	Theme               string    `json:"theme"`        // a theme's id: default, graphite, sky, mauve, aurora, peach, amber, sage
	Background          string    `json:"background"`   // gradient (the theme's art) or solid
	Transparency        float64   `json:"transparency"` // 0–100: how much of what's behind the window shows through
	Zoom                float64   `json:"zoom"`
	Window              *Geometry `json:"window,omitempty"`
	Maximized           bool      `json:"maximized"`
	SidebarVisible      bool      `json:"sidebar_visible"`
	SidebarWidth        float64   `json:"sidebar_width"`
	SidebarWideWidth    float64   `json:"sidebar_wide_width"`
	Notifications       bool      `json:"notifications"`
	NotificationPreview bool      `json:"notification_preview"`
	NotificationSound   bool      `json:"notification_sound"`
	EnterSends          bool      `json:"enter_sends"`
	SendTyping          bool      `json:"send_typing"`
	Muted               []string  `json:"muted"` // conversations that don't notify; the phone doesn't report mute state
}

// Geometry is the window's position and size in points; the position also
// says which monitor it was on.
type Geometry struct {
	X, Y, W, H int
}

func defaultSettings() Settings {
	return Settings{
		Mode: "system", Theme: "default", Background: "gradient", Zoom: 1, SidebarVisible: true, SidebarWidth: 340, SidebarWideWidth: 340,
		Notifications: true, NotificationPreview: true, NotificationSound: true, EnterSends: true, SendTyping: true,
	}
}

// parseSettings fills what a file leaves out with defaults, so older files
// keep working, and keeps the zoom usable.
func parseSettings(data []byte) Settings {
	s := defaultSettings()
	if err := json.Unmarshal(data, &s); err != nil {
		s = defaultSettings()
	}
	if math.IsNaN(s.Zoom) || s.Zoom <= 0 {
		s.Zoom = 1
	}
	s.Zoom = math.Min(math.Max(s.Zoom, 0.5), 3)
	migrateTheme(data, &s)
	s.Transparency = math.Min(math.Max(s.Transparency, 0), 100)
	if math.IsNaN(s.Transparency) {
		s.Transparency = 0
	}
	return s
}

func loadSettings(path string) Settings {
	data, err := os.ReadFile(path)
	if err != nil {
		return defaultSettings()
	}
	return parseSettings(data)
}

func (m *Messages) settingsPath() string { return filepath.Join(m.dir, "settings.json") }

// GetSettings returns the saved preferences.
func (m *Messages) GetSettings() Settings {
	m.settingsMu.Lock()
	defer m.settingsMu.Unlock()
	return m.settings
}

// UpdateSettings stores the UI's preferences; the window's place stays Go's.
func (m *Messages) UpdateSettings(s Settings) {
	m.settingsMu.Lock()
	s.Window, s.Maximized, s.Zoom = m.settings.Window, m.settings.Maximized, m.settings.Zoom
	m.settings = s
	m.settingsMu.Unlock()
	m.scheduleSave()
}

// scheduleSave writes shortly after the last change, so a drag writes once.
func (m *Messages) scheduleSave() {
	m.settingsMu.Lock()
	defer m.settingsMu.Unlock()
	if m.saveTimer != nil {
		m.saveTimer.Stop()
	}
	m.saveTimer = time.AfterFunc(600*time.Millisecond, m.flushSettings)
}

func (m *Messages) flushSettings() {
	if m.demo {
		return
	}
	m.settingsMu.Lock()
	data, err := json.MarshalIndent(m.settings, "", "  ")
	m.settingsMu.Unlock()
	if err != nil {
		return
	}
	tmp := m.settingsPath() + ".tmp"
	if os.WriteFile(tmp, data, 0o600) == nil {
		_ = os.Rename(tmp, m.settingsPath())
	}
}

// restoreWindow puts the window back where it was, through its creation
// options: before app.Run, calling the window's setters crashes Wails.
func (m *Messages) restoreWindow(o *application.WebviewWindowOptions) {
	s := m.GetSettings()
	if g := s.Window; g != nil && g.W > 200 && g.H > 200 {
		o.Width, o.Height = g.W, g.H
		o.InitialPosition, o.X, o.Y = application.WindowXY, g.X, g.Y
	}
	if s.Maximized {
		o.StartState = application.WindowStateMaximised
	}
}

// trackWindow remembers moves and resizes. Full screen and minimized
// aren't places to come back to; maximized is kept as a flag, over the
// place it was before.
func (m *Messages) trackWindow(w *application.WebviewWindow) {
	m.window = w
	remember := func(*application.WindowEvent) {
		if w.IsFullscreen() || w.IsMinimised() {
			return
		}
		maximized := w.IsMaximised()
		m.settingsMu.Lock()
		m.settings.Maximized = maximized
		if !maximized {
			x, y := w.Position()
			width, height := w.Size()
			m.settings.Window = &Geometry{X: x, Y: y, W: width, H: height}
		}
		m.settingsMu.Unlock()
		m.scheduleSave()
	}
	w.OnWindowEvent(wailevents.Common.WindowDidMove, remember)
	w.OnWindowEvent(wailevents.Common.WindowDidResize, remember)
	w.OnWindowEvent(wailevents.Common.WindowRuntimeReady, func(*application.WindowEvent) {
		setContentZoom(w, m.GetSettings().Zoom)
		// Saved on a monitor that's since gone: bring it back in view.
		if !w.IsMaximised() {
			var b application.Rect
			b.X, b.Y = w.Position()
			b.Width, b.Height = w.Size()
			if !onScreen(m.app, b) {
				w.Center()
			}
		}
	})
}
