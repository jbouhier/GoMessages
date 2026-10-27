# Developer docs

GoMessages is a Google Messages desktop client. It's one binary: a Go
backend that talks to your phone through
[libgm](https://github.com/mautrix/gmessages), and a Svelte UI, joined
by [Wails v3](https://v3.wails.io). Details in [ARCHITECTURE](ARCHITECTURE.md).

## Requirements

- Go 1.27.3+, Node.js 22+, and [just](https://just.systems)
- macOS: `xcode-select --install`
- Linux: `build-essential pkg-config libgtk-4-dev libwebkitgtk-6.0-dev`
- Windows: nothing extra. WebView2 ships with Windows 10 and 11.
- UI tests: Chrome for Testing (`cd app/frontend && bunx playwright-core install chromium`), or `$CHROME`
- Website: [Bun](https://bun.sh)

The Wails CLI matching `app/go.mod` is installed into `app/bin/tools` the first
time a recipe needs it.

## Run it

```sh
just           # list all recipes
just demo      # sample data, no phone needed
just run       # your account
just dev       # live-reloading window
just lint      # gofmt, go vet, svelte-check
just test      # Go tests, UI unit tests, UI tests in headless Chrome
just package   # bin/gomessages.app (macOS)
just install   # package and copy GoMessages.app to /Applications (macOS)
just clean
```

`just serve` runs the demo backend over HTTP at
`http://127.0.0.1:9250/?http`, so you can use the UI in a browser. The UI
tests and screenshots use it too.

After changing the exported methods of `Messages` (the Go service), run
`just bindings`.

## Screenshots

```sh
just tour /tmp/shots dark        # every screen, from the demo
just tour /tmp/shots light sky   # one theme
just screenshot                  # refresh the website's product images
```

## Website

The user guide at `site/` is [Starlight](https://starlight.astro.build),
deployed to GitHub Pages by `.github/workflows/docs.yml` on every push to
`main` that touches `site/`.

```sh
just site-install  # once
just site-dev      # preview at localhost:4321
just site-build
```

## Read next

1. [ARCHITECTURE](ARCHITECTURE.md): how the pieces fit, and why.
2. [CODEBASE](CODEBASE.md): file map and conventions.
3. [CONTRIBUTING](CONTRIBUTING.md): how to land a change.
4. [RELEASING](RELEASING.md): versions, the changelog, and the release workflow.
5. [THEMING](THEMING.md): appearance settings and how to make a theme.
