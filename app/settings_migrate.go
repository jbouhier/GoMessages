package main

import "encoding/json"

// Appearance settings have changed shape twice. migrateTheme carries older
// files over so the app looks as close to how it did as the themes allow:
//
//   - v0: "theme" (system, light, dark, a palette, clear) and "blur" levels.
//   - v1: "mode", "light_theme", "dark_theme", "transparency", "accent".
//   - now: "mode", "theme" (a color family with a light and a dark half),
//     "background", "transparency". Told apart from v0 by "mode", from v1
//     by no slots. Themes since renamed are in renamed.

// The family each old palette belongs to now.
var families = map[string]string{
	"system": "default", "light": "default", "dark": "default", "clear": "default",
	"latte": "mauve", "iced-latte": "mauve", "frappe": "mauve", "macchiato": "mauve", "mocha": "mauve", "espresso": "mauve",
	"sage": "sage", "peach": "peach", "aurora": "aurora", "sky": "sky", "glass": "sky",
}

// Themes renamed since: Horizon became Amber.
var renamed = map[string]string{"horizon": "amber"}

// v1's accents, for someone who kept Default and only changed the accent.
var accentFamilies = map[string]string{"purple": "mauve", "pink": "mauve", "green": "sage", "orange": "peach", "red": "peach"}

var lightPalettes = map[string]bool{"latte": true, "iced-latte": true, "sage": true, "peach": true}
var darkPalettes = map[string]bool{"frappe": true, "macchiato": true, "mocha": true, "espresso": true, "aurora": true, "sky": true}

// v0's blur levels as transparency: the panes were 60, 84, and 88%
// opaque. Clear's panes were 0, 14, and 40%.
var blurLevels = map[string]float64{"off": 0, "light": 40, "medium": 16, "heavy": 12}
var clearLevels = map[string]float64{"off": 0, "light": 100, "medium": 86, "heavy": 60}
var blurred = map[string]bool{"latte": true, "iced-latte": true, "frappe": true, "macchiato": true, "mocha": true, "espresso": true, "sky": true}

func migrateTheme(data []byte, s *Settings) {
	var old struct {
		Mode       *string `json:"mode"`
		Theme      *string `json:"theme"`
		Blur       string  `json:"blur"`
		LightTheme *string `json:"light_theme"`
		DarkTheme  *string `json:"dark_theme"`
		Accent     string  `json:"accent"`
	}
	if json.Unmarshal(data, &old) != nil {
		return
	}
	switch {
	case old.LightTheme != nil || old.DarkTheme != nil:
		light, dark := "light", "dark"
		if old.LightTheme != nil {
			light = *old.LightTheme
		}
		if old.DarkTheme != nil {
			dark = *old.DarkTheme
		}
		s.Theme = family(s.Mode, light, dark, old.Accent)
	case old.Mode == nil && old.Theme != nil:
		mode, light, dark, transparency := fromV0(*old.Theme, old.Blur)
		s.Mode, s.Transparency = mode, transparency
		s.Theme = family(mode, light, dark, "")
	}
	if to, ok := renamed[s.Theme]; ok {
		s.Theme = to
	}
	if s.Background != "solid" {
		s.Background = "gradient"
	}
}

// family picks the theme now: the slot in use; following the system, the
// slot that isn't Default (dark first, as the bolder pick).
func family(mode, light, dark, accent string) string {
	fl, fd := families[light], families[dark]
	f := fd
	switch {
	case mode == "light":
		f = fl
	case mode != "dark" && fd == "default":
		f = fl
	}
	if f == "" {
		f = "default"
	}
	if f == "default" && accentFamilies[accent] != "" {
		f = accentFamilies[accent]
	}
	return f
}

// fromV0 reads a v0 theme and blur level as v1's mode, slots, and transparency.
func fromV0(theme, blur string) (mode, light, dark string, transparency float64) {
	if blur == "" {
		blur = "medium"
	}
	if theme == "glass" {
		theme = "sky"
	}
	mode, light, dark = "system", "light", "dark"
	switch {
	case theme == "light" || theme == "dark":
		mode = theme
	case theme == "clear":
		transparency = clearLevels[blur]
	case lightPalettes[theme]:
		mode, light = "light", theme
	case darkPalettes[theme]:
		mode, dark = "dark", theme
	}
	if blurred[theme] {
		transparency = blurLevels[blur]
	}
	return
}
