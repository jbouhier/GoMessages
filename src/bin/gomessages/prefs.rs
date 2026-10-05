//! User preferences: one `settings.json` in the data dir.
//!
//! Every field has a default, so adding a setting is one field. Older
//! `zoom.json` / `window.json` files are migrated once, then removed.

use std::collections::HashMap;
use std::path::Path;

use serde::{Deserialize, Serialize};

use super::webview::{DEFAULT_IDX, PRESETS};

const FILE: &str = "settings.json";
const LEGACY_ZOOM: &str = "zoom.json";
const LEGACY_WINDOW: &str = "window.json";

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize, Default)]
#[serde(rename_all = "snake_case")]
pub enum NotificationSound {
    #[default]
    #[serde(alias = "google_messages", alias = "soft_bell", alias = "system_alert")]
    SystemNotification,
    Off,
}

/// Main window size + position, physical px.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub struct Geometry {
    pub x: i32,
    pub y: i32,
    pub w: u32,
    pub h: u32,
}

impl Geometry {
    fn valid(&self) -> bool {
        self.w >= 400 && self.h >= 300
    }
}

/// Window rectangle in logical pixels, relative to its monitor.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub struct MonitorGeometry {
    pub x: i32,
    pub y: i32,
    pub w: u32,
    pub h: u32,
}

impl MonitorGeometry {
    fn valid(&self) -> bool {
        self.w >= 400 && self.h >= 300
    }
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(default)]
pub struct DisplayProfile {
    pub zoom_idx: usize,
    pub window: Option<MonitorGeometry>,
}

impl Default for DisplayProfile {
    fn default() -> Self {
        Self {
            zoom_idx: DEFAULT_IDX,
            window: None,
        }
    }
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(default)]
pub struct Prefs {
    /// Last active zoom and geometry, retained for older settings files.
    pub zoom_idx: usize,
    pub notification_sound: NotificationSound,
    pub window: Option<Geometry>,
    pub displays: HashMap<String, DisplayProfile>,
    pub last_monitor: Option<String>,
    /// Closing the main window hides it instead of quitting.
    pub keep_running: bool,
    /// Menu bar icon (macOS) / tray icon (Windows, Linux).
    pub tray: bool,
    /// macOS only: no Dock icon. Requires `tray`, or the app is unreachable.
    pub hide_dock: bool,
}

impl Default for Prefs {
    fn default() -> Self {
        Self {
            zoom_idx: DEFAULT_IDX,
            notification_sound: NotificationSound::default(),
            window: None,
            displays: HashMap::new(),
            last_monitor: None,
            keep_running: true,
            // Windows: the tray is the only way back to a hidden window.
            tray: cfg!(target_os = "windows"),
            hide_dock: false,
        }
    }
}

impl Prefs {
    pub fn load() -> Self {
        gomessages::paths::data_dir()
            .map(|d| Self::load_from(&d))
            .unwrap_or_default()
    }

    pub fn save(&self) {
        if let Some(d) = gomessages::paths::data_dir() {
            self.save_to(&d);
        }
    }

    /// Closing hides only when there is a way back: the Dock (macOS) or a
    /// tray icon that actually exists (the setting alone isn't enough).
    pub fn close_hides(&self, tray_active: bool) -> bool {
        self.keep_running && (cfg!(target_os = "macos") || tray_active)
    }

    fn load_from(dir: &Path) -> Self {
        match std::fs::read(dir.join(FILE)) {
            Ok(bytes) => serde_json::from_slice::<Self>(&bytes)
                .unwrap_or_default()
                .sanitized(),
            Err(_) => {
                let p = Self::migrate(dir).sanitized();
                p.save_to(dir);
                if dir.join(FILE).exists() {
                    let _ = std::fs::remove_file(dir.join(LEGACY_ZOOM));
                    let _ = std::fs::remove_file(dir.join(LEGACY_WINDOW));
                }
                p
            }
        }
    }

    fn migrate(dir: &Path) -> Self {
        let mut p = Self::default();
        if let Some(idx) = std::fs::read(dir.join(LEGACY_ZOOM))
            .ok()
            .and_then(|b| serde_json::from_slice::<serde_json::Value>(&b).ok())
            .and_then(|v| v.get("idx")?.as_u64())
        {
            p.zoom_idx = idx as usize;
        }
        p.window = std::fs::read(dir.join(LEGACY_WINDOW))
            .ok()
            .and_then(|b| serde_json::from_slice(&b).ok());
        p
    }

    fn sanitized(mut self) -> Self {
        if self.zoom_idx >= PRESETS.len() {
            self.zoom_idx = DEFAULT_IDX;
        }
        self.window = self.window.filter(Geometry::valid);
        for profile in self.displays.values_mut() {
            if profile.zoom_idx >= PRESETS.len() {
                profile.zoom_idx = DEFAULT_IDX;
            }
            profile.window = profile.window.filter(MonitorGeometry::valid);
        }
        if !cfg!(target_os = "macos") || !self.tray {
            self.hide_dock = false;
        }
        self
    }

