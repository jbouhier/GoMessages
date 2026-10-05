//! App state: windows, webviews, menu + tray actions, settings IPC,
//! background mode (hide on close), unread badge.

use std::collections::{HashMap, HashSet};
use std::time::{Duration, Instant};

use muda::Menu;
use winit::window::{Window, WindowId};

use super::display::{self, Display};
use super::menu::{build_menu, MenuIds};
use super::platform;
use super::prefs::{DisplayProfile, Geometry, NotificationSound, Prefs};
use super::settings::{prefs_state_json, settings_html, zoom_state_args};
use super::tray::Tray;
use super::webview::{fullscreen_bounds, navigation_allowed, webview_bounds, INIT_JS, PRESETS};
use super::UserEvent;
use gomessages::config;
use winit::event::WindowEvent;
use winit::event_loop::ActiveEventLoop;

#[derive(Clone, Copy, PartialEq, Eq)]
pub enum Role {
    Main,
    Settings,
}

const MOVE_SETTLE: Duration = Duration::from_millis(650);
const RESTORE_SETTLE: Duration = Duration::from_millis(350);

struct PendingDisplay {
    display: Display,
    deadline: Instant,
    forced: bool,
}

pub struct GoMessages {
    windows: HashMap<WindowId, Role>,
    main_window: Option<Window>,
    main_view: Option<wry::WebView>,
    settings_window: Option<Window>,
    settings_view: Option<wry::WebView>,
    #[allow(dead_code)] // must stay alive: dropping the Menu removes the menubar
    menu: Menu,
    ids: MenuIds,
    proxy: winit::event_loop::EventLoopProxy<UserEvent>,
    prefs: Prefs,
    tray: Option<Tray>,
    /// False when the tray was asked for but the desktop has none.
    tray_ok: bool,
    unread: u32,
    active_display: Option<Display>,
    pending_display: Option<PendingDisplay>,
    using_fallback: bool,
    restoring_until: Option<Instant>,
    geometry_save_at: Option<Instant>,
    pending_geometry: Option<(String, Geometry)>,
}

fn profile_dir() -> Option<std::path::PathBuf> {
    super::data_dir().map(|d| d.join(config::PROFILE_SUBDIR))
}

/// `unread:N` or `unread-baseline:N` from the page poller.
pub fn parse_unread(msg: &str) -> Option<(u32, bool)> {
    if let Some(n) = msg.strip_prefix("unread-baseline:") {
        return n.parse().ok().map(|n| (n, true));
    }
    msg.strip_prefix("unread:")?
        .parse()
        .ok()
        .map(|n| (n, false))
}

/// `pref:<key>:<0|1>` from the settings page.
pub fn parse_pref(msg: &str) -> Option<(&str, bool)> {
    let (key, val) = msg.strip_prefix("pref:")?.split_once(':')?;
    match val {
        "1" => Some((key, true)),
        "0" => Some((key, false)),
        _ => None,
    }
}

fn should_play_sound(
    previous: u32,
    current: u32,
    baseline: bool,
    selected: NotificationSound,
) -> bool {
    !baseline && current > previous && selected != NotificationSound::Off
}

/// `None` when this desktop has no tray (Linux without AppIndicator).
fn try_tray(unread: u32) -> Option<Tray> {
    if !platform::tray_supported() {
        return None;
    }
    std::panic::catch_unwind(|| Tray::new(unread))
        .ok()
        .flatten()
}

impl GoMessages {
    pub fn new(
        proxy: winit::event_loop::EventLoopProxy<UserEvent>,
        menu: Menu,
        ids: MenuIds,
        prefs: Prefs,
    ) -> Self {
        Self {
            windows: HashMap::new(),
            main_window: None,
            main_view: None,
            settings_window: None,
            settings_view: None,
            menu,
            ids,
            proxy,
            prefs,
            tray: None,
            tray_ok: true,
            unread: 0,
            active_display: None,
            pending_display: None,
            using_fallback: false,
            restoring_until: None,
            geometry_save_at: None,
            pending_geometry: None,
        }
    }

