//! Menu bar icon (macOS) / tray icon (Windows, Linux) with an unread dot.
//!
//! Menu: Open, Settings…, Quit. Its items go through muda's global menu
//! channel, same as the menubar, so [`super::app`] matches their ids.
//! Windows: left click toggles the window, right click opens the menu.
//! macOS/Linux: click opens the menu (platform convention / AppIndicator).

use gomessages::config::APP_NAME;
use muda::{IsMenuItem, Menu, MenuId, MenuItem, PredefinedMenuItem};
use tray_icon::{Icon, TrayIcon, TrayIconBuilder};

/// Raw RGBA, generated from the splash logo geometry. macOS gets a
/// monochrome template so the menu bar tints it for light/dark.
#[cfg(target_os = "macos")]
const ICONS: (&[u8], &[u8], u32) = (
    include_bytes!("../../../assets/tray/tray-mac.rgba"),
    include_bytes!("../../../assets/tray/tray-mac-unread.rgba"),
    44,
);
#[cfg(not(target_os = "macos"))]
const ICONS: (&[u8], &[u8], u32) = (
    include_bytes!("../../../assets/tray/tray.rgba"),
    include_bytes!("../../../assets/tray/tray-unread.rgba"),
    32,
);

fn icon(unread: bool) -> Option<Icon> {
    let (normal, dot, size) = ICONS;
    let rgba = if unread { dot } else { normal };
    Icon::from_rgba(rgba.to_vec(), size, size).ok()
}

pub struct Tray {
    icon: TrayIcon,
    // Must outlive the tray icon: dropping it removes the menu.
    _menu: Menu,
    pub open: MenuId,
    pub settings: MenuId,
    pub quit: MenuId,
    unread: u32,
}

impl Tray {
    /// `None` when the platform has no tray (e.g. Linux without AppIndicator).
    pub fn new(unread: u32) -> Option<Self> {
        let menu = Menu::new();
        let open = MenuItem::new(format!("Open {APP_NAME}"), true, None);
        let settings = MenuItem::new("Settings…", true, None);
        let quit = MenuItem::new(format!("Quit {APP_NAME}"), true, None);
        menu.append_items(&[
            &open as &dyn IsMenuItem,
            &settings as &dyn IsMenuItem,
            &PredefinedMenuItem::separator() as &dyn IsMenuItem,
            &quit as &dyn IsMenuItem,
        ])
        .ok()?;
        let builder = TrayIconBuilder::new().with_menu(Box::new(menu.clone()));
        #[cfg(target_os = "macos")]
        let builder = builder.with_icon_templated(icon(unread > 0)?);
        #[cfg(not(target_os = "macos"))]
        let builder = builder.with_icon(icon(unread > 0)?);
        let icon = builder
            .with_menu_on_left_click(!cfg!(target_os = "windows"))
            .with_tooltip(tooltip(unread))
            .build()
            .ok()?;
        let tray = Self {
            icon,
            _menu: menu,
            open: open.id().clone(),
            settings: settings.id().clone(),
            quit: quit.id().clone(),
            unread,
        };
        tray.set_title();
        Some(tray)
    }

    pub fn set_unread(&mut self, unread: u32) {
        if (unread > 0) != (self.unread > 0) {
            #[cfg(target_os = "macos")]
            let _ = self.icon.set_icon_templated(icon(unread > 0));
            #[cfg(not(target_os = "macos"))]
            let _ = self.icon.set_icon(icon(unread > 0));
        }
        self.unread = unread;
        let _ = self.icon.set_tooltip(Some(tooltip(unread)));
        self.set_title();
    }

    /// macOS: count next to the menu bar icon.
    fn set_title(&self) {
        if cfg!(target_os = "macos") {
            let title = (self.unread > 0).then(|| self.unread.to_string());
            self.icon.set_title(title);
        }
    }
}

fn tooltip(unread: u32) -> String {
    match unread {
        0 => APP_NAME.to_string(),
        1 => format!("{APP_NAME}: 1 unread"),
        n => format!("{APP_NAME}: {n} unread"),
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn icons_are_square_rgba() {
        let (normal, dot, size) = ICONS;
        let len = (size * size * 4) as usize;
        assert_eq!(normal.len(), len);
        assert_eq!(dot.len(), len);
        assert!(icon(false).is_some() && icon(true).is_some());
    }

    #[test]
    fn tooltip_text() {
        assert_eq!(tooltip(0), "GoMessages");
        assert_eq!(tooltip(3), "GoMessages: 3 unread");
    }
}
