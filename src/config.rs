//! Global app config. Change it here, it applies everywhere.
//!
//! Paths live in [`crate::paths`], which reads the triple below.

/// User-visible name.
pub const APP_NAME: &str = "GoMessages";
/// Page hosted in the main webview.
pub const APP_URL: &str = "https://messages.google.com/web";
/// Crate version, shown in Settings and About.
pub const VERSION: &str = env!("CARGO_PKG_VERSION");

/// Data dir triple. Renaming these moves the profile (login, pairing, zoom).
pub const DATA_QUALIFIER: &str = "dev";
pub const DATA_ORG: &str = "go-messages";
pub const DATA_APP: &str = "go-messages";

/// File/dir names inside the data dir.
pub const PROFILE_SUBDIR: &str = "webview-profile";
pub const UNPAIR_FLAG_FILE: &str = "UNPAIR_ON_NEXT_LAUNCH";
pub const CHAT_FILE: &str = "chat.json";
pub const AUTH_FILE: &str = "auth.json";

/// Main window geometry (logical px).
pub const WINDOW_WIDTH: f64 = 1280.0;
pub const WINDOW_HEIGHT: f64 = 800.0;
pub const WINDOW_MIN_WIDTH: f64 = 900.0;
pub const WINDOW_MIN_HEIGHT: f64 = 560.0;
/// Settings window geometry (logical px, fixed size).
pub const SETTINGS_WIDTH: f64 = 470.0;
pub const SETTINGS_HEIGHT: f64 = 650.0;
/// Strip above the page for traffic lights (macOS transparent titlebar).
#[cfg(target_os = "macos")]
pub const TOP_INSET: f64 = 28.0;
#[cfg(not(target_os = "macos"))]
pub const TOP_INSET: f64 = 0.0;