    /// Atomic write (tmp + rename) so a crash never leaves a torn file.
    fn save_to(&self, dir: &Path) {
        let _ = std::fs::create_dir_all(dir);
        let Ok(bytes) = serde_json::to_vec_pretty(self) else {
            return;
        };
        let path = dir.join(FILE);
        let tmp = path.with_extension("json.tmp");
        if std::fs::write(&tmp, bytes).is_ok() {
            let _ = std::fs::rename(&tmp, &path);
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn tmp_dir(name: &str) -> std::path::PathBuf {
        let d = std::env::temp_dir().join(format!("gomsg-prefs-{name}-{}", std::process::id()));
        let _ = std::fs::remove_dir_all(&d);
        std::fs::create_dir_all(&d).unwrap();
        d
    }

    #[test]
    fn round_trip() {
        let d = tmp_dir("rt");
        let mut displays = HashMap::new();
        displays.insert(
            "mac:123".into(),
            DisplayProfile {
                zoom_idx: 8,
                window: Some(MonitorGeometry {
                    x: 120,
                    y: 80,
                    w: 1000,
                    h: 650,
                }),
            },
        );
        let p = Prefs {
            zoom_idx: 10,
            notification_sound: NotificationSound::Off,
            window: Some(Geometry {
                x: 5,
                y: 6,
                w: 1200,
                h: 700,
            }),
            keep_running: false,
            tray: true,
            hide_dock: cfg!(target_os = "macos"),
            displays,
            last_monitor: Some("mac:123".into()),
        };
        p.save_to(&d);
        assert_eq!(Prefs::load_from(&d), p);
    }

    #[test]
    fn missing_fields_take_defaults() {
        let d = tmp_dir("partial");
        std::fs::write(d.join(FILE), br#"{"zoom_idx":9}"#).unwrap();
        let p = Prefs::load_from(&d);
        assert_eq!(p.zoom_idx, 9);
        assert_eq!(p.notification_sound, NotificationSound::SystemNotification);
        assert!(p.keep_running);
    }

    #[test]
    fn old_sound_choices_migrate_to_system_notification() {
        assert_eq!(
            serde_json::to_string(&NotificationSound::Off).unwrap(),
            "\"off\""
        );
        let d = tmp_dir("sound-migration");
        for old in ["google_messages", "soft_bell", "system_alert"] {
            std::fs::write(d.join(FILE), format!(r#"{{"notification_sound":"{old}"}}"#)).unwrap();
            assert_eq!(
                Prefs::load_from(&d).notification_sound,
                NotificationSound::SystemNotification
            );
        }
    }

    #[test]
    fn corrupt_or_out_of_range_is_safe() {
        let d = tmp_dir("bad");
        std::fs::write(d.join(FILE), b"{nope").unwrap();
        assert_eq!(Prefs::load_from(&d), Prefs::default());
        std::fs::write(
            d.join(FILE),
            br#"{"zoom_idx":99,"window":{"x":0,"y":0,"w":10,"h":10}}"#,
        )
        .unwrap();
        let p = Prefs::load_from(&d);
        assert_eq!(p.zoom_idx, DEFAULT_IDX);
        assert_eq!(p.window, None);
        std::fs::write(
            d.join(FILE),
            br#"{"displays":{"screen":{"zoom_idx":99,"window":{"x":0,"y":0,"w":10,"h":10}}}}"#,
        )
        .unwrap();
        let p = Prefs::load_from(&d);
        assert_eq!(p.displays["screen"].zoom_idx, DEFAULT_IDX);
        assert_eq!(p.displays["screen"].window, None);
    }

    #[test]
    fn hide_dock_needs_tray() {
        let d = tmp_dir("dock");
        std::fs::write(d.join(FILE), br#"{"tray":false,"hide_dock":true}"#).unwrap();
        assert!(!Prefs::load_from(&d).hide_dock);
    }

    #[test]
    fn migrates_legacy_files() {
        let d = tmp_dir("legacy");
        std::fs::write(d.join(LEGACY_ZOOM), br#"{"idx":11}"#).unwrap();
        std::fs::write(d.join(LEGACY_WINDOW), br#"{"x":1,"y":2,"w":1000,"h":700}"#).unwrap();
        let p = Prefs::load_from(&d);
        assert_eq!(p.zoom_idx, 11);
        assert_eq!(p.window.map(|g| g.w), Some(1000));
        assert!(d.join(FILE).exists());
        assert!(!d.join(LEGACY_ZOOM).exists() && !d.join(LEGACY_WINDOW).exists());
    }

    #[test]
    fn close_hides_needs_a_way_back() {
        let p = Prefs {
            keep_running: true,
            tray: true,
            ..Prefs::default()
        };
        // Tray on in settings but unavailable: no way back on Win/Linux.
        assert_eq!(p.close_hides(false), cfg!(target_os = "macos"));
        assert!(p.close_hides(true));
        let p = Prefs {
            keep_running: false,
            ..p
        };
        assert!(!p.close_hides(true));
    }
}