    /// Pick the last used display when connected. Existing single-display
    /// settings become that display's first profile.
    fn startup_geometry(&mut self, event_loop: &ActiveEventLoop) -> Option<Geometry> {
        let displays: Vec<Display> = event_loop
            .available_monitors()
            .map(|m| Display::from_monitor(&m))
            .collect();
        let chosen = self
            .prefs
            .last_monitor
            .as_ref()
            .and_then(|key| displays.iter().find(|d| &d.key == key))
            .or_else(|| {
                if self.prefs.displays.is_empty() {
                    self.prefs
                        .window
                        .and_then(|g| displays.iter().find(|d| d.contains_center(g)))
                } else {
                    None
                }
            })
            .or_else(|| {
                let key = event_loop
                    .primary_monitor()
                    .as_ref()
                    .map(|m| Display::from_monitor(m).key)?;
                displays.iter().find(|d| d.key == key)
            })
            .or_else(|| displays.first())
            .cloned();
        let Some(display) = chosen else {
            return self.prefs.window;
        };
        self.using_fallback = self
            .prefs
            .last_monitor
            .as_ref()
            .is_some_and(|key| key != &display.key);
        // Initial window events are generated by the OS. Keep them from
        // treating a fallback placement as a user decision.
        if self.using_fallback {
            self.restoring_until = Some(Instant::now() + RESTORE_SETTLE);
        }
        let legacy_window = if self.prefs.displays.is_empty() {
            self.prefs.window.filter(|g| display.contains_center(*g))
        } else {
            None
        };
        let old_zoom = self.prefs.zoom_idx;
        let profile = self
            .prefs
            .displays
            .entry(display.key.clone())
            .or_insert_with(|| DisplayProfile {
                zoom_idx: old_zoom,
                window: legacy_window.map(|g| display.relative(g)),
            });
        self.prefs.zoom_idx = profile.zoom_idx;
        let geometry = profile
            .window
            .map(|g| display.absolute(g))
            .unwrap_or_else(|| display.default_geometry());
        profile.window = Some(display.relative(geometry));
        self.prefs.window = Some(geometry);
        if self.prefs.last_monitor.is_none() {
            self.prefs.last_monitor = Some(display.key.clone());
        }
        self.active_display = Some(display);
        self.prefs.save();
        Some(geometry)
    }

    pub fn track_main(&mut self, window: Window, view: wry::WebView) {
        self.windows.insert(window.id(), Role::Main);
        self.main_window = Some(window);
        self.main_view = Some(view);
        self.observe_display(Instant::now(), false);
        if self.pending_display.is_none() {
            self.record_geometry(false);
        }
    }

    fn record_geometry(&mut self, user_action: bool) {
        if self.pending_display.is_some() || self.restoring_until.is_some() {
            return;
        }
        let (Some(window), Some(display)) = (&self.main_window, &self.active_display) else {
            return;
        };
        let Some(geometry) = display::window_geometry(window) else {
            return;
        };
        self.pending_geometry = Some((display.key.clone(), geometry));
        self.geometry_save_at = Some(Instant::now() + MOVE_SETTLE);
        if user_action && self.using_fallback {
            self.using_fallback = false;
            self.prefs.last_monitor = Some(display.key.clone());
            self.geometry_save_at = Some(Instant::now() + MOVE_SETTLE);
        }
    }

