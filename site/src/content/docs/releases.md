---
title: Releases
description: How GoMessages is released.
---

Release with `just` (version lives in `Cargo.toml`):

```sh
just release 0.2.0
```

That's `bump + commit + tag + push` in one step:

1. `just bump 0.2.0` — `scripts/bump.py` updates `Cargo.toml`, then
   `cargo update -p gomessages` syncs `Cargo.lock`, then `cargo fmt --check`.
2. Commits as `release: v0.2.0`, tags `v0.2.0`, pushes `main` + the tag.
3. Push a `v*` tag, the rest is automatic — the gate runs first (fmt,
   clippy on both feature tracks, tests). Then each OS builds and uploads
   its artifact to the GitHub release.

Check status:

```sh
just release-status
# git status -sb + gh run list
```

Artifacts per release:

- Mac (Apple Silicon): `GoMessages-macos-aarch64.dmg`
- Windows (x64): `GoMessages-windows-x86_64.zip`
- Linux (x64): `GoMessages-linux-x86_64.tar.gz`

Until Developer-ID signing is set up, macOS downloads need right-click →
Open on first launch (see [Troubleshooting](/GoMessages/troubleshooting/)).
