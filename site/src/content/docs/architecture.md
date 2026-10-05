---
title: Architecture
description: How GoMessages is built and why.
---

GoMessages is a thin native frame around the real Google Messages web
client. The page owns chat, auth, and pairing. The binary owns the window,
the menu, persistence, and polish.

## Why a wrapper, not a reimplementation

There is no public Messages API. Login, pairing (including the Bluetooth
proximity check), relay, and RCS live in Google's proprietary web client.
Every embedding attempt confirmed the same rule: **the engine decides what
works, the frame is just chrome.**

- Chrome `--app` host: pairing works (real Chromium + Bluetooth), frame is Chromium.
- Tauri spike: `WKWebView` has no Bluetooth API, pairing fails. Dead end.
- `egui` + `wry` hybrid: same `WKWebView`, same result, until Bluetooth was
  enabled on both ends, at which point pairing passed. Engine + proximity,
  not toolkit.

Every approach lived or died on the web engine and Bluetooth, never on the frame around it.

So: native Rust window, real page inside. No Electron (150 MB+, flagged UA),
no protocol reverse-engineering (breaks on every Google change).

## Runtime pieces

| Piece | Crate | Job |
|---|---|---|
| Window + event loop | `winit` 0.30 | native window, transparent titlebar (macOS) |
| Web engine | `wry` 0.57 (`WKWebView` on mac) | the Messages page, persistent `WebContext` profile |
| Menu bar | `muda` 0.21 | GoMessages / Edit / View / Window menus; `Cmd+,` and zoom `Cmd+=/-/0` accelerators |
| Zoom | `wry` page zoom | per-monitor level in `settings.json`, re-applied on page load |
| Sound | `NSSound` / OS alert API | optional system sound when unread count rises; preview in Settings |
| Tray / menu bar icon | `tray-icon` 0.26 | Open / Settings / Quit, unread dot; opt-in on Linux (AppIndicator) |
| Platform glue | `objc2` (macOS), `gtk` (Linux) | Dock icon on/off, Dock badge, Dock-click reopen; GTK init + pump |
| Settings window | second `wry` view | local HTML + `window.ipc.postMessage` bridge |

No JavaScript framework, no bundler in the app. The only frontend code is
two small scripts: the init script (keybinds, splash, unread poller) and the
settings page.

## Data on disk

All under the app data dir (`~/Library/Application Support/dev.go-messages.go-messages/`
on macOS):

- `settings.json`, per-monitor zoom and window geometry, background/tray/Dock choices.
  Migrated once from older `zoom.json` / `window.json`.
- `webview-profile/`, cookies, login, pairing.
- `UNPAIR_ON_NEXT_LAUNCH`, flag file; next start wipes the profile, then
  removes itself.

Paths and globals live in one place: `src/config.rs`.

## Native prototype (separate track)

`src/lib.rs` holds an `egui`-based native client shell: mock conversations,
local JSON store with send-outbox, and a real Google OAuth desktop flow
(PKCE loopback). It proves the native-UI direction but carries no traffic. Sync needs the undiscovered Messages scope and endpoints.

Next: [Codebase map](/GoMessages/codebase/) for setup, build, and the file map.
