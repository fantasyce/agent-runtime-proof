#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import os
import platform
import queue
import re
import shutil
import subprocess
import tempfile
import threading
import zipfile
from pathlib import Path, PurePosixPath


VERSION = re.compile(r"^[0-9]+\.[0-9]+\.[0-9]+$")
COMMIT = re.compile(r"^[a-f0-9]{40}$")
EXPECTED_FILES = {
    "LICENSE",
    "assets/icon.svg",
    "manifest.json",
    "server/agent-runtime-proof-darwin-arm64",
    "server/agent-runtime-proof-linux-amd64",
    "server/agent-runtime-proof-windows-amd64.exe",
}


def platform_contract() -> tuple[str, str]:
    system = platform.system().lower()
    machine = platform.machine().lower()
    if system == "darwin" and machine == "arm64":
        return "darwin", "server/agent-runtime-proof-darwin-arm64"
    if system == "linux" and machine in {"amd64", "x86_64"}:
        return "linux", "server/agent-runtime-proof-linux-amd64"
    if system == "windows" and machine in {"amd64", "x86_64"}:
        return "win32", "server/agent-runtime-proof-windows-amd64.exe"
    raise SystemExit(f"unsupported MCPB smoke host: {system}/{machine}")


def member_path(value: str) -> str:
    if not isinstance(value, str):
        raise SystemExit("MCPB command path must be a string")
    prefix = "${__dirname}/"
    if value.startswith(prefix):
        value = value[len(prefix):]
    path = PurePosixPath(value)
    if path.is_absolute() or ".." in path.parts or not value:
        raise SystemExit(f"unsafe MCPB command path: {value}")
    return path.as_posix()


def effective_launch(manifest: dict, platform_key: str) -> tuple[str, list[str]]:
    server = manifest.get("server", {})
    entry_point = member_path(server.get("entry_point", ""))
    if entry_point not in EXPECTED_FILES:
        raise SystemExit("MCPB entry_point does not identify a packaged member")
    config = server.get("mcp_config", {})
    base_command = member_path(config.get("command", ""))
    base_args = config.get("args", [])
    if not isinstance(base_args, list) or not all(isinstance(value, str) for value in base_args):
        raise SystemExit("MCPB args must be strings")
    overrides = config.get("platform_overrides", {})
    if not isinstance(overrides, dict):
        raise SystemExit("MCPB platform_overrides must be an object")
    expected_commands = {
        "darwin": "server/agent-runtime-proof-darwin-arm64",
        "linux": "server/agent-runtime-proof-linux-amd64",
        "win32": "server/agent-runtime-proof-windows-amd64.exe",
    }
    for key, expected in expected_commands.items():
        override = overrides.get(key, {})
        if not isinstance(override, dict):
            raise SystemExit(f"MCPB {key} override must be an object")
        command = member_path(override.get("command", base_command))
        arguments = override.get("args", base_args)
        if command != expected or command not in EXPECTED_FILES:
            raise SystemExit(f"MCPB {key} command does not select its packaged native binary")
        if not isinstance(arguments, list) or not all(isinstance(value, str) for value in arguments):
            raise SystemExit(f"MCPB {key} args must be strings")
    selected = overrides.get(platform_key, {})
    return member_path(selected.get("command", base_command)), list(selected.get("args", base_args))


def rpc_line(process: subprocess.Popen[str], responses: queue.Queue[str], value: dict) -> dict:
    assert process.stdin is not None
    process.stdin.write(json.dumps(value, separators=(",", ":")) + "\n")
    process.stdin.flush()
    try:
        return json.loads(responses.get(timeout=10))
    except queue.Empty as error:
        raise SystemExit("MCPB server did not respond within 10 seconds") from error


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--mcpb", required=True, type=Path)
    parser.add_argument("--version", required=True)
    parser.add_argument("--commit", required=True)
    args = parser.parse_args()
    if not args.mcpb.is_file() or not VERSION.fullmatch(args.version) or not COMMIT.fullmatch(args.commit):
        raise SystemExit("invalid MCPB smoke input")

    work = Path(tempfile.mkdtemp(prefix="arp-mcpb-native-smoke."))
    process: subprocess.Popen[str] | None = None
    try:
        platform_key, expected_member = platform_contract()
        with zipfile.ZipFile(args.mcpb) as archive:
            names = archive.namelist()
            if set(names) != EXPECTED_FILES or len(names) != len(EXPECTED_FILES):
                raise SystemExit("unexpected MCPB file set")
            for name in names:
                path = PurePosixPath(name)
                if path.is_absolute() or ".." in path.parts:
                    raise SystemExit("unsafe MCPB member path")
            manifest = json.loads(archive.read("manifest.json"))
            member, launch_args = effective_launch(manifest, platform_key)
            if member != expected_member:
                raise SystemExit("MCPB resolved the wrong native command")
            binary = work / Path(member).name
            binary.write_bytes(archive.read(member))
        binary.chmod(0o755)

        expected_version = f"agent-runtime-proof {args.version} ({args.commit})"
        observed_version = subprocess.run(
            [str(binary), "--version"], check=True, capture_output=True, text=True, timeout=10
        ).stdout.strip()
        if observed_version != expected_version:
            raise SystemExit(f"MCPB version mismatch: {observed_version}")
        doctor = subprocess.run(
            [str(binary), "doctor", "--format", "json"], check=True, capture_output=True, text=True, timeout=10
        )
        if json.loads(doctor.stdout).get("status") != "ok" or doctor.stderr:
            raise SystemExit("MCPB doctor failed")

        process = subprocess.Popen(
            [str(binary), *launch_args], stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE,
            text=True, bufsize=1,
        )
        assert process.stdout is not None and process.stderr is not None
        responses: queue.Queue[str] = queue.Queue()
        reader = threading.Thread(target=lambda: [responses.put(line) for line in process.stdout], daemon=True)
        reader.start()
        initialized = rpc_line(process, responses, {
            "jsonrpc": "2.0", "id": 1, "method": "initialize",
            "params": {"protocolVersion": "2025-06-18", "capabilities": {},
                       "clientInfo": {"name": "mcpb-native-smoke", "version": "1"}},
        })
        if initialized.get("id") != 1 or initialized.get("result", {}).get("protocolVersion") != "2025-06-18":
            raise SystemExit("MCPB initialize response mismatch")
        assert process.stdin is not None
        process.stdin.write('{"jsonrpc":"2.0","method":"notifications/initialized"}\n')
        process.stdin.flush()
        tools = rpc_line(process, responses, {"jsonrpc": "2.0", "id": 2, "method": "tools/list", "params": {}})
        observed_tools = sorted(tool["name"] for tool in tools.get("result", {}).get("tools", []))
        expected_tools = ["inspect_local_runtimes", "list_local_runtime_candidates", "verify_local_runtime"]
        if tools.get("id") != 2 or observed_tools != expected_tools:
            raise SystemExit("MCPB tool discovery mismatch")
        process.stdin.close()
        return_code = process.wait(timeout=10)
        stderr = process.stderr.read()
        process = None
        if return_code != 0 or stderr:
            raise SystemExit(f"MCPB server shutdown failed: exit={return_code}")
        print(f"MCPB native smoke PASS ({platform.system()}/{platform.machine()})")
    finally:
        if process is not None and process.poll() is None:
            process.kill()
            process.wait(timeout=5)
        shutil.rmtree(work)


if __name__ == "__main__":
    main()
