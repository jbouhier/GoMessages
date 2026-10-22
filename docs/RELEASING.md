# Releasing

The version lives in `app/build/config.yml` and the platform files generated
from it. The release notes live in `CHANGELOG.md`.

1. In `CHANGELOG.md`, rename `Unreleased` to the new version and date:
   `## [0.5.0] - 2026-11-02`. Write for people using the app, not for
   developers: what's new, what changed, what was removed.
2. Run:

   ```sh
   just release 0.5.0
   ```

   This checks that the changelog has a `0.5.0` section, runs `just bump
   0.5.0`, commits `release: v0.5.0`, tags `v0.5.0`, and pushes both.
3. The tag starts `.github/workflows/release.yml`:
   - The gate runs `just lint` and `just test` on macOS.
   - Each platform builds and uploads its file to the GitHub release.
   - The release notes are that version's `CHANGELOG.md` section
     (`app/scripts/release-notes.sh`).
4. Follow it with `just release-status`.

| Platform | File |
|---|---|
| Mac (Apple Silicon) | `GoMessages-macos-aarch64.dmg` |
| Windows (x64) | `GoMessages-windows-x86_64.zip` |
| Linux (x64) | `GoMessages-linux-x86_64.tar.gz` |

## macOS signing

Without secrets, the app is ad-hoc signed: on first launch people
right-click it, choose Open, then Open. To sign with a Developer ID and
notarize, set these repository secrets: `APPLE_CERTIFICATE` (base64 .p12),
`APPLE_CERTIFICATE_PASSWORD`, `APPLE_API_KEY` (base64 .p8),
`APPLE_API_KEY_ID`, and `APPLE_API_ISSUER`.

## Before tagging

Sign-in isn't covered by CI. Pair a real phone with a build from `main` on
macOS, Windows, and Linux.
