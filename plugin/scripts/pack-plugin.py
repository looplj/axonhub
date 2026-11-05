#!/usr/bin/env python3
"""Pack a plugin directory into a .axplugin ZIP.

The archive contains manifest.json and the files under ui/.
A root README.md and .DS_Store files are left out. Any other path is rejected.
Limits match internal/server/plugin/package.go.
"""

from __future__ import annotations

import argparse
import json
import sys
import zipfile
from pathlib import Path

# Keep in sync with internal/server/plugin/package.go.
MAX_PACKAGE_BYTES = 5 << 20
MAX_UNCOMPRESSED_BYTES = 20 << 20
MAX_PACKAGE_FILES = 200
MAX_PACKAGE_FILE_BYTES = 5 << 20


def fail(message: str) -> None:
    print(f"Error: {message}", file=sys.stderr)
    raise SystemExit(1)


def archive_name(root: Path, path: Path) -> str:
    relative = path.relative_to(root).as_posix()
    if relative.startswith("/") or ".." in relative.split("/"):
        fail(f"file path {relative!r} escapes the archive")
    return relative


def iter_files(root: Path) -> list[Path]:
    files: list[Path] = []
    for path in sorted(root.rglob("*")):
        if path.is_symlink():
            fail(f"file {archive_name(root, path)!r} is a symlink")
        if not path.is_file():
            continue
        files.append(path)
    return files


def select_files(root: Path, files: list[Path]) -> list[tuple[str, Path]]:
    selected: list[tuple[str, Path]] = []
    for path in files:
        name = archive_name(root, path)
        if name == "plugin.js" or name.endswith("/plugin.js"):
            fail("JS plugins are not enabled yet")
        if path.name == ".DS_Store" or name == "README.md":
            continue
        if name == "manifest.json" or name.startswith("ui/"):
            selected.append((name, path))
            continue
        fail(f"file {name!r} is not allowed in a plugin package")
    return selected


def check_manifest(selected: list[tuple[str, Path]]) -> None:
    names = {name for name, _ in selected}
    manifest_path = next((path for name, path in selected if name == "manifest.json"), None)
    if manifest_path is None:
        fail("manifest.json is required")
    try:
        manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
    except (UnicodeError, json.JSONDecodeError) as exc:
        fail(f"invalid plugin manifest: {exc}")
    if not isinstance(manifest, dict):
        fail("invalid plugin manifest: manifest.json must be an object")

    ui = manifest.get("ui")
    if ui is None:
        return
    if not isinstance(ui, dict):
        fail("invalid plugin manifest: ui must be an object")
    pages = ui.get("pages") or []
    if not isinstance(pages, list):
        fail("invalid plugin manifest: ui.pages must be an array")
    for page in pages:
        if not isinstance(page, dict):
            fail("invalid plugin manifest: each ui page must be an object")
        entry = page.get("entry")
        if not isinstance(entry, str) or entry not in names:
            shown = entry if isinstance(entry, str) else ""
            fail(f"ui page entry {shown!r} does not exist in the package")


def check_sizes(selected: list[tuple[str, Path]]) -> None:
    if len(selected) > MAX_PACKAGE_FILES:
        fail(f"plugin package exceeds {MAX_PACKAGE_FILES} files")
    total = 0
    for name, path in selected:
        size = path.stat().st_size
        if size > MAX_PACKAGE_FILE_BYTES:
            fail(f"file {name!r} exceeds 5 MiB")
        total += size
        if total > MAX_UNCOMPRESSED_BYTES:
            fail("plugin package uncompressed size exceeds 20 MiB")


def write_archive(selected: list[tuple[str, Path]], output: Path) -> int:
    output.parent.mkdir(parents=True, exist_ok=True)
    temporary = output.with_suffix(output.suffix + ".tmp")
    try:
        with zipfile.ZipFile(temporary, "w", compression=zipfile.ZIP_DEFLATED) as archive:
            for name, path in selected:
                archive.write(path, name)
        size = temporary.stat().st_size
        if size > MAX_PACKAGE_BYTES:
            fail("plugin package exceeds 5 MiB")
        temporary.replace(output)
    except BaseException:
        temporary.unlink(missing_ok=True)
        raise
    return size


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Pack a plugin directory into a .axplugin ZIP.",
    )
    parser.add_argument(
        "plugin_dir",
        type=Path,
        help="plugin root containing manifest.json and ui/",
    )
    parser.add_argument(
        "-o",
        "--output",
        type=Path,
        help="output path (default: <parent>/<directory-name>.axplugin)",
    )
    return parser.parse_args()


def main() -> None:
    args = parse_args()
    root = args.plugin_dir.expanduser().resolve()
    if not root.is_dir():
        fail(f"plugin directory not found: {root}")

    output = args.output.expanduser() if args.output else root.parent / f"{root.name}.axplugin"
    if not output.is_absolute():
        output = Path.cwd() / output
    output = output.resolve()
    if output == root or root in output.parents:
        fail("output must be outside the plugin directory")
    if output.exists() and output.is_dir():
        fail(f"output is a directory: {output}")

    selected = select_files(root, iter_files(root))
    check_manifest(selected)
    check_sizes(selected)
    size = write_archive(selected, output)
    print(f"{output} ({len(selected)} files, {size} bytes)")


if __name__ == "__main__":
    main()
