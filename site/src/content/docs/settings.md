---
title: Settings
description: The GoMessages settings window.
---

Open with `Cmd+,` (Mac) or `Ctrl+,` (Windows, Linux), GoMessages → Settings…
in the Mac menu bar, or Settings… in the tray icon menu.

- **Page zoom**, `A−` / `A+` / `Reset` act on the main window immediately and
  show the current level. Remembered across restarts.
- **Show menu bar icon** (macOS) / **Show tray icon** (Windows, Linux), with
  Open, Settings… and Quit, plus a dot when you have unread messages. On by
  default on Windows. Linux needs AppIndicator support (GNOME: the
  AppIndicator extension).
- **Keep running when the window is closed**, on by default. Closing the
  window hides it so messages keep arriving. Reopen from the Dock (macOS) or
  the tray icon. `Cmd+Q` / Quit really quits. On Windows and Linux this needs
  the tray icon.
- **Hide Dock icon** (macOS), menu bar only. Needs the menu bar icon.
- **Forget this computer**, unpairs your phone on next launch (use this on
  shared machines, or to start pairing fresh).
- **Quit**, quits the app.

Unread count shows as a Dock badge (macOS), a number next to the menu bar
icon, and a dot on the tray icon.

Settings are saved in `settings.json` in the app data folder
(`~/Library/Application Support/dev.go-messages.go-messages/` on Mac).

Standard Edit (undo, copy, paste…), View (zoom) and Window menus are also provided so text
fields behave like any Mac app.
