---
title: Troubleshooting
description: Fix common GoMessages problems.
---

## Pairing asks about Bluetooth

Turn Bluetooth on, on both computer and phone, and keep them close together.
Pairing verifies proximity.

## Logged out after a few weeks

Google expires inactive pairings automatically. Pair again. Nothing is broken.

## Start completely fresh

Settings → Unpair, relaunch the app, pair again. This wipes the local web
profile, including login state.

## Closing the window doesn't quit

By design: the app keeps running so messages keep arriving. Reopen it from the
Dock (Mac) or the tray icon. Quit with `Cmd+Q` (`Ctrl+Q` elsewhere) or Quit in
the tray menu. To quit on close, turn off **Keep running when the window is
closed** in [Settings](/GoMessages/settings/).

## Can't find the app after hiding the Dock icon (Mac)

With **Hide Dock icon** on, the app lives in the menu bar only. Click its icon
there → Open. Turning the menu bar icon off brings the Dock icon back.

## No unread badge

The count is read from the Messages page. If Google changes the page, the
badge can stop updating while messages still arrive. Please
[open an issue](https://github.com/jbouhier/GoMessages/issues).

## No tray icon on Linux

The tray needs AppIndicator support:

- Install `libayatana-appindicator3-1` (Debian/Ubuntu).
- On GNOME, also enable the AppIndicator extension.

Settings shows "No tray available on this desktop" when neither is present.
Without a tray, closing the window quits the app.

## Downloaded app won't open

Unsigned builds are blocked by Gatekeeper on first launch: right-click the
app → Open → Open. If macOS says the app "is damaged", strip the download
quarantine instead:

```sh
xattr -cr /Applications/GoMessages.app
```

Signed + notarized releases (once set up) open normally.
