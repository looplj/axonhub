#!/usr/bin/env python3
"""Write a per-OS download table into a GitHub Release.

The release asset list also holds updater archives, deltas, and checksums.
This table links only to the installers people download by hand. Run it after
those assets are uploaded; filenames come from the release, not a template.

  python3 scripts/release-download-notes.py --tag v1.2.3
  python3 scripts/release-download-notes.py --tag v1.2.3 --print --assets-file release.json
"""

from __future__ import annotations

import argparse
import html
import json
import re
import subprocess
import sys
import urllib.parse
from pathlib import Path

START = "<!-- axonhub-downloads -->"
END = "<!-- /axonhub-downloads -->"

# User-facing installers. Updater tar.gz, deltas, manifests, and checksums stay
# in the asset list and are omitted here.
DESKTOP = (
    (re.compile(r"^axonhub-desktop-.+-arm64\.dmg$"), "macOS", "arm64", "DMG", "Apple Silicon", "111111", "apple"),
    (re.compile(r"^axonhub-desktop-.+-amd64\.dmg$"), "macOS", "amd64", "DMG", "Intel", "4A5568", "apple"),
    (re.compile(r"^axonhub-desktop-setup-.+-amd64\.exe$"), "Windows", "amd64", "Setup", "x64", "0078D4", "windows"),
    (re.compile(r"^axonhub-desktop-setup-.+-arm64\.exe$"), "Windows", "arm64", "Setup", "ARM64", "0078D4", "windows"),
    (re.compile(r"^axonhub-desktop_.+_amd64\.deb$"), "Linux", "amd64", "Deb", "x64", "A81D33", "debian"),
    (re.compile(r"^axonhub-desktop_.+_arm64\.deb$"), "Linux", "arm64", "Deb", "ARM64", "A81D33", "debian"),
)
SERVER = (
    (re.compile(r"^axonhub_.+_windows_amd64\.zip$"), "Windows", "amd64", "zip", "x64", "2D3748", "windows"),
    (re.compile(r"^axonhub_.+_windows_arm64\.zip$"), "Windows", "arm64", "zip", "ARM64", "2D3748", "windows"),
    (re.compile(r"^axonhub_.+_darwin_arm64\.zip$"), "macOS", "arm64", "zip", "Apple Silicon", "111111", "apple"),
    (re.compile(r"^axonhub_.+_darwin_amd64\.zip$"), "macOS", "amd64", "zip", "Intel", "4A5568", "apple"),
    (re.compile(r"^axonhub_.+_linux_amd64\.zip$"), "Linux", "amd64", "zip", "x64", "2D3748", "linux"),
    (re.compile(r"^axonhub_.+_linux_arm64\.zip$"), "Linux", "arm64", "zip", "ARM64", "2D3748", "linux"),
    (re.compile(r"^axonhub_.+_linux_loong64\.zip$"), "Linux", "loong64", "zip", "LoongArch", "2D3748", "linux"),
    (re.compile(r"^axonhub_.+_freebsd_amd64\.zip$"), "FreeBSD", "amd64", "zip", "x64", "AB2B28", "freebsd"),
    (re.compile(r"^axonhub_.+_freebsd_arm64\.zip$"), "FreeBSD", "arm64", "zip", "ARM64", "AB2B28", "freebsd"),
)

OS_ORDER = ("Windows", "macOS", "Linux", "FreeBSD")
ARCH_ORDER = {"amd64": 0, "loong64": 2}
# Apple Silicon is the first macOS button, matching the usual download table.
MAC_ARCH_ORDER = {"arm64": 0, "amd64": 1}


def badge(label: str, message: str, color: str, logo: str, url: str) -> str:
    def shields(part: str) -> str:
        return part.replace("-", "--").replace(" ", "_")

    src = (
        f"https://img.shields.io/badge/{shields(label)}-{shields(message)}-{color}"
        f"?style=flat&logo={urllib.parse.quote(logo)}&logoColor=white"
    )
    alt = html.escape(f"{label} {message}", quote=True)
    return f'<a href="{html.escape(url, quote=True)}"><img alt="{alt}" src="{html.escape(src, quote=True)}"></a>'


def download_url(repo: str, tag: str, name: str) -> str:
    return (
        f"https://github.com/{repo}/releases/download/"
        f"{urllib.parse.quote(tag)}/{urllib.parse.quote(name)}"
    )


def classify(names: list[str], rules: tuple) -> dict[str, list[tuple]]:
    found: dict[str, list[tuple]] = {}
    for name in names:
        for pattern, system, arch, label, message, color, logo in rules:
            if pattern.match(name):
                found.setdefault(system, []).append((arch, label, message, color, logo, name))
                break
    for system, items in found.items():
        order = MAC_ARCH_ORDER if system == "macOS" else ARCH_ORDER
        items.sort(key=lambda item: (order.get(item[0], 1), item[5]))
    return found