    fn observe_display(&mut self, now: Instant, moving: bool) {
        if self.restoring_until.is_some_and(|until| now < until) {
            return;
        }
        let Some(window) = &self.main_window else {
            return;
        };
        let Some(current) = Display::current(window) else {
            return;
        };
        let available: HashSet<String> = window
            .available_monitors()
            .map(|m| Display::from_monitor(&m).key)
            .collect();
        if self
            .active_display
            .as_ref()
            .is_some_and(|d| d.key == current.key)
        {
            self.active_display = Some(current.clone());
            if let Some(pending) = &mut self.pending_display {
                pending.display = current;
                if moving {
                    pending.deadline = now + MOVE_SETTLE;
                }
            }
            return;
        }
        let forced = self
            .active_display
            .as_ref()
            .is_some_and(|old| !available.contains(&old.key));
        let old_zoom = self.prefs.zoom_idx;
        let zoom = self
            .prefs
            .displays
            .entry(current.key.clone())
            .or_insert_with(|| DisplayProfile {
                zoom_idx: old_zoom,
                window: None,
            })
            .zoom_idx;
        self.active_display = Some(current.clone());
        self.pending_geometry = None;
        self.geometry_save_at = None;
        self.prefs.zoom_idx = zoom;
        if zoom != old_zoom {
            self.push_zoom_to_page();
            self.show_zoom_toast();
            self.sync_settings_zoom();
        }
        self.pending_display = Some(PendingDisplay {
            display: current,
            deadline: now + MOVE_SETTLE,
            forced,
        });
    }

    fn restore_on(&mut self, display: &Display, now: Instant) {
        self.pending_geometry = None;
        let geometry = self
            .prefs
            .displays
            .get(&display.key)
            .and_then(|profile| profile.window)
            .map(|g| display.absolute(g))
            .unwrap_or_else(|| display.default_geometry());
        if let Some(window) = &self.main_window {
            let _ =
                window.request_inner_size(winit::dpi::PhysicalSize::new(geometry.w, geometry.h));
            window.set_outer_position(winit::dpi::PhysicalPosition::new(geometry.x, geometry.y));
        }
        self.prefs.window = Some(geometry);
        self.restoring_until = Some(now + RESTORE_SETTLE);
        self.geometry_save_at = Some(now + RESTORE_SETTLE + MOVE_SETTLE);
    }

    /// Called after window events and periodically for display hotplug changes.
    pub fn tick(&mut self, now: Instant) -> Instant {
        self.observe_display(now, false);
        if self.using_fallback && self.pending_display.is_none() && self.restoring_until.is_none() {
            if let (Some(window), Some(preferred)) = (&self.main_window, &self.prefs.last_monitor) {
                let returned = window
                    .available_monitors()
                    .map(|m| Display::from_monitor(&m))
                    .find(|d| &d.key == preferred);
                if let Some(display) = returned {
                    if self
                        .active_display
                        .as_ref()
                        .is_none_or(|d| d.key != display.key)
                    {
                        self.active_display = Some(display.clone());
                        self.prefs.zoom_idx = self
                            .prefs
                            .displays
                            .get(&display.key)
                            .map(|profile| profile.zoom_idx)
                            .unwrap_or(self.prefs.zoom_idx);
                        self.push_zoom_to_page();
                        self.sync_settings_zoom();
                        self.restore_on(&display, now);
                        self.using_fallback = false;
                        self.prefs.save();
                    }
                }
            }
        }
        if self
            .pending_display
            .as_ref()
            .is_some_and(|p| now >= p.deadline)
        {
            let pending = self.pending_display.take().unwrap();
            let same_monitor = self
                .main_window
                .as_ref()
                .and_then(Display::current)
                .is_some_and(|d| d.key == pending.display.key);
            if same_monitor {
                self.restore_on(&pending.display, now);
                if pending.forced {
                    self.using_fallback = true;
                } else {
                    self.using_fallback = false;
                    self.prefs.last_monitor = Some(pending.display.key);
                }
                self.prefs.save();
            } else {
                self.observe_display(now, false);
            }
        }
        if self.restoring_until.is_some_and(|until| now >= until) {
            self.restoring_until = None;
            self.record_geometry(false);
        }
        if self
            .geometry_save_at
            .is_some_and(|deadline| now >= deadline)
        {
            self.geometry_save_at = None;
            self.commit_geometry();
        }
        [
            self.pending_display.as_ref().map(|p| p.deadline),
            self.restoring_until,
            self.geometry_save_at,
        ]
        .into_iter()
        .flatten()
        .min()
        .unwrap_or(now + Duration::from_secs(1))
    }

