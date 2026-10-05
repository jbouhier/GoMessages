# Developer docs

GoMessages puts Google Messages on Mac, Linux, and Windows as a real app. A
thin Rust binary owns the native window, menu, and settings. The real
Messages page runs embedded inside it and owns chat, auth, and pairing.
There is no public Messages API, so a wrapper is the whole strategy. Details
in [ARCHITECTURE](ARCHITECTURE.md).

## Requirements

- Rust stable: `rustup` from [rustup.rs](https://rustup.rs) (all three OSes)
- `just` for the short commands below (`brew install just`, `cargo install just`, or `winget install just`)
- macOS: `xcode-select --install`
- Linux: `libwebkit2gtk-4.1-dev build-essential curl wget file libssl-dev libgtk-3-dev libayatana-appindicator3-dev librsvg2-dev`
- Windows: nothing extra, WebView2 ships with the OS
- Bun, only for the docs site in `site/`

## Run it

```sh
just -l        # list all commands
just run       # debug build
just lint      # fmt + clippy + tests (alias: just test)
just build     # release binary
just bundle    # macOS .app in dist/ (runs build first)
just install   # build + bundle + copy .app to /Applications for testing (macOS)
just clean     # remove target/ and dist/
```

Release (version comes from `Cargo.toml`, `bundle.sh` reads it back):

```sh
just bump 0.2.0       # Cargo.toml + Cargo.lock + fmt check
just release 0.2.0    # bump + commit + tag + push (triggers Release workflow)
just release-status   # git sync + gh run list
```

Install the app: macOS runs `just install` (builds release, bundles the
`.app` into `dist/`, copies it into `/Applications`). Windows and Linux run
the release binary straight from `target/release/` (packaged into
zip/tarball by CI on tagged releases).

Docs site:

```sh
just site-install  # one time
just site-dev      # preview at localhost:4321
just site-build    # static build
just screenshot  # staged Mac app image for the site
```

## Read next

1. [ARCHITECTURE](ARCHITECTURE.md), why it is built this way.
2. [CODEBASE](CODEBASE.md), file map and conventions.
3. [CONTRIBUTING](CONTRIBUTING.md), how to land a change.
4. [SCREENSHOT](SCREENSHOT.md), how to refresh the product image.