def table(repo: str, tag: str, rows: dict[str, list[tuple]], extra: list[tuple[str, str]] | None = None) -> str:
    body = [
        "<table>",
        "<thead><tr><th>OS</th><th>Download</th></tr></thead>",
        "<tbody>",
    ]
    for system in OS_ORDER:
        items = rows.get(system)
        if not items:
            continue
        buttons = " ".join(
            badge(label, message, color, logo, download_url(repo, tag, name))
            for _arch, label, message, color, logo, name in items
        )
        body.append(f"<tr><td>{html.escape(system)}</td><td>{buttons}</td></tr>")
    for system, button in extra or ():
        body.append(f"<tr><td>{html.escape(system)}</td><td>{button}</td></tr>")
    body.append("</tbody>")
    body.append("</table>")
    return "\n".join(body)


def download_section(repo: str, tag: str, names: list[str]) -> str:
    owner, name = repo.split("/", 1)
    count = (
        f"https://img.shields.io/github/downloads/{owner}/{name}/"
        f"{urllib.parse.quote(tag)}/total?label={urllib.parse.quote('downloads@' + tag)}&color=brightgreen"
    )
    desktop = classify(names, DESKTOP)
    server = classify(names, SERVER)
    docker = badge(
        "Docker",
        "amd64 / arm64",
        "2496ED",
        "docker",
        f"https://hub.docker.com/r/{repo}",
    )
    brew = badge(
        "Homebrew",
        "macOS",
        "FBB040",
        "homebrew",
        "https://github.com/looplj/homebrew-axonhub",
    )
    shot = (
        f'<img alt="AxonHub Desktop" src="{html.escape(download_url(repo, tag, "axonhub-desktop.png"), quote=True)}" width="320">'
    )
    parts = [
        "## 按系统下载 | Download",
        "",
        "安装包按系统列在下面。`.tar.gz`、`.delta` 和校验文件在下方 Assets 里，给应用内更新用。",
        "",
        "新增桌面版本，支持自动更新",
        "",
        f"<p>{shot}</p>",
        "",
        f'<p align="right"><img alt="downloads@{html.escape(tag, quote=True)}" src="{html.escape(count, quote=True)}"></p>',
        "",
    ]
    if desktop:
        parts.extend(["### Desktop", "", table(repo, tag, desktop), ""])
    parts.extend(
        [
            "### Server",
            "",
            table(repo, tag, server, [("Docker", docker), ("Homebrew", brew)]),
            "",
        ]
    )
    return "\n".join(parts).rstrip() + "\n"


def splice(body: str, section: str) -> str:
    block = f"{START}\n{section}{END}\n"
    if START in body and END in body:
        pre, rest = body.split(START, 1)
        _, post = rest.split(END, 1)
        post = post.lstrip("\n")
        if pre.strip():
            return pre.rstrip() + "\n\n" + block + "\n" + post
        return block + "\n" + post
    body = body.strip()
    if not body:
        return block
    return block + "\n" + body + "\n"


def release_view(repo: str, tag: str) -> dict:
    out = subprocess.check_output(
        ["gh", "release", "view", tag, "--repo", repo, "--json", "body,assets,tagName"],
        text=True,
    )
    return json.loads(out)


def release_edit(repo: str, tag: str, notes: str) -> None:
    subprocess.run(
        ["gh", "release", "edit", tag, "--repo", repo, "--notes-file", "-"],
        input=notes,
        text=True,
        check=True,
    )


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--tag", required=True)
    parser.add_argument("--repo", default="looplj/axonhub")
    parser.add_argument("--assets-file", type=Path, help="Release JSON, for a dry run without gh")
    parser.add_argument("--print", action="store_true", dest="print_notes", help="Write the notes to stdout")
    args = parser.parse_args()
    if args.repo.count("/") != 1:
        print(f"--repo must be owner/name, got {args.repo}", file=sys.stderr)
        return 2

    if args.assets_file:
        data = json.loads(args.assets_file.read_text())
        names = [item["name"] for item in data.get("assets", data if isinstance(data, list) else [])]
        body = data.get("body") or ""
    else:
        data = release_view(args.repo, args.tag)
        names = [item["name"] for item in data["assets"]]
        body = data.get("body") or ""

    notes = splice(body, download_section(args.repo, args.tag, names))
    if args.print_notes or args.assets_file:
        sys.stdout.write(notes)
        return 0
    release_edit(args.repo, args.tag, notes)
    print(f"Updated the download table on {args.repo} {args.tag}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
