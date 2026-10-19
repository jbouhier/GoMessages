---
title: Troubleshooting
description: Fix common GoMessages problems.
---

## Sign-in window says the browser isn't supported

Google sometimes blocks sign-in from apps it doesn't recognize. Close the
window, update your system (on Linux, WebKitGTK; on Windows, Microsoft Edge
WebView2), and try again. If it keeps happening, please
[open an issue](https://github.com/jbouhier/GoMessages/issues) with your
system and version.

## The emoji doesn't show on my phone

Open Google Messages on your phone and make sure it's online. Cancel and
choose **Sign in with Google** again.

## "Phone offline"

Messages sync through your phone. Check that it's on, connected, and that
Google Messages isn't restricted by battery saving.

## Signed out after a few weeks

Google ends pairings it doesn't see used. Pair again. Nothing is broken.

## Start fresh

**Settings → Account → Unpair…**, then sign in again.

## Mac says the app can't be opened

Right-click GoMessages in Applications, choose **Open**, then **Open**. If
macOS says the app "is damaged", run:

```sh
xattr -cr /Applications/GoMessages.app
```

## No notifications

Check **Settings → Notifications**, that the conversation isn't muted, and
that your system allows notifications from GoMessages.


## Linux: the app doesn't start

Install GTK 4 and WebKitGTK 6.0 (see
[getting started](/GoMessages/getting-started/#linux-requirements)).
