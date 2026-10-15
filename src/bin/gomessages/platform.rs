//! Per-OS glue that winit/wry don't cover.
//!
//! - macOS: Dock icon on/off, Dock badge, Dock-click reopen, Notification
//!   Center banners, App Nap opt-out.
//! - Linux: GTK init + pumping (wry webviews and the tray both need it).
//! - Windows: nothing; the tray icon carries the unread state.

use super::UserEvent;

pub type Proxy = winit::event_loop::EventLoopProxy<UserEvent>;

/// Can this build show native banners, and did the user allow them?
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
#[cfg_attr(not(target_os = "macos"), allow(dead_code))]
pub enum BannerStatus {
    /// No banner backend: not macOS, or a bare binary outside a `.app`.
    Unsupported,
    /// Not asked yet, or the answer hasn't arrived.
    Unknown,
    Allowed,
    /// Off in System Settings > Notifications.
    Denied,
}

impl BannerStatus {
    pub fn as_str(self) -> &'static str {
        match self {
            Self::Unsupported => "unsupported",
            Self::Unknown => "unknown",
            Self::Allowed => "allowed",
            Self::Denied => "denied",
        }
    }
}

#[cfg(target_os = "macos")]
mod imp {
    use std::cell::RefCell;
    use std::sync::atomic::{AtomicU8, Ordering};
    use std::sync::{Mutex, OnceLock};

    use block2::RcBlock;
    use objc2::rc::Retained;
    use objc2::runtime::{
        AnyClass, AnyObject, Bool, Imp, NSObject, NSObjectProtocol, ProtocolObject, Sel,
    };
    use objc2::{define_class, msg_send, sel, AnyThread, MainThreadMarker};
    use objc2_app_kit::{NSApplication, NSApplicationActivationPolicy, NSSound};
    use objc2_foundation::{NSActivityOptions, NSBundle, NSError, NSProcessInfo, NSString};
    use objc2_user_notifications::{
        UNAuthorizationOptions, UNMutableNotificationContent, UNNotification,
        UNNotificationPresentationOptions, UNNotificationRequest, UNNotificationResponse,
        UNUserNotificationCenter, UNUserNotificationCenterDelegate,
    };

    use super::{BannerStatus, Proxy, UserEvent};

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

    const NOTIFICATION_SOUND: &str = "/System/Library/PrivateFrameworks/ToneLibrary.framework/Versions/A/Resources/AlertTones/EncoreInfinitum/Rebound-EncoreInfinitum.caf";
    const MESSAGE_SOUND: &str = "/System/Library/PrivateFrameworks/ToneLibrary.framework/Versions/A/Resources/AlertTones/ReceivedMessage.caf";

    thread_local! {
        static SYSTEM_SOUND: RefCell<Option<Retained<NSSound>>> = const { RefCell::new(None) };
    }

    fn installed_notification_sound() -> Option<Retained<NSSound>> {
        for path in [NOTIFICATION_SOUND, MESSAGE_SOUND] {
            let path = NSString::from_str(path);
            if let Some(sound) =
                NSSound::initWithContentsOfFile_byReference(NSSound::alloc(), &path, true)
            {
                return Some(sound);
            }
        }
        NSSound::soundNamed(&NSString::from_str("Glass"))
    }

    /// Keep the NSSound alive while its asynchronous playback finishes.
    pub fn play_system_notification() {
        SYSTEM_SOUND.with(|slot| {
            let mut slot = slot.borrow_mut();
            if slot.is_none() {
                *slot = installed_notification_sound();
            }
            if let Some(sound) = slot.as_ref() {
                let _ = sound.stop();
                let _ = sound.play();
            }
        });
    }

    /// Opt out of App Nap. A hidden window otherwise gets its timers and
    /// network callbacks throttled, and new messages arrive minutes late.
    pub fn keep_awake() {
        let reason = NSString::from_str("Receiving messages in the background");
        let token = NSProcessInfo::processInfo().beginActivityWithOptions_reason(
            NSActivityOptions::UserInitiatedAllowingIdleSystemSleep,
            &reason,
        );
        // Held for the whole process life; ending it would re-enable App Nap.
        std::mem::forget(token);
    }

    // ---- Notification Center banners ----

    const STATUS_UNKNOWN: u8 = 0;
    const STATUS_ALLOWED: u8 = 1;
    const STATUS_DENIED: u8 = 2;
    static BANNER_STATUS: AtomicU8 = AtomicU8::new(STATUS_UNKNOWN);
    static NOTIFY_PROXY: OnceLock<Mutex<Proxy>> = OnceLock::new();

    thread_local! {
        static NOTIFY_DELEGATE: RefCell<Option<Retained<NotificationDelegate>>> = const { RefCell::new(None) };
    }

    define_class!(
        // Delegate calls can land on any thread; it only forwards to the
        // event loop proxy.
        #[unsafe(super(NSObject))]
        #[name = "GoMessagesNotificationDelegate"]
        struct NotificationDelegate;

        unsafe impl NSObjectProtocol for NotificationDelegate {}

        unsafe impl UNUserNotificationCenterDelegate for NotificationDelegate {
            /// Show the banner even when the app is active (the page only
            /// notifies when its window is in the background anyway).
            #[unsafe(method(userNotificationCenter:willPresentNotification:withCompletionHandler:))]
            fn will_present(
                &self,
                _center: &UNUserNotificationCenter,
                _notification: &UNNotification,
                handler: &block2::DynBlock<dyn Fn(UNNotificationPresentationOptions)>,
            ) {
                handler.call((UNNotificationPresentationOptions::Banner
                    | UNNotificationPresentationOptions::List,));
            }

            /// Banner clicked: reopen the window on that conversation.
            #[unsafe(method(userNotificationCenter:didReceiveNotificationResponse:withCompletionHandler:))]
            fn did_receive(
                &self,
                _center: &UNUserNotificationCenter,
                response: &UNNotificationResponse,
                handler: &block2::DynBlock<dyn Fn()>,
            ) {
                let id = response.notification().request().identifier().to_string();
                if let Some(proxy) = NOTIFY_PROXY.get().and_then(|p| p.lock().ok()) {
                    let _ = proxy.send_event(UserEvent::NotificationClicked(id));
                }
                handler.call(());
            }
        }
    );

