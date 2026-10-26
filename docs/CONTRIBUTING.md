# Contributing

1. Fork, branch, and open a pull request against `main`. The commands are in
   [README](README.md), the file map in [CODEBASE](CODEBASE.md).
2. CI must stay green: `just lint` and `just test` on macOS, plus builds and
   Go tests on Windows and Linux.
3. Changing something people see? Update the matching page in
   `site/src/content/docs/` and add a line to the `Unreleased` section of
   `CHANGELOG.md` in the same pull request (create the section if needed).
4. Changing a screen? Attach before and after shots from `just tour`.

## Tests

`just test` runs all of them:

- **Go tests** (`*_test.go`): settings parsing and migration, model
  conversion. Decisions live in plain functions so they can be tested
  without a window or a phone.
- **UI unit tests** (`app/frontend/test/`): pure TypeScript, Node's test runner.
- **UI tests** (`app/frontend/e2e/ui.test.mjs`): the real Go backend on demo
  data, served over HTTP, driven in headless Chrome. They cover layout at
  every size and zoom, keyboard use, and WCAG AA contrast for every theme.
  `?titlebar` shows the Windows and Linux title bar on any platform.

Sign-in can't run in CI because it needs a Google account and a phone. When
you change `service.go`'s sign-in or a `webviewauth_*.go` file, pair a real
phone on each platform you touched, and say which ones in the pull request.

New behavior: cover the normal case, the failure people will hit first, and
known edge cases.
