set windows-shell := ["powershell.exe", "-NoLogo", "-Command"]

BIN := "target/release/gomessages"

default:
    @just --list

# fmt + clippy (warnings deny, product + native track) + tests
lint:
    cargo fmt --all -- --check
    cargo clippy --all-targets -- -D warnings
    cargo clippy --all-targets --features native -- -D warnings
    cargo test --locked
    cargo test --locked --features native

alias test := lint

# Release binary (all OSes)
build:
    cargo build --locked --release --bin gomessages

# macOS .app in dist/
[unix]
bundle: build
    bash scripts/bundle.sh {{BIN}}

# Bundle + copy to /Applications (macOS)
[unix]
install: bundle
    cp -R "dist/GoMessages.app" /Applications/

# Run debug build
run:
    cargo run --bin gomessages

# Install docs site deps
site-install:
    cd site && bun install

# Preview docs locally
site-dev:
    cd site && bun run dev

# Build docs site
site-build:
    cd site && bun run build

# Remove build output
[unix]
clean:
    cargo clean
    rm -rf dist/

# Bump Cargo.toml + Cargo.lock to VERSION (e.g. just bump 0.2.0)
bump version:
    python3 scripts/bump.py {{version}}
    cargo update -p gomessages
    cargo fmt --all -- --check

# Commit bump, tag, push (triggers Release workflow)
release version:
    just bump {{version}}
    git add Cargo.toml Cargo.lock
    git commit -m "release: v{{version}}"
    git tag "v{{version}}"
    git push origin main
    git push origin "v{{version}}"

# Show sync + release CI status
release-status:
    git status -sb
    gh run list --limit 10