    fn commit_geometry(&mut self) {
        let Some((key, geometry)) = self.pending_geometry.take() else {
            return;
        };
        let Some(display) = self.active_display.as_ref().filter(|d| d.key == key) else {
            return;
        };
        self.prefs.displays.entry(key).or_default().window = Some(display.relative(geometry));
        self.prefs.window = Some(geometry);
        self.prefs.save();
    }

    pub fn save_on_exit(&mut self) {
        self.commit_geometry();
        self.prefs.save();
    }

    /// After the main window exists: tray, Dock state, Dock reopen hook.
    pub fn start_background(&mut self) {
        platform::install_reopen_handler(self.proxy.clone());
        if self.prefs.tray {
            self.set_tray(true);
        }
        // Dock was hidden at launch via the activation policy; if the tray
        // failed, bring it back or the app would be unreachable.
        if self.prefs.hide_dock && self.tray.is_none() {
            self.prefs.hide_dock = false;
            platform::set_dock_visible(true);
        }
    }

    fn set_tray(&mut self, on: bool) {
        if on && self.tray.is_none() {
            self.tray = try_tray(self.unread);
            self.tray_ok = self.tray.is_some();
        } else if !on {
            self.tray = None;
            self.tray_ok = true;
        }
    }

    pub fn show_main(&self) {
        if let Some(w) = &self.main_window {
            w.set_visible(true);
            w.set_minimized(false);
            w.focus_window();
        }
        platform::activate();
    }

    fn hide_main(&self) {
        if let Some(w) = &self.main_window {
            w.set_visible(false);
        }
    }

    fn toggle_main(&self) {
        let visible = self
            .main_window
            .as_ref()
            .and_then(|w| w.is_visible())
            .unwrap_or(true);
        if visible {
            self.hide_main();
        } else {
            self.show_main();
        }
    }

    pub fn open_settings(&mut self, event_loop: &ActiveEventLoop) {
        platform::activate();
        if let Some(w) = &self.settings_window {
            w.focus_window();
            return;
        }
        let attrs = Window::default_attributes()
            .with_title("GoMessages Settings")
            .with_inner_size(winit::dpi::LogicalSize::new(
                config::SETTINGS_WIDTH,
                config::SETTINGS_HEIGHT,
            ))
            .with_resizable(false);
        let window = event_loop.create_window(attrs).expect("settings window");
        let proxy = self.proxy.clone();
        let view = wry::WebViewBuilder::new()
            .with_html(settings_html(&self.prefs, self.tray_ok))
            .with_background_color((0, 0, 0, 0))
            .with_bounds(fullscreen_bounds(&window, 0.0))
            .with_ipc_handler(move |req: http::Request<String>| {
                let _ = proxy.send_event(UserEvent::Ipc(req.body().clone()));
            })
            .build_as_child(&window)
            .expect("settings webview");
        window.focus_window();
        self.windows.insert(window.id(), Role::Settings);
        self.settings_window = Some(window);
        self.settings_view = Some(view);
    }

    fn set_zoom(&mut self, idx: usize) {
        self.prefs.zoom_idx = idx.min(PRESETS.len() - 1);
        if let Some(display) = &self.active_display {
            self.prefs
                .displays
                .entry(display.key.clone())
                .or_default()
                .zoom_idx = self.prefs.zoom_idx;
        }
        self.prefs.save();
        if self.push_zoom_to_page() {
            self.show_zoom_toast();
        }
        self.sync_settings_zoom();
    }

    /// Apply browser zoom so fixed overlays and page menus share viewport coordinates.
    fn push_zoom_to_page(&self) -> bool {
        self.main_view
            .as_ref()
            .is_some_and(|v| v.zoom(PRESETS[self.prefs.zoom_idx] / 100.0).is_ok())
    }

