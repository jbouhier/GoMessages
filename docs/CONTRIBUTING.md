# Contributing

1. Fork, branch, open a PR against `main`. Commands for the loop live in
   [README](README.md); the map in [CODEBASE](CODEBASE.md).
2. CI must stay green: fmt, clippy, tests, and debug builds on macOS,
   Windows, and Linux.
3. Touching user-visible behavior? Update the matching docs page under
   `site/src/content/docs/` in the same PR.
4. Keep the app thin: the page owns chat, the binary owns the frame.
