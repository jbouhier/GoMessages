package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSettingsDefaults(t *testing.T) {
	for name, data := range map[string]string{"empty object": "{}", "corrupt": "{nope", "empty file": ""} {
		t.Run(name, func(t *testing.T) {
			got := parseSettings([]byte(data))
			if got.Zoom != 1 || !got.SidebarVisible || got.LinkPreviews != "contacts" || !got.EnterSends || got.Mode != "system" {
				t.Fatalf("%+v", got)
			}
		})
	}
}

// Fields a file leaves out keep their defaults: older settings files and new
// app versions keep working.
func TestSettingsPartialFileKeepsDefaults(t *testing.T) {
	got := parseSettings([]byte(`{"zoom": 1.5, "enter_sends": false}`))
	if got.Zoom != 1.5 || got.EnterSends {
		t.Fatalf("saved values lost: %+v", got)
	}
	if !got.SidebarVisible || !got.Notifications || got.SidebarWidth != 340 {
		t.Fatalf("defaults lost: %+v", got)
	}
}

func TestSettingsValuesAreClamped(t *testing.T) {
	cases := []struct {
		json string
		zoom float64
		tr   float64
		lp   string
	}{
		{`{"zoom": 0}`, 1, 0, "contacts"},
		{`{"zoom": -2}`, 1, 0, "contacts"},
		{`{"zoom": 0.1}`, 0.5, 0, "contacts"},
		{`{"zoom": 10}`, 3, 0, "contacts"},
		{`{"transparency": -5}`, 1, 0, "contacts"},
		{`{"transparency": 150}`, 1, 100, "contacts"},
		{`{"link_previews": "everyone"}`, 1, 0, "everyone"},
		{`{"link_previews": "off"}`, 1, 0, "off"},
		{`{"link_previews": "strangers"}`, 1, 0, "contacts"},
	}
	for _, tc := range cases {
		got := parseSettings([]byte(tc.json))
		if got.Zoom != tc.zoom || got.Transparency != tc.tr || got.LinkPreviews != tc.lp {
			t.Errorf("%s: zoom=%v transparency=%v previews=%q", tc.json, got.Zoom, got.Transparency, got.LinkPreviews)
		}
	}
}

func TestLoadSettings(t *testing.T) {
	dir := t.TempDir()
	if got := loadSettings(filepath.Join(dir, "missing.json")); got.Zoom != 1 || !got.SidebarVisible {
		t.Fatalf("missing file: %+v", got)
	}
	path := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(path, []byte(`{"sidebar_visible": false, "muted": ["c1"]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	got := loadSettings(path)
	if got.SidebarVisible || len(got.Muted) != 1 || got.Muted[0] != "c1" {
		t.Fatalf("%+v", got)
	}
}