    fn show_zoom_toast(&self) {
        if let Some(v) = &self.main_view {
            let js = format!(
                "window.__gomsgShowZoom&&window.__gomsgShowZoom({})",
                PRESETS[self.prefs.zoom_idx]
            );
            let _ = v.evaluate_script(&js);
        }
    }

    /// Refresh the Settings readout and button states, if that window is open.
    fn sync_settings_zoom(&self) {
        if let Some(v) = &self.settings_view {
            let js = format!(
                "window.__setZoom&&__setZoom({})",
                zoom_state_args(self.prefs.zoom_idx)
            );
            let _ = v.evaluate_script(&js);
        }
    }

    fn sync_settings_prefs(&self) {
        if let Some(v) = &self.settings_view {
            let js = format!(
                "window.__setPrefs&&__setPrefs({})",
                prefs_state_json(&self.prefs, self.tray_ok)
            );
            let _ = v.evaluate_script(&js);
        }
    }

    /// Settings toggles. Rules: hiding the Dock needs the tray; turning the
    /// tray off brings the Dock back.
    fn set_pref(&mut self, key: &str, on: bool) {
        match key {
            "keep_running" => self.prefs.keep_running = on,
            "tray" => {
                self.set_tray(on);
                self.prefs.tray = on && self.tray.is_some();
                if !self.prefs.tray && self.prefs.hide_dock {
                    self.prefs.hide_dock = false;
                    platform::set_dock_visible(true);
                }
            }
            "hide_dock" if cfg!(target_os = "macos") => {
                let hide = on && self.prefs.tray;
                if hide != self.prefs.hide_dock {
                    self.prefs.hide_dock = hide;
                    platform::set_dock_visible(!hide);
                    // Keep Settings in front after the policy flip.
                    if let Some(w) = &self.settings_window {
                        w.focus_window();
                    }
                }
            }
            _ => return,
        }
        self.prefs.save();
        self.sync_settings_prefs();
    }

    fn set_notification_sound(&mut self, sound: NotificationSound) {
        self.prefs.notification_sound = sound;
        self.prefs.save();
        self.sync_settings_prefs();
    }

    fn set_unread(&mut self, unread: u32, baseline: bool) {
        if unread == self.unread {
            return;
        }
        if should_play_sound(self.unread, unread, baseline, self.prefs.notification_sound) {
            platform::play_system_notification();
        }
        self.unread = unread;
        platform::set_badge(unread);
        if let Some(t) = self.tray.as_mut() {
            t.set_unread(unread);
        }
    }

    fn handle_ipc(&mut self, event_loop: &ActiveEventLoop, msg: &str) {
        use super::webview::DEFAULT_IDX;
        if let Some((key, on)) = parse_pref(msg) {
            self.set_pref(key, on);
            return;
        }
        match msg {
            "sound:on" => self.set_notification_sound(NotificationSound::SystemNotification),
            "sound:off" => self.set_notification_sound(NotificationSound::Off),
            "sound-preview" => {
                platform::play_system_notification();
            }
            "zoom-in" => self.set_zoom(self.prefs.zoom_idx + 1),
            "zoom-out" => self.set_zoom(self.prefs.zoom_idx.saturating_sub(1)),
            "zoom-reset" => self.set_zoom(DEFAULT_IDX),
            "unpair" => {
                if let Some(f) = super::unpair_flag() {
                    if let Some(parent) = f.parent() {
                        let _ = std::fs::create_dir_all(parent);
                    }
                    let _ = std::fs::write(f, b"1");
                }
                event_loop.exit();
            }
            "open-settings" => self.open_settings(event_loop),
            _ => {}
        }
    }

    pub fn on_user_event(&mut self, event_loop: &ActiveEventLoop, event: UserEvent) {
        match event {
            UserEvent::Ipc(msg) => self.handle_ipc(event_loop, &msg),
            UserEvent::PageLoaded => {
                self.push_zoom_to_page();
            }
            UserEvent::Unread { count, baseline } => self.set_unread(count, baseline),
            UserEvent::Reopen => self.show_main(),
            UserEvent::TrayClick => self.toggle_main(),
            UserEvent::Menu(id) => self.on_menu(event_loop, id),
        }
    }

