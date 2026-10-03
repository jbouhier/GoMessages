//! Per-OS glue that winit/wry don't cover.
//!
//! - macOS: Dock icon on/off, Dock badge, Dock-click reopen.
//! - Linux: GTK init + pumping (wry webviews and the tray both need it).
//! - Windows: nothing; the tray icon carries the unread state.

use super::UserEvent;

pub type Proxy = winit::event_loop::EventLoopProxy<UserEvent>;

#[cfg(target_os = "macos")]
mod imp {
    use std::sync::{Mutex, OnceLock};

    use objc2::rc::Retained;
    use objc2::runtime::{AnyClass, AnyObject, Bool, Imp, Sel};
    use objc2::{sel, MainThreadMarker};
    use objc2_app_kit::{NSApplication, NSApplicationActivationPolicy};
    use objc2_foundation::NSString;

    use super::{Proxy, UserEvent};

    fn app() -> Option<Retained<NSApplication>> {
        MainThreadMarker::new().map(NSApplication::sharedApplication)
    }

    /// Bring the app to the front (needed when the Dock icon is hidden).
    pub fn activate() {
        if let Some(app) = app() {
            #[allow(deprecated)]
            app.activateIgnoringOtherApps(true);
        }
    }

    /// Show or hide the Dock icon (Regular vs Accessory policy).
    pub fn set_dock_visible(visible: bool) {
        let Some(app) = app() else { return };
        let policy = if visible {
            NSApplicationActivationPolicy::Regular
        } else {
            NSApplicationActivationPolicy::Accessory
        };
        app.setActivationPolicy(policy);
        // Flipping the policy can drop focus; take it back.
        activate();
    }

    /// Unread count on the Dock icon; 0 clears it.
    pub fn set_badge(unread: u32) {
        let Some(app) = app() else { return };
        let label = (unread > 0).then(|| NSString::from_str(&unread.to_string()));
        app.dockTile().setBadgeLabel(label.as_deref());
    }

    static REOPEN: OnceLock<Mutex<Proxy>> = OnceLock::new();

    extern "C-unwind" fn should_handle_reopen(
        _this: *mut AnyObject,
        _cmd: Sel,
        _app: *mut AnyObject,
        _has_visible_windows: Bool,
    ) -> Bool {
        if let Some(proxy) = REOPEN.get().and_then(|p| p.lock().ok()) {
            let _ = proxy.send_event(UserEvent::Reopen);
        }
        Bool::YES
    }

    /// Dock click (or relaunch) while running. winit 0.30 has no hook for it,
    /// so add `applicationShouldHandleReopen:hasVisibleWindows:` to its app
    /// delegate class. Call after the event loop started (delegate is set).
    pub fn install_reopen_handler(proxy: Proxy) {
        let _ = REOPEN.set(Mutex::new(proxy));
        let Some(app) = app() else { return };
        let Some(delegate) = app.delegate() else {
            return;
        };
        let obj: &AnyObject = (*delegate).as_ref();
        let cls = obj.class() as *const AnyClass as *mut AnyClass;
        type Reopen = extern "C-unwind" fn(*mut AnyObject, Sel, *mut AnyObject, Bool) -> Bool;
        let f: Reopen = should_handle_reopen;
        // SAFETY: the IMP matches the selector's signature (BOOL, id self,
        // SEL, id app, BOOL flag); the runtime calls it through that type.
        let imp = unsafe { std::mem::transmute::<Reopen, Imp>(f) };
        let types = if cfg!(target_arch = "aarch64") {
            c"B@:@B"
        } else {
            c"c@:@c"
        };
        unsafe {
            objc2::ffi::class_addMethod(
                cls,
                sel!(applicationShouldHandleReopen:hasVisibleWindows:),
                imp,
                types.as_ptr(),
            );
        }
        // AppKit caches which optional delegate methods exist; re-set it.
        app.setDelegate(Some(&delegate));
    }
}

#[cfg(not(target_os = "macos"))]
mod imp {
    use super::Proxy;

    pub fn activate() {}
    pub fn set_dock_visible(_visible: bool) {}
    pub fn set_badge(_unread: u32) {}
    pub fn install_reopen_handler(_proxy: Proxy) {}
}

pub use imp::*;

/// Linux: tray-icon panics (and later aborts) when no AppIndicator library
/// is installed, so check with the same names it tries before using it.
#[cfg(target_os = "linux")]
pub fn tray_supported() -> bool {
    const LIBS: [&str; 4] = [
        "libayatana-appindicator3.so.1",
        "libappindicator3.so.1",
        "libayatana-appindicator3.so",
        "libappindicator3.so",
    ];
    // SAFETY: loading a system GTK helper library; it runs no unusual init.
    LIBS.iter()
        .any(|name| unsafe { libloading::Library::new(name) }.is_ok())
}

#[cfg(not(target_os = "linux"))]
pub fn tray_supported() -> bool {
    true
}

/// Linux: GTK must be up before any webview or tray exists.
#[cfg(target_os = "linux")]
pub fn init_toolkit() -> bool {
    gtk::init().is_ok()
}

#[cfg(not(target_os = "linux"))]
pub fn init_toolkit() -> bool {
    true
}

/// Linux: drain pending GTK events. winit doesn't drive GTK's loop.
#[cfg(target_os = "linux")]
pub fn pump_toolkit() {
    while gtk::events_pending() {
        gtk::main_iteration_do(false);
    }
}

#[cfg(not(target_os = "linux"))]
pub fn pump_toolkit() {}
