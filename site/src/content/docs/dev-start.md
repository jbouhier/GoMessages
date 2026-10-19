---
title: For developers
description: Build GoMessages and find your way around the code.
---

GoMessages is a Go backend built on
[libgm](https://github.com/mautrix/gmessages) and a Svelte UI, in one
[Wails](https://v3.wails.io) app. It's open source under the AGPL-3.0.

The developer documentation lives with the code, in
[`docs/`](https://github.com/jbouhier/GoMessages/tree/main/docs):

- [Getting set up](https://github.com/jbouhier/GoMessages/blob/main/docs/README.md):
  requirements and commands
- [Architecture](https://github.com/jbouhier/GoMessages/blob/main/docs/ARCHITECTURE.md)
- [Codebase map](https://github.com/jbouhier/GoMessages/blob/main/docs/CODEBASE.md)
- [Contributing](https://github.com/jbouhier/GoMessages/blob/main/docs/CONTRIBUTING.md)
- [Releasing](https://github.com/jbouhier/GoMessages/blob/main/docs/RELEASING.md)
- [Theming](https://github.com/jbouhier/GoMessages/blob/main/docs/THEMING.md)

The fast loop:

```sh
just demo   # run on sample data, no phone needed
just lint
just test
```
