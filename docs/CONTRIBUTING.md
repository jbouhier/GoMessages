# Contributing

1. Fork, branch, open a PR against `main`. Commands for the loop live in
   [README](README.md); the map in [CODEBASE](CODEBASE.md).
2. CI must stay green: fmt, clippy, tests, and debug builds on macOS,
   Windows, and Linux.
3. Touching user-visible behavior? Update the matching docs page under
   `site/src/content/docs/` in the same PR.
4. Keep the app thin: the page owns chat, the binary owns the frame.

## Tests

`just lint` runs everything: fmt, clippy, Rust tests, JS tests.

- **Rust unit tests** sit next to the code (`#[cfg(test)]`). Logic that
  decides something (alert plan, IPC allowlist, prefs parsing) lives in pure
  functions so it can be tested without windows.
- **JS tests** (`tests/js/`, Node built-in runner, no deps) run the exact
  `INIT_JS` and settings script strings from the Rust sources inside a fake
  browser (`harness.mjs`). Needs Node 20+.
- **Contract fixture** `tests/fixtures/notify_payloads.json`: the JS tests
  assert what the page shim posts, the Rust tests assert `parse_notify` reads
  it. Change one side, the other fails.
- New behavior: cover the happy path, the default failure, and known edge
  cases.
