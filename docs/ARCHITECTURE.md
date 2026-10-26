# Architecture

GoMessages is one process: a Go backend that speaks the Google Messages
protocol, and a Svelte UI in the system web view. [Wails v3](https://v3.wails.io)
provides the window, the menu bar, the bridge between the two,
notifications, and dialogs.

```
┌──────────────── GoMessages ────────────────┐
│ Svelte UI (app/frontend/)                  │
│   ↑ events "gm"         ↓ Messages.Call(…) │
│ Wails service: Messages (service.go)       │
│   ↓ backend interface (bridge.go)          │
│ daemon.go (libgm)  or  demo.go             │
└───────────────┬────────────────────────────┘
                │ long-polling + RPC
       Google Messages relay ⇄ your phone
```

## Why a client, not a web wrapper

Up to 0.3.0, GoMessages showed Google's web page in a native window. The
page decided what was possible: no real shortcuts, unread state scraped
from the page, and pairing that depended on the web engine supporting
Bluetooth. From 0.4.0, the app uses
[libgm](https://github.com/mautrix/gmessages), the protocol library behind
mautrix-gmessages, and draws its own UI. libgm is AGPL-3.0, so GoMessages
is too.

## Backend

- `service.go`: `Messages`, the Wails service the UI calls. `Call(cmd, args)`
  runs one protocol command (`conversations`, `messages`, `send`, `react`,
  `download`, …). The other methods are desktop work: sign-in, file
  dialogs, notifications, appearance, zoom.
- `daemon.go`: the libgm client. It loads `session.json`, connects, keeps a
  conversation cache, and turns libgm events into UI events (`state`,
  `conversation`, `message`, `typing`, `pair_emoji`, …).
- `demo.go`: the same `backend` interface with sample conversations, for
  `--demo`, the UI tests, and screenshots.
- `linkpreview.go`: link previews. Go fetches the page itself (no cookies,
  no scripts), refuses any address on this computer or its network, and
  keeps the result for a week. The UI asks through `Call("link_preview")`.
- `model.go`: the JSON shapes the UI receives, converted from libgm's protobufs.
- `settings.go`, `settings_migrate.go`: `settings.json`, with the window's
  place tracked by Go and older settings converted on load.
- `server.go`: `--serve addr` exposes the same service over HTTP (`/rpc/{method}`)
  and server-sent events (`/events`), so the UI runs in a normal browser.

## Sign-in

Pairing uses a Google account and an emoji, not a QR code:

1. `SignIn()` opens Google's sign-in page in a second web view window.
2. When that window reaches `https://messages.google.com/web/config`, the
   app reads Google's session cookies (`SID`, `HSID`, `SSID`, `OSID`,
   `APISID`, `SAPISID`) from the window's cookie store and closes it.
3. `pair_google` hands the cookies to libgm. The phone shows a prompt, the
   app shows the emoji to tap, and libgm saves the result to `session.json`.

Each platform reads the URL and cookies from its own web engine, since
Wails has no cookie API:

| Platform | File | Page-load event | Cookie store |
|---|---|---|---|
| macOS | `webviewauth_darwin.go` | `Mac.WebViewDidCommitNavigation` | `WKHTTPCookieStore` (Objective-C via cgo) |
| Windows | `webviewauth_windows.go` | `Windows.WebViewNavigationCompleted` | WebView2 `ICoreWebView2CookieManager` (COM vtable calls) |
| Linux | `webviewauth_linux.go` | `Linux.WindowLoadCommitted` | WebKitGTK `WebKitCookieManager` (cgo; GTK 4 by default, GTK 3 with `-tags gtk3`) |

On Windows and Linux, the web view handle is a private field of Wails'
window. `webviewauth_native.go` reads it by name with `reflect`, so a Wails
upgrade that renames it makes sign-in fail with a message, not crash.
Check sign-in on both platforms after bumping Wails in `app/go.mod`.

## UI

Svelte 5 with runes, built by Vite into `app/frontend/dist`, which is embedded
in the binary. `app/frontend/src/lib/app.svelte.ts` holds the state: it calls
`Messages` through the generated bindings (`app/frontend/bindings/`) and
applies backend events. Themes are JSON files checked against
`theme.schema.json` (see [THEMING](THEMING.md)).

Shortcuts with `Cmd`/`Ctrl` live in the native menu (`buildMenu` in
`main.go`), which sends an `action` event to the UI. Keys without a
modifier are handled in the UI.

On Windows and Linux the window is frameless and has no menu bar
(`customTitleBar` in `window.go`). The same shortcuts are registered as
window key bindings instead, and the UI draws the title bar: a strip of
`--top-inset` at the top of each pane, draggable like the macOS one, with
`WindowControls` at the right.

Files dropped on the conversation pane (`data-file-drop-target`) reach Go
as `WindowFilesDropped`. Go keeps the regular files and sends them to the
UI as a `files_dropped` event; they join the message box like `Cmd+O`.
In `--serve`, the `DropFiles` method stands in for a drop, for the UI tests.

## Data on disk

All under the OS config folder, in `GoMessages/` (for example
`~/Library/Application Support/GoMessages/` on macOS,
`%AppData%\GoMessages\` on Windows, `~/.config/GoMessages/` on Linux).
The folder and files are readable by the owner only.

- `session.json`: the libgm pairing. Treat it like account credentials.
- `settings.json`: appearance, notifications, chats, zoom, window place.
- `links/`: cached link previews, one small JSON file per link. Their
  pictures and icons go in `media/` as `lp-*` files.
- `media/`: downloaded photos, files, and contact photos. The UI loads them
  through `/media/<file>`, which serves nothing outside this folder.
- `demo/`: the same layout for `--demo`.

Builds named "GoMessages Next" used `GoMessagesNext/`. It's moved to
`GoMessages/` on first launch. The 0.3.0 web wrapper's profile isn't
read, so updating from it means pairing again.
