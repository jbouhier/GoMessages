#!/usr/bin/env python3
"""Bump [package] version in Cargo.toml. Usage: bump.py 0.2.0"""
import pathlib
import sys

def main() -> None:
    if len(sys.argv) != 2:
        print("usage: bump.py VERSION", file=sys.stderr)
        raise SystemExit(2)
    version = sys.argv[1]
    path = pathlib.Path("Cargo.toml")
    lines = path.read_text().splitlines(keepends=True)
    in_package = False
    done = False
    out: list[str] = []
    for line in lines:
        stripped = line.strip()
        if stripped.startswith("["):
            in_package = stripped == "[package]"
        if in_package and not done and stripped.startswith("version"):
            indent = line[: len(line) - len(line.lstrip())]
            out.append(f'{indent}version = "{version}"\n')
            done = True
        else:
            out.append(line)
    if not done:
        print("[package] version not found in Cargo.toml", file=sys.stderr)
        raise SystemExit(1)
    path.write_text("".join(out))
    print(f"Cargo.toml -> {version}")

if __name__ == "__main__":
    main()
