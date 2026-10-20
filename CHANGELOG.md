# Changelog

What changed in each GoMessages release. The section for a version is also
its release notes on GitHub.

## [0.4.2] - 2026-10-20

### Fixed

- When you send or receive a message, the conversation now scrolls all the
  way down, so the time and delivery checkmarks under it stay in view.

## [0.4.1] - 2026-10-20

### Fixed

- Letters no longer shift or jump while you type a message, for example
  after an apostrophe in "I'm".

## [0.4.0] - 2026-10-19

A new GoMessages. Instead of showing the Google Messages web page in a
window, GoMessages is now a full desktop app with its own interface. It
connects to your phone directly.

**You need to pair once more after updating.** Open the app, choose
**Sign in with Google**, and tap the matching emoji on your phone.

### New

- **A desktop interface.** Conversation list, archive, search, photos and
  files, reactions, replies, deleting messages, typing indicators, dual SIM,
  and starting new chats from your contacts.
- **Quick switcher.** Press `Cmd+K` and type a name to jump to any
  conversation.
- **Keyboard first.** A full menu bar shows every shortcut, and
  `Cmd+/` lists them. Every menu and confirmation works from the keyboard.
- **Message actions.** Hover a message to react, reply, copy, or delete.
  React with six quick emoji or any other one.
- **Conversation actions.** Archive, mute, mark as unread, block and report
  spam, or delete a conversation.
- **Themes.** Eight themes, each with a light and a dark side: Default,
  Graphite, Sky, Mauve, Aurora, Peach, Amber, and Sage. Choose System,
  Light, or Dark mode, a gradient or solid background, and how see-through
  the window is.
- **Notifications** with an optional message preview and sound, and
  per-conversation mute.
- **Chat settings.** Choose whether Enter sends, and whether to send
  typing indicators.
- **Sign-in on every platform.** Mac, Windows, and Linux all pair inside
  the app.

### Changed

- The unread count shows in the window title.
- Closing the window quits the app.
- Text size (`Cmd+=`, `Cmd+-`, `Cmd+0`) is one setting for all monitors.
- Unpair is in **Settings → Account**.
- Linux needs GTK 4 and WebKitGTK 6.0 (Ubuntu 24.04, Fedora 39, or newer).
- GoMessages is now licensed under the AGPL-3.0, because it uses
  [libgm](https://github.com/mautrix/gmessages) from mautrix-gmessages.

### Removed

These are not in this version yet:

- The menu bar and tray icon.
- Keeping the app running when the window is closed.
- The unread badge on the Dock icon, and the option to hide the Dock icon.

## [0.3.0] - 2026-10-05

- Last version that shows the Google Messages web page in a native window.
