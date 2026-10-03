
```
src/bin/gomessages/     product binary: main (loop), app (windows+IPC),
                        menu, settings page, webview (init JS + bounds),
                        prefs (settings.json), tray, platform (per-OS glue)
src/lib.rs              native-track library root, `native` feature-gated
src/config.rs           globals: name, URL, paths, window sizes (edit here)
src/paths.rs            data dir (single triple, no history)
src/ui.rs               egui panels (`native` only)
src/store.rs            local chat.json store: outbox, read state
src/auth.rs             Google OAuth desktop flow (PKCE loopback, refresh)
src/backend.rs          SyncBackend seam + MockBackend sample data
scripts/bundle.sh       macOS .app bundle + icon + Info.plist
site/                   docs site (Astro Starlight, published copy of these docs)
.github/workflows/     ci / release / docs
```

`auth`, `backend`, `store`, `ui` are behind `--features native` (default
builds skip them, CI lints both). Product code never depends on them.

## `src/bin/gomessages/`

Split by job: `main` (loop + `Bootstrap`), `app` (windows, settings IPC,
background mode, unread), `menu` (menubar), `settings` (local HTML page),
`webview` (init script, bounds math), `prefs` (`settings.json`), `tray`
(tray / menu bar icon), `platform` (Dock + reopen on macOS, GTK on Linux).
Tray icons are raw RGBA in `assets/tray/`.

## `src/auth.rs` (`native` feature)

Real OAuth 2.0 for installed apps: random-port loopback listener, PKCE
(`S256`, RFC 7636 test vector in unit tests), code exchange, silent refresh
on startup, `0600` token file. Scope is `openid email profile`
(`AUTH_SCOPE`); the Messages scope is still unknown.

Setup lives in [README](README.md). Landing a change lives in
[CONTRIBUTING](CONTRIBUTING.md).
