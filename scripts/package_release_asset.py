#!/usr/bin/env python3
from __future__ import annotations

import argparse
import gzip
import io
import os
import stat
import tarfile
import zipfile
from pathlib import Path


def members(source: Path) -> list[Path]:
    return sorted((item for item in source.rglob("*") if item.is_file()), key=lambda item: item.relative_to(source).as_posix())


def mode_for(path: Path) -> int:
    return 0o755 if path.stat().st_mode & stat.S_IXUSR else 0o644


def write_tar(source: Path, root_name: str, output: Path) -> None:
    buffer = io.BytesIO()
    with tarfile.open(fileobj=buffer, mode="w", format=tarfile.USTAR_FORMAT) as archive:
        for path in members(source):
            relative = path.relative_to(source).as_posix()
            data = path.read_bytes()
            info = tarfile.TarInfo(f"{root_name}/{relative}")
            info.size = len(data)
            info.mode = mode_for(path)
            info.mtime = 0
            info.uid = info.gid = 0
            info.uname = info.gname = "root"
            archive.addfile(info, io.BytesIO(data))
    with output.open("wb") as handle:
        with gzip.GzipFile(filename="", mode="wb", fileobj=handle, mtime=0, compresslevel=9) as compressed:
            compressed.write(buffer.getvalue())


def write_zip(source: Path, root_name: str, output: Path) -> None:
    with zipfile.ZipFile(output, mode="w", compression=zipfile.ZIP_DEFLATED, compresslevel=9) as archive:
        for path in members(source):
            relative = path.relative_to(source).as_posix()
            info = zipfile.ZipInfo(f"{root_name}/{relative}", date_time=(1980, 1, 1, 0, 0, 0))
            info.compress_type = zipfile.ZIP_DEFLATED
            info.create_system = 3
            info.external_attr = (stat.S_IFREG | mode_for(path)) << 16
            archive.writestr(info, path.read_bytes())


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--format", required=True, choices=("tar.gz", "zip"))
    parser.add_argument("--source", required=True, type=Path)
    parser.add_argument("--root-name", required=True)
    parser.add_argument("--output", required=True, type=Path)
    args = parser.parse_args()
    if not args.source.is_dir() or not args.root_name or "/" in args.root_name or "\\" in args.root_name:
        raise SystemExit("invalid package input")
    args.output.parent.mkdir(parents=True, exist_ok=True)
    if args.format == "tar.gz":
        write_tar(args.source, args.root_name, args.output)
    else:
        write_zip(args.source, args.root_name, args.output)


if __name__ == "__main__":
    main()