    impl NotificationDelegate {
        fn new() -> Retained<Self> {
            let this = Self::alloc().set_ivars(());
            // SAFETY: plain NSObject init on a freshly allocated object.
            unsafe { msg_send![super(this), init] }
        }
    }

    /// `UNUserNotificationCenter` raises (and aborts) in a process without a
    /// bundle identifier, e.g. `cargo run`. Banners need the bundled `.app`.
    fn banners_possible() -> bool {
        NSBundle::mainBundle().bundleIdentifier().is_some()
    }

    pub fn banner_status() -> BannerStatus {
        if !banners_possible() {
            return BannerStatus::Unsupported;
        }
        match BANNER_STATUS.load(Ordering::Relaxed) {
            STATUS_ALLOWED => BannerStatus::Allowed,
            STATUS_DENIED => BannerStatus::Denied,
            _ => BannerStatus::Unknown,
        }
    }

    /// Install the click delegate. Call once, after the event loop started.
    pub fn init_notifications(proxy: Proxy) {
        if !banners_possible() {
            return;
        }
        let _ = NOTIFY_PROXY.set(Mutex::new(proxy));
        let delegate = NotificationDelegate::new();
        UNUserNotificationCenter::currentNotificationCenter()
            .setDelegate(Some(ProtocolObject::from_ref(&*delegate)));
        // The center holds it weakly; keep it alive here.
        NOTIFY_DELEGATE.with(|slot| *slot.borrow_mut() = Some(delegate));
    }

    /// First call shows the macOS "Allow notifications?" prompt; later calls
    /// just report the saved answer. `done` fires on an arbitrary thread.
    pub fn request_banner_permission(done: impl Fn() + Send + 'static) {
        if !banners_possible() {
            return;
        }
        let block = RcBlock::new(move |granted: Bool, _err: *mut NSError| {
            let status = if granted.as_bool() {
                STATUS_ALLOWED
            } else {
                STATUS_DENIED
            };
            BANNER_STATUS.store(status, Ordering::Relaxed);
            done();
        });
        UNUserNotificationCenter::currentNotificationCenter()
            .requestAuthorizationWithOptions_completionHandler(
                UNAuthorizationOptions::Alert | UNAuthorizationOptions::Badge,
                &block,
            );
    }

    /// Silent banner: the app plays its own sound so the two settings stay
    /// independent. `thread` groups banners per conversation.
    pub fn post_banner(id: &str, title: &str, body: &str, thread: &str) {
        if !banners_possible() {
            return;
        }
        let content = UNMutableNotificationContent::new();
        content.setTitle(&NSString::from_str(title));
        content.setBody(&NSString::from_str(body));
        if !thread.is_empty() {
            content.setThreadIdentifier(&NSString::from_str(thread));
        }
        let request = UNNotificationRequest::requestWithIdentifier_content_trigger(
            &NSString::from_str(id),
            &content,
            None,
        );
        UNUserNotificationCenter::currentNotificationCenter()
            .addNotificationRequest_withCompletionHandler(&request, None);
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
    use super::{BannerStatus, Proxy};

    pub fn keep_awake() {}
    pub fn banner_status() -> BannerStatus {
        BannerStatus::Unsupported
    }
    pub fn init_notifications(_proxy: Proxy) {}
    pub fn request_banner_permission(_done: impl Fn() + Send + 'static) {}
    pub fn post_banner(_id: &str, _title: &str, _body: &str, _thread: &str) {}
    pub fn activate() {}
    pub fn set_dock_visible(_visible: bool) {}
    pub fn set_badge(_unread: u32) {}
    pub fn install_reopen_handler(_proxy: Proxy) {}
}

pub use imp::*;

#[cfg(target_os = "linux")]
pub fn play_system_notification() {
    gtk::gdk::beep();
}

#[cfg(target_os = "windows")]
pub fn play_system_notification() {
    #[link(name = "user32")]
    unsafe extern "system" {
        fn MessageBeep(kind: u32) -> i32;
    }
    // 0 uses the user's default system sound.
    unsafe { MessageBeep(0) };
}

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

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn banner_status_strings_match_the_settings_page() {
        // settings.rs JS compares against these literals.
        assert_eq!(BannerStatus::Unsupported.as_str(), "unsupported");
        assert_eq!(BannerStatus::Unknown.as_str(), "unknown");
        assert_eq!(BannerStatus::Allowed.as_str(), "allowed");
        assert_eq!(BannerStatus::Denied.as_str(), "denied");
    }

    /// Test binaries aren't bundled, so no platform may claim banners here
    /// (macOS would otherwise abort inside UNUserNotificationCenter).
    #[test]
    fn unbundled_process_has_no_banners() {
        assert_eq!(banner_status(), BannerStatus::Unsupported);
        // Must be a safe no-op, not a crash.
        post_banner("1#1", "t", "b", "");
        request_banner_permission(|| panic!("no callback without a bundle"));
    }
}
