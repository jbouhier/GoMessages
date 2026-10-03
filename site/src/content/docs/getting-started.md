---
title: Getting started
description: Install GoMessages and pair your phone.
---

1. Download the build for your system from the
   [downloads page](/GoMessages/downloads/) (or the
   [releases page](https://github.com/jbouhier/GoMessages/releases)).
2. Mac: copy `GoMessages.app` to `/Applications`. Windows: unzip and run the
   `.exe`. Linux: unpack and run the binary.
3. Sign in with Google, then tap the matching emoji on your phone.

That's it, the app stays signed in.

Closing the window keeps GoMessages running so messages keep arriving.
Reopen it from the Dock (Mac) or the tray icon, quit with `Cmd+Q` (`Ctrl+Q`
elsewhere). Change this in [Settings](/GoMessages/settings/).

## Linux requirements

The app uses your system's web engine. On Debian/Ubuntu:

```sh
sudo apt install libwebkit2gtk-4.1-0
# optional, for the tray icon:
sudo apt install libayatana-appindicator3-1
```

Run it in an X11 session. Wayland sessions aren't supported yet.

## Build from source

```sh
cargo build --release --bin gomessages
bash scripts/bundle.sh target/release/gomessages
# → dist/GoMessages.app
```
