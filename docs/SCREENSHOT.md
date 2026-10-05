# Screenshot

Run this on an interactive Mac desktop:

```sh
just screenshot
```

This builds GoMessages, opens a temporary app profile with the synthetic inbox
in `assets/marketing/demo.html`, captures the native window, and writes
`site/public/product/desktop.png`. It then captures the app's real Settings
window to `site/public/product/settings.png`. The docs home and Settings pages
use those files. The temporary profile is removed when the command finishes.
No Google account or message history is loaded.

macOS may ask for Screen Recording access for your terminal on the first run.
Grant it, then run the command again. The capture uses a fixed 1280 × 800 app
window. Its pixel dimensions follow the display scale; use the same Mac/display
for identical exports.

To refresh the image, edit the demo inbox, run the command, inspect the PNG,
then commit the updated image with the fixture. To write a copy elsewhere:

```sh
bash scripts/screenshot.sh /absolute/path/to/output-directory
```

The fixture is a staged illustration of the Google Messages page. Update its
layout when the live page changes materially. The image caption on the site
identifies the conversation as illustrative.
