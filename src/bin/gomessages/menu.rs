//! App menu: GoMessages / Edit / View / Window.

use gomessages::config;
use muda::accelerator::{Accelerator, Code, CMD_OR_CTRL};
use muda::{AboutMetadata, IsMenuItem, Menu, MenuId, MenuItem, PredefinedMenuItem, Submenu};

use config::{APP_NAME, VERSION};

/// Ids of custom items the app handles itself.
pub struct MenuIds {
    pub settings: MenuId,
    pub zoom_in: MenuId,
    pub zoom_out: MenuId,
    pub zoom_reset: MenuId,
}

pub fn build_menu() -> anyhow::Result<(Menu, MenuIds)> {
    let menu = Menu::new();
    let app_menu = Submenu::new(APP_NAME, true);
    let about = PredefinedMenuItem::about(
        Some("About GoMessages"),
        Some(AboutMetadata {
            version: Some(VERSION.into()),
            comments: Some(
                "Google Messages on your Mac, Linux, and Windows, as a real app. Unofficial project, not affiliated with Google.".into(),
            ),
            website: Some("https://github.com/jbouhier/GoMessages".into()),
            website_label: Some("GoMessages on GitHub".into()),
            copyright: Some("MIT".into()),
            credits: Some("Made by JB Bouhier".into()),
            ..Default::default()
        }),
    );
    let settings = MenuItem::new(
        "Settings…",
        true,
        Some(Accelerator::new(CMD_OR_CTRL, Code::Comma)),
    );
    let settings_id = settings.id().clone();
    let quit = PredefinedMenuItem::quit(Some("Quit GoMessages"));
    app_menu.append_items(&[
        &about as &dyn IsMenuItem,
        &PredefinedMenuItem::separator() as &dyn IsMenuItem,
        &settings as &dyn IsMenuItem,
        &PredefinedMenuItem::separator() as &dyn IsMenuItem,
        &quit as &dyn IsMenuItem,
    ])?;

    let edit = Submenu::new("Edit", true);
    edit.append_items(&[
        &PredefinedMenuItem::undo(None) as &dyn IsMenuItem,
        &PredefinedMenuItem::redo(None) as &dyn IsMenuItem,
        &PredefinedMenuItem::separator() as &dyn IsMenuItem,
        &PredefinedMenuItem::cut(None) as &dyn IsMenuItem,
        &PredefinedMenuItem::copy(None) as &dyn IsMenuItem,
        &PredefinedMenuItem::paste(None) as &dyn IsMenuItem,
        &PredefinedMenuItem::select_all(None) as &dyn IsMenuItem,
    ])?;

    // Zoom lives in the menu, not page JS: wry child webviews return NO from
    // performKeyEquivalent, so Cmd+key never reaches the page on macOS. Menu
    // key equivalents match by character ("=", "-", "0"), so AZERTY works too.
    let zoom_in = MenuItem::new(
        "Zoom In",
        true,
        Some(Accelerator::new(CMD_OR_CTRL, Code::Equal)),
    );
    let zoom_out = MenuItem::new(
        "Zoom Out",
        true,
        Some(Accelerator::new(CMD_OR_CTRL, Code::Minus)),
    );
    let zoom_reset = MenuItem::new(
        "Actual Size",
        true,
        Some(Accelerator::new(CMD_OR_CTRL, Code::Digit0)),
    );
    let view = Submenu::new("View", true);
    view.append_items(&[
        &zoom_reset as &dyn IsMenuItem,
        &zoom_in as &dyn IsMenuItem,
        &zoom_out as &dyn IsMenuItem,
    ])?;

    let window_menu = Submenu::new("Window", true);
    window_menu.append_items(&[
        &PredefinedMenuItem::minimize(None) as &dyn IsMenuItem,
        &PredefinedMenuItem::zoom(None) as &dyn IsMenuItem,
        &PredefinedMenuItem::separator() as &dyn IsMenuItem,
        &PredefinedMenuItem::close_window(Some("Close Window")) as &dyn IsMenuItem,
    ])?;

    menu.append_items(&[
        &app_menu as &dyn IsMenuItem,
        &edit as &dyn IsMenuItem,
        &view as &dyn IsMenuItem,
        &window_menu as &dyn IsMenuItem,
    ])?;
    #[cfg(target_os = "macos")]
    menu.init_for_nsapp();
    Ok((
        menu,
        MenuIds {
            settings: settings_id,
            zoom_in: zoom_in.id().clone(),
            zoom_out: zoom_out.id().clone(),
            zoom_reset: zoom_reset.id().clone(),
        },
    ))
}
