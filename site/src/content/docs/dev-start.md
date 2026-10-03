---
title: For developers
description: Where to start in the GoMessages codebase.
---

New here? Ten minutes, then build:

1. [Architecture](/GoMessages/architecture/) — what the app is and why it is built this way.
2. [Codebase map](/GoMessages/codebase/) — setup, build, test, and where everything lives.
3. [Contributing](/GoMessages/contributing/) — how to land a change.

The fast loop, once set up:

```sh
just lint   # fmt + clippy + tests
just build  # release binary
just run    # debug build
```
