//! GoMessages: native window with embedded real Messages page (`wry`).
//!
//! The page owns contacts, search, and auth. This binary is a thin native
//! frame: traffic-lights window, persistent profile, webview zoom, app menu
//! with Settings (`Cmd+,`), a local settings window, background mode with a
//! menu bar / tray icon, and an unread badge.
//!
//! Split: [`menu`] builds the menubar, [`settings`] holds the settings page,
//! [`webview`] the init script and bounds, [`app`] the windows and IPC,
//! [`prefs`] `settings.json`, [`tray`] the tray icon, [`platform`] per-OS glue.

mod app;
mod display;
mod menu;
mod platform;
mod prefs;
mod settings;
mod tray;
mod webview;

use app::GoMessages;
use gomessages::paths;
use muda::MenuId;
use winit::application::ApplicationHandler;
use winit::event::WindowEvent;
use winit::event_loop::{ActiveEventLoop, ControlFlow, EventLoop};
use winit::window::WindowId;

#[derive(Debug)]
enum UserEvent {
    Menu(MenuId),
    Ipc(String),
    /// Main page finished loading: re-apply remembered zoom.
    PageLoaded,
    /// Unread count from the page; baseline readings never play a sound.
    Unread {
        count: u32,
        baseline: bool,
    },
    /// macOS Dock click / relaunch while running.
    #[cfg_attr(not(target_os = "macos"), allow(dead_code))]
    Reopen,
    /// Windows tray left click.
    TrayClick,
}

/// App data dir (see [`paths`]).
fn data_dir() -> Option<std::path::PathBuf> {
    paths::data_dir()
}

fn profile_dir() -> Option<std::path::PathBuf> {
    data_dir().map(|d| d.join(gomessages::config::PROFILE_SUBDIR))
}

fn unpair_flag() -> Option<std::path::PathBuf> {
    data_dir().map(|d| d.join(gomessages::config::UNPAIR_FLAG_FILE))
}

struct Bootstrap {
    app: Option<GoMessages>,
    prefs: Option<prefs::Prefs>,
    proxy: winit::event_loop::EventLoopProxy<UserEvent>,
    marketing_demo: bool,
    marketing_settings: bool,
}

impl ApplicationHandler<UserEvent> for Bootstrap {
    fn resumed(&mut self, event_loop: &ActiveEventLoop) {
        // Guard re-entry (macOS can call this more than once): menu thread
        // and windows must be created exactly once.
        if self.app.is_some() {
            return;
        }
        event_loop.set_control_flow(ControlFlow::Wait);

        // Fresh-start unpair: wipe the profile before the webview is created.
        if let Some(f) = unpair_flag() {
            if f.exists() {
                if let Some(profile) = profile_dir() {
                    let _ = std::fs::remove_dir_all(profile);
                }
                let _ = std::fs::remove_file(f);
            }
        }

        if let Some(d) = data_dir() {
            let _ = std::fs::create_dir_all(d.join(gomessages::config::PROFILE_SUBDIR));
        }

        let (menu, ids) = app::wire_menu_thread(&self.proxy);
        app::wire_tray_events(&self.proxy);
        let prefs = self.prefs.take().unwrap_or_default();
        let mut state = GoMessages::new(self.proxy.clone(), menu, ids, prefs);
        if app::build_main_window(event_loop, &mut state, &self.proxy, self.marketing_demo).is_err()
        {
            event_loop.exit();
            return;
        }
        if self.marketing_settings {
            state.open_settings(event_loop);
        }
        state.start_background();
        self.app = Some(state);
    }

    /// Check display changes and save settled geometry. Linux also needs GTK
    /// pumped at ~60 Hz because winit doesn't run its loop.
    fn about_to_wait(&mut self, event_loop: &ActiveEventLoop) {
        let now = std::time::Instant::now();
        let mut deadline = self
            .app
            .as_mut()
            .map(|app| app.tick(now))
            .unwrap_or(now + std::time::Duration::from_secs(1));
        if cfg!(target_os = "linux") {
            platform::pump_toolkit();
            deadline = deadline.min(now + std::time::Duration::from_millis(16));
        }
        event_loop.set_control_flow(ControlFlow::WaitUntil(deadline));
    }

    fn user_event(&mut self, event_loop: &ActiveEventLoop, event: UserEvent) {
        let Some(app) = self.app.as_mut() else { return };
        app.on_user_event(event_loop, event);
    }

    fn window_event(&mut self, event_loop: &ActiveEventLoop, id: WindowId, event: WindowEvent) {
        let Some(app) = self.app.as_mut() else { return };
        app.on_window_event(event_loop, id, event);
    }
}

fn main() {
    let marketing_demo = std::env::args_os().any(|arg| arg == "--marketing-demo");
    let marketing_settings = std::env::args_os().any(|arg| arg == "--marketing-settings");
    if marketing_settings && !marketing_demo {
        eprintln!("--marketing-settings requires --marketing-demo");
        std::process::exit(2);
    }
    if marketing_demo {
        let data_dir =
            std::env::var_os("GOMESSAGES_MARKETING_DATA_DIR").map(std::path::PathBuf::from);
        if !data_dir.as_ref().is_some_and(|path| path.is_absolute()) {
            eprintln!("--marketing-demo requires an absolute GOMESSAGES_MARKETING_DATA_DIR");
            std::process::exit(2);
        }
        std::env::set_var("GOMESSAGES_MARKETING_DEMO", "1");
    }
    if !platform::init_toolkit() {
        eprintln!("GoMessages: could not initialize GTK (is a display available?)");
        std::process::exit(1);
    }
    let prefs = prefs::Prefs::load();
    let mut builder = EventLoop::<UserEvent>::with_user_event();
    #[cfg(target_os = "macos")]
    {
        use winit::platform::macos::{ActivationPolicy, EventLoopBuilderExtMacOS};
        // Decided before launch so a hidden Dock icon never flashes.
        builder.with_activation_policy(if prefs.hide_dock {
            ActivationPolicy::Accessory
        } else {
            ActivationPolicy::Regular
        });
    }
    let event_loop = builder.build().expect("event loop");
    let proxy = event_loop.create_proxy();
    let mut boot = Bootstrap {
        app: None,
        prefs: Some(prefs),
        proxy,
        marketing_demo,
        marketing_settings,
    };
    // Exit-time platform errors (e.g. late X11/GL errors) shouldn't panic.
    if let Err(e) = event_loop.run_app(&mut boot) {
        eprintln!("GoMessages: event loop ended with an error: {e}");
    }
    if let Some(app) = &mut boot.app {
        app.save_on_exit();
    }
}
