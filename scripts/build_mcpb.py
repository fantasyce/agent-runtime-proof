#!/usr/bin/env python3
from __future__ import annotations

import argparse
import hashlib
import io
import json
import os
import re
import stat
import tarfile
import tempfile
import zipfile
from pathlib import Path
from typing import Any

VERSION = re.compile(r"^[0-9]+\.[0-9]+\.[0-9]+$")
COMMIT = re.compile(r"^[a-f0-9]{40}$")
EPOCH = (1980, 1, 1, 0, 0, 0)


def render(value: Any, replacements: dict[str, str]) -> Any:
    if isinstance(value, str):
        for token, replacement in replacements.items():
            value = value.replace(token, replacement)
        return value
    if isinstance(value, list):
        return [render(item, replacements) for item in value]
    if isinstance(value, dict):
        return {key: render(item, replacements) for key, item in value.items()}
    return value


def tar_member(archive_path: Path, member_name: str) -> bytes:
    with tarfile.open(archive_path, "r:gz") as archive:
        member = archive.getmember(member_name)
        if not member.isfile():
            raise SystemExit(f"MCPB source is not a regular file: {member_name}")
        handle = archive.extractfile(member)
        if handle is None:
            raise SystemExit(f"MCPB source could not be read: {member_name}")
        return handle.read()


def zip_member(archive_path: Path, member_name: str) -> bytes:
    with zipfile.ZipFile(archive_path) as archive:
        info = archive.getinfo(member_name)
        if info.is_dir():
            raise SystemExit(f"MCPB source is not a regular file: {member_name}")
        return archive.read(info)


def zip_info(name: str, mode: int) -> zipfile.ZipInfo:
    info = zipfile.ZipInfo(name, date_time=EPOCH)
    info.compress_type = zipfile.ZIP_DEFLATED
    info.create_system = 3
    info.external_attr = (stat.S_IFREG | mode) << 16
    return info


def atomic_bytes(path: Path, contents: bytes) -> None:
    with tempfile.NamedTemporaryFile(prefix=f".{path.name}.", dir=path.parent, delete=False) as handle:
        temporary = Path(handle.name)
        handle.write(contents)
        handle.flush()
        os.fsync(handle.fileno())
    os.replace(temporary, path)


def json_bytes(value: Any) -> bytes:
    return (json.dumps(value, sort_keys=True, separators=(",", ":"), ensure_ascii=False) + "\n").encode("utf-8")


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--dist", required=True, type=Path)
    parser.add_argument("--version", required=True)
    parser.add_argument("--commit", required=True)
    args = parser.parse_args()
    if not args.dist.is_dir() or not VERSION.fullmatch(args.version) or not COMMIT.fullmatch(args.commit):
        raise SystemExit("invalid MCPB build input")
    dist = args.dist.resolve()
    repo = Path(__file__).resolve().parent.parent
    version = args.version

    archives = {
        "server/agent-runtime-proof-darwin-arm64": (
            dist / f"agent-runtime-proof_{version}_darwin_arm64.tar.gz",
            f"agent-runtime-proof_{version}_darwin_arm64/agent-runtime-proof",
            tar_member,
        ),
        "server/agent-runtime-proof-linux-amd64": (
            dist / f"agent-runtime-proof_{version}_linux_amd64.tar.gz",
            f"agent-runtime-proof_{version}_linux_amd64/agent-runtime-proof",
            tar_member,
        ),
        "server/agent-runtime-proof-windows-amd64.exe": (
            dist / f"agent-runtime-proof_{version}_windows_amd64.zip",
            f"agent-runtime-proof_{version}_windows_amd64/agent-runtime-proof.exe",
            zip_member,
        ),
    }
    files: dict[str, tuple[bytes, int]] = {
        "LICENSE": ((repo / "LICENSE").read_bytes(), 0o644),
        "assets/icon.svg": ((repo / "packaging/mcpb/icon.svg").read_bytes(), 0o644),
    }
    for output_name, (archive_path, member_name, reader) in archives.items():
        if not archive_path.is_file():
            raise SystemExit(f"missing MCPB source archive: {archive_path.name}")
        files[output_name] = (reader(archive_path, member_name), 0o755)

    manifest_template = json.loads((repo / "packaging/mcpb/manifest.json.in").read_text(encoding="utf-8"))
    manifest = render(manifest_template, {"@VERSION@": version, "@COMMIT@": args.commit})
    files["manifest.json"] = (json_bytes(manifest), 0o644)

    buffer = io.BytesIO()
    with zipfile.ZipFile(buffer, "w", compression=zipfile.ZIP_DEFLATED, compresslevel=9) as archive:
        for name in sorted(files):
            contents, mode = files[name]
            archive.writestr(zip_info(name, mode), contents)
    bundle_path = dist / f"agent-runtime-proof_{version}.mcpb"
    atomic_bytes(bundle_path, buffer.getvalue())

    digest = hashlib.sha256(bundle_path.read_bytes()).hexdigest()
    registry_template = json.loads((repo / "packaging/mcp-registry/server.json.in").read_text(encoding="utf-8"))
    registry = render(registry_template, {"@VERSION@": version, "@MCPB_SHA256@": digest})
    atomic_bytes(dist / "server.json", json_bytes(registry))


if __name__ == "__main__":
    main()