    fn on_menu(&mut self, event_loop: &ActiveEventLoop, id: muda::MenuId) {
        use super::webview::DEFAULT_IDX;
        if id == self.ids.settings {
            self.open_settings(event_loop);
        } else if id == self.ids.zoom_in {
            self.set_zoom(self.prefs.zoom_idx + 1);
        } else if id == self.ids.zoom_out {
            self.set_zoom(self.prefs.zoom_idx.saturating_sub(1));
        } else if id == self.ids.zoom_reset {
            self.set_zoom(DEFAULT_IDX);
        } else if let Some(t) = &self.tray {
            if id == t.open {
                self.show_main();
            } else if id == t.settings {
                self.open_settings(event_loop);
            } else if id == t.quit {
                event_loop.exit();
            }
        }
        // Predefined items (Quit, window ops) act natively; no handling needed.
    }

    pub fn on_window_event(
        &mut self,
        event_loop: &ActiveEventLoop,
        id: WindowId,
        event: WindowEvent,
    ) {
        match self.windows.get(&id).copied() {
            Some(Role::Main) => match event {
                WindowEvent::CloseRequested => {
                    self.commit_geometry();
                    self.prefs.save();
                    if self.prefs.close_hides(self.tray.is_some()) {
                        self.hide_main();
                    } else {
                        event_loop.exit();
                    }
                }
                event @ (WindowEvent::Resized(_)
                | WindowEvent::Moved(_)
                | WindowEvent::ScaleFactorChanged { .. }) => {
                    if let (Some(w), Some(v)) = (&self.main_window, &self.main_view) {
                        let _ = v.set_bounds(webview_bounds(w));
                    }
                    let user_action = !matches!(event, WindowEvent::ScaleFactorChanged { .. });
                    self.observe_display(Instant::now(), user_action);
                    self.record_geometry(user_action);
                }
                _ => {}
            },
            Some(Role::Settings) => match event {
                WindowEvent::CloseRequested => {
                    self.windows.remove(&id);
                    self.settings_window = None;
                    self.settings_view = None;
                }
                WindowEvent::Resized(_) => {
                    if let (Some(w), Some(v)) = (&self.settings_window, &self.settings_view) {
                        let _ = v.set_bounds(fullscreen_bounds(w, 0.0));
                    }
                }
                _ => {}
            },
            None => {}
        }
    }
}

