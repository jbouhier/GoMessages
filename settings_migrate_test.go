package main

import "testing"

func TestMigrateTheme(t *testing.T) {
	cases := []struct {
		file         string
		mode, theme  string
		transparency float64
	}{
		// v0: one theme and blur levels.
		{`{"theme":"system","blur":"medium"}`, "system", "default", 0},
		{`{"theme":"dark"}`, "dark", "default", 0},
		{`{"theme":"mocha","blur":"medium"}`, "dark", "mauve", 16},
		{`{"theme":"latte","blur":"light"}`, "light", "mauve", 40},
		{`{"theme":"espresso","blur":"off"}`, "dark", "mauve", 0},
		{`{"theme":"sage","blur":"heavy"}`, "light", "sage", 0},
		{`{"theme":"glass","blur":"heavy"}`, "dark", "sky", 12},
		{`{"theme":"clear","blur":"light"}`, "system", "default", 100},
		{`{"theme":"clear","blur":"heavy"}`, "system", "default", 60},
		{`{"theme":"nonsense"}`, "system", "default", 0},
		// v1: a light and a dark slot, and an accent.
		{`{"mode":"light","light_theme":"peach","dark_theme":"aurora","transparency":30}`, "light", "peach", 30},
		{`{"mode":"dark","light_theme":"peach","dark_theme":"aurora"}`, "dark", "aurora", 0},
		{`{"mode":"system","light_theme":"sage","dark_theme":"dark"}`, "system", "sage", 0},
		{`{"mode":"system","light_theme":"latte","dark_theme":"sky"}`, "system", "sky", 0},
		{`{"mode":"dark","light_theme":"light","dark_theme":"dark","accent":"green"}`, "dark", "sage", 0},
		{`{"mode":"dark","light_theme":"light","dark_theme":"frappe","accent":"green"}`, "dark", "mauve", 0},
		// Now: left alone, but for renamed themes.
		{`{"mode":"dark","theme":"peach","transparency":20}`, "dark", "peach", 20},
		{`{"mode":"light","theme":"horizon"}`, "light", "amber", 0},
		{`{"mode":"dark","theme":"aurora"}`, "dark", "aurora", 0},
		{`{}`, "system", "default", 0},
	}
	for _, c := range cases {
		s := parseSettings([]byte(c.file))
		if s.Mode != c.mode || s.Theme != c.theme || s.Transparency != c.transparency {
			t.Errorf("%s: got %s/%s/%v, want %s/%s/%v", c.file, s.Mode, s.Theme, s.Transparency, c.mode, c.theme, c.transparency)
		}
	}
	if s := parseSettings([]byte(`{"transparency":250}`)); s.Transparency != 100 {
		t.Errorf("transparency is kept to 0–100, got %v", s.Transparency)
	}
	for file, want := range map[string]string{`{}`: "gradient", `{"background":"solid"}`: "solid", `{"background":"odd"}`: "gradient"} {
		if s := parseSettings([]byte(file)); s.Background != want {
			t.Errorf("%s: background %q, want %q", file, s.Background, want)
		}
	}
}
