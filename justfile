# GoMessages (Wails): Go backend + Svelte UI in one binary.

wails := "./bin/tools/wails3"

default:
    @just --list

# Build the UI and the binary (bin/gomessages)
build: ui
    go build -o bin/gomessages .

# Build the UI into frontend/dist
ui:
    cd frontend && npm install --silent && npx vite build --mode production

# Run on your account
run: build
    ./bin/gomessages

# Run on sample data, no phone needed
demo: build
    ./bin/gomessages --demo

# The UI in a browser at http://127.0.0.1:9250/?http (sample data)
serve: build
    ./bin/gomessages --demo --serve 127.0.0.1:9250

# Live-reloading development window
dev: tools
    PATH="$PWD/bin/tools:$PATH" {{wails}} dev

# Regenerate the TypeScript bindings after changing Go service methods
bindings: tools
    {{wails}} generate bindings -clean=true -ts

# The .app bundle in bin/
package: tools
    PATH="$PWD/bin/tools:$PATH" {{wails}} package

# Package and install to /Applications (macOS)
install:
    scripts/install.sh

# Go tests, UI unit tests, then the UI tests in headless Chrome against the demo backend
test: build unit
    go test ./...
    cd frontend && node --test e2e/

# UI unit tests (pure TS, no browser)
unit:
    cd frontend && node --test --experimental-strip-types test/*.test.ts

# Screenshots of every screen into DIR (light or dark)
tour dir scheme="light" theme="": build
    #!/usr/bin/env bash
    set -euo pipefail
    ./bin/gomessages --demo --serve 127.0.0.1:9250 >/dev/null 2>&1 &
    trap "kill $!" EXIT
    sleep 1
    cd frontend && node e2e/tour.mjs "{{dir}}" {{scheme}} {{theme}}

# gofmt, go vet, and svelte-check
lint: ui
    test -z "$(gofmt -l .)"
    go vet ./...
    cd frontend && npx svelte-check --threshold error

# The Wails CLI matching go.mod, in bin/tools
tools:
    #!/usr/bin/env bash
    set -euo pipefail
    [[ -x {{wails}} ]] && exit 0
    version="$(awk '$1 == "github.com/wailsapp/wails/v3" { print $2; exit }' go.mod)"
    GOBIN="$PWD/bin/tools" go install "github.com/wailsapp/wails/v3/cmd/wails3@$version"

# Product images for the website (site/public/product), from the demo
screenshot: build
    #!/usr/bin/env bash
    set -euo pipefail
    shots="$(mktemp -d)"
    just tour "$shots" light
    cp "$shots/a-maya-light.png" site/public/product/desktop.png
    cp "$shots/e-settings-light.png" site/public/product/settings.png
    rm -rf "$shots"

# Install the website's dependencies
site-install:
    cd site && bun install

# Preview the website at localhost:4321
site-dev:
    cd site && bun run dev

# Build the website
site-build:
    cd site && bun run build

# Set the app version everywhere (e.g. just bump 0.5.0)
bump version:
    scripts/bump.sh {{version}}

# Bump, commit, tag, and push: the Release workflow does the rest.
# Needs a CHANGELOG.md section for the version first.
release version:
    scripts/release-notes.sh {{version}} >/dev/null
    just bump {{version}}
    git add build CHANGELOG.md
    git commit -m "release: v{{version}}"
    git tag "v{{version}}"
    git push origin main
    git push origin "v{{version}}"

# Git sync and recent CI runs
release-status:
    git status -sb
    gh run list --limit 10

# Remove build output
clean:
    rm -rf bin/gomessages bin/*.app frontend/dist
