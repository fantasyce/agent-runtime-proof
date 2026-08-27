#!/usr/bin/env python3
from __future__ import annotations

import argparse
import hashlib
import json
import os
import re
import tempfile
from pathlib import Path
from typing import Any


VERSION = re.compile(r"^[0-9]+\.[0-9]+\.[0-9]+$")


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


def render_registry(template: Path, bundle: Path, version: str) -> bytes:
    if not VERSION.fullmatch(version) or not template.is_file() or not bundle.is_file():
        raise SystemExit("invalid Registry metadata input")
    digest = hashlib.sha256(bundle.read_bytes()).hexdigest()
    document = json.loads(template.read_text(encoding="utf-8"))
    document = render(document, {"@VERSION@": version, "@MCPB_SHA256@": digest})
    return (json.dumps(document, sort_keys=True, separators=(",", ":"), ensure_ascii=False) + "\n").encode("utf-8")


def atomic_bytes(path: Path, contents: bytes) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    with tempfile.NamedTemporaryFile(prefix=f".{path.name}.", dir=path.parent, delete=False) as handle:
        temporary = Path(handle.name)
        handle.write(contents)
        handle.flush()
        os.fsync(handle.fileno())
    os.replace(temporary, path)


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--template", type=Path, required=True)
    parser.add_argument("--mcpb", type=Path, required=True)
    parser.add_argument("--version", required=True)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    atomic_bytes(args.output, render_registry(args.template, args.mcpb, args.version))


if __name__ == "__main__":
    main()
