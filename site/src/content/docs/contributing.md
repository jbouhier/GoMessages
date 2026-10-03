---
title: Contributing
description: How to contribute to GoMessages.
---

1. Fork, branch, open a PR against `main`. Loop commands and the file map:
   [For developers](/GoMessages/dev-start/).
2. CI must stay green: fmt, clippy, tests, and debug builds on macOS,
   Windows, and Linux.
3. Touching user-visible behavior? Update the matching docs page in the same
   PR.
4. Keep the app thin: the page owns chat, the binary owns the frame.

Release process: [Releases](/GoMessages/releases/).