pub fn build_main_window(
    event_loop: &ActiveEventLoop,
    app: &mut GoMessages,
    proxy: &winit::event_loop::EventLoopProxy<UserEvent>,
    marketing_demo: bool,
) -> anyhow::Result<()> {
    use winit::window::Window;
    let attrs = Window::default_attributes().with_title(config::APP_NAME);
    let attrs = match app.startup_geometry(event_loop) {
        Some(g) => attrs
            .with_position(winit::dpi::PhysicalPosition::new(g.x, g.y))
            .with_inner_size(winit::dpi::PhysicalSize::new(g.w, g.h)),
        None => attrs.with_inner_size(winit::dpi::LogicalSize::new(
            config::WINDOW_WIDTH,
            config::WINDOW_HEIGHT,
        )),
    };
    let attrs = attrs.with_min_inner_size(winit::dpi::LogicalSize::new(
        config::WINDOW_MIN_WIDTH,
        config::WINDOW_MIN_HEIGHT,
    ));
    #[cfg(target_os = "macos")]
    let attrs = {
        use winit::platform::macos::WindowAttributesExtMacOS;
        attrs
            .with_titlebar_transparent(true)
            .with_title_hidden(true)
            .with_fullsize_content_view(true)
    };
    let window = event_loop.create_window(attrs)?;
    let mut context = wry::WebContext::new(profile_dir());
    let builder = wry::WebViewBuilder::new_with_web_context(&mut context)
        .with_bounds(webview_bounds(&window));
    let builder = if marketing_demo {
        builder.with_html(include_str!("../../../assets/marketing/demo.html"))
    } else {
        builder.with_url(config::APP_URL)
    };
    let view = builder
        .with_initialization_script(INIT_JS)
        // Zoom shortcuts go through app state so presets and settings stay synced.
        .with_hotkeys_zoom(false)
        .with_navigation_handler(|url| {
            if navigation_allowed(&url) {
                return true;
            }
            let _ = webbrowser::open(&url);
            false
        })
        .with_on_page_load_handler({
            let proxy = proxy.clone();
            move |ev, _url| {
                if matches!(ev, wry::PageLoadEvent::Finished) {
                    let _ = proxy.send_event(UserEvent::PageLoaded);
                }
            }
        })
        .with_ipc_handler({
            let proxy = proxy.clone();
            move |req: http::Request<String>| {
                let body = req.body();
                if let Some((count, baseline)) = parse_unread(body) {
                    let _ = proxy.send_event(UserEvent::Unread { count, baseline });
                } else if matches!(
                    body.as_str(),
                    "open-settings" | "zoom-in" | "zoom-out" | "zoom-reset"
                ) {
                    let _ = proxy.send_event(UserEvent::Ipc(body.clone()));
                }
            }
        })
        .build_as_child(&window)?;
    view.zoom(PRESETS[app.prefs.zoom_idx] / 100.0)?;
    // `context` must outlive the webview on some platforms; wry holds it.
    std::mem::forget(context);
    app.track_main(window, view);
    Ok(())
}

pub fn wire_menu_thread(proxy: &winit::event_loop::EventLoopProxy<UserEvent>) -> (Menu, MenuIds) {
    let (menu, ids) = build_menu().expect("menu");
    // Forward menu clicks (menubar and tray) into the event loop.
    let proxy = proxy.clone();
    std::thread::spawn(move || {
        while let Ok(ev) = muda::MenuEvent::receiver().recv() {
            if proxy.send_event(UserEvent::Menu(ev.id)).is_err() {
                break;
            }
        }
    });
    (menu, ids)
}

/// Tray clicks into the event loop. Windows only: left click toggles the
/// window. macOS/Linux open the tray menu on click instead.
pub fn wire_tray_events(proxy: &winit::event_loop::EventLoopProxy<UserEvent>) {
    if !cfg!(target_os = "windows") {
        return;
    }
    use tray_icon::{MouseButton, MouseButtonState, TrayIconEvent};
    let proxy = std::sync::Mutex::new(proxy.clone());
    TrayIconEvent::set_event_handler(Some(move |ev: TrayIconEvent| {
        if let TrayIconEvent::Click {
            button: MouseButton::Left,
            button_state: MouseButtonState::Up,
            ..
        } = ev
        {
            if let Ok(p) = proxy.lock() {
                let _ = p.send_event(UserEvent::TrayClick);
            }
        }
    }));
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn parses_ipc() {
        assert_eq!(parse_unread("unread:3"), Some((3, false)));
        assert_eq!(parse_unread("unread-baseline:3"), Some((3, true)));
        assert_eq!(parse_unread("unread:x"), None);
        assert_eq!(parse_unread("zoom:110%"), None);
        assert_eq!(parse_pref("pref:tray:1"), Some(("tray", true)));
        assert_eq!(
            parse_pref("pref:keep_running:0"),
            Some(("keep_running", false))
        );
        assert_eq!(parse_pref("pref:tray:2"), None);
        assert_eq!(parse_pref("zoom-in"), None);
    }

    #[test]
    fn notification_sound_only_follows_new_unread_count() {
        let on = NotificationSound::SystemNotification;
        assert!(!should_play_sound(0, 3, true, on));
        assert!(should_play_sound(0, 1, false, on));
        assert!(!should_play_sound(1, 1, false, on));
        assert!(!should_play_sound(2, 1, false, on));
        assert!(!should_play_sound(0, 1, false, NotificationSound::Off));
    }
}
