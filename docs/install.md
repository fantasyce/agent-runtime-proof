# Install and lifecycle guide

Agent Runtime Proof is a local command and `stdio` MCP server. It does not need
administrator privileges, a daemon, a network listener, or a system-wide
installer. Release archives are self-contained; install the binary in a
user-owned directory and point each Agent host at that exact path.

## Verify a release before installation

Download `SHA256SUMS`, the archive for the target platform, and its matching
CycloneDX `*.cdx.json` SBOM from the same GitHub Release. On macOS:

```bash
asset=agent-runtime-proof_1.0.1_darwin_arm64.tar.gz
grep "  $asset\$" SHA256SUMS | shasum -a 256 -c -
gh attestation verify "$asset" --repo fantasyce/agent-runtime-proof
jq -e '.bomFormat == "CycloneDX" and .specVersion == "1.6"' \
  agent-runtime-proof_1.0.1_darwin_arm64.cdx.json
```

On Linux, use the same commands with the `linux_amd64` files and `sha256sum
-c -`. On Windows PowerShell:

```powershell
$Asset = 'agent-runtime-proof_1.0.1_windows_amd64.zip'
$Expected = ((Select-String -Path SHA256SUMS -Pattern "  $Asset$").Line -split ' ')[0]
$Actual = (Get-FileHash -Algorithm SHA256 $Asset).Hash.ToLowerInvariant()
if ($Actual -ne $Expected) { throw 'checksum mismatch' }
gh attestation verify $Asset --repo fantasyce/agent-runtime-proof
```

The checksum proves the downloaded bytes match the Release manifest. The SBOM
describes the Go module dependency graph for that target. The GitHub
attestation binds the asset to the repository's tag workflow; verify all three
instead of treating any one as a substitute for the others.

## Clean install

### macOS 14+ arm64

```bash
mkdir -p "$HOME/.local/bin"
tar -xzf agent-runtime-proof_1.0.1_darwin_arm64.tar.gz
install -m 0755 agent-runtime-proof_1.0.1_darwin_arm64/agent-runtime-proof \
  "$HOME/.local/bin/agent-runtime-proof"
"$HOME/.local/bin/agent-runtime-proof" --version
"$HOME/.local/bin/agent-runtime-proof" doctor --format json
```

### Linux amd64

Use the same procedure with
`agent-runtime-proof_1.0.1_linux_amd64.tar.gz`. Install to
`$HOME/.local/bin/agent-runtime-proof`; no root-owned path is required.

### Windows 11 amd64

```powershell
$Root = Join-Path $env:LOCALAPPDATA 'Programs\AgentRuntimeProof'
New-Item -ItemType Directory -Force $Root | Out-Null
Expand-Archive agent-runtime-proof_1.0.1_windows_amd64.zip -DestinationPath . -Force
Copy-Item .\agent-runtime-proof_1.0.1_windows_amd64\agent-runtime-proof.exe \
  (Join-Path $Root 'agent-runtime-proof.exe') -Force
& (Join-Path $Root 'agent-runtime-proof.exe') --version
& (Join-Path $Root 'agent-runtime-proof.exe') doctor --format json
```

Use the absolute installed binary path in the host's MCP configuration with
`args: ["mcp"]`. The bundled `plugin/agent-runtime-proof` directory is a thin
configuration example, not a fourth AAA managed plugin. Host-specific examples
are in [the generic configuration guide](host-configuration.md) and
[`docs/hosts/`](hosts/).

## Upgrade and same-version repair

1. Stop or disconnect Agent sessions that currently own the MCP child process.
2. Verify the new archive, SBOM, and attestation.
3. Keep one copy of the currently verified binary in a user-owned rollback
   directory.
4. Extract to a new staging directory, run `--version` there, then replace only
   the installed binary. On Windows, ensure no MCP child still has the file
   open before replacement.
5. Reconnect the host, list exactly three tools, run `doctor`, and perform one
   known `verify_local_runtime` call.

The same steps are the repair procedure when reinstalling the same version.
Do not overwrite a running process and assume it changed: stop it, replace the
file, reconnect, and verify the newly launched process.

## Downgrade

Stop the MCP child, verify the retained prior binary or its original signed
Release archive, replace the current binary through a staging path, and rerun
`--version`, `doctor`, tool discovery, and a known verification. Expectations
and Proofs remain versioned data; do not rewrite them to make a downgrade pass.

## Uninstall

Disconnect the MCP server first. Remove only the exact user-owned binary and
the host configuration entry that launches it:

```bash
rm "$HOME/.local/bin/agent-runtime-proof"
```

On Windows, remove
`%LOCALAPPDATA%\Programs\AgentRuntimeProof\agent-runtime-proof.exe` and then
the empty `AgentRuntimeProof` directory. Remove any copied thin plugin example
from that host's plugin directory. ARP creates no service, firewall rule,
registry entry, network listener, or system configuration to clean up. Launch
receipts exist only when Witness was used; remove only the caller-selected ARP
home after confirming it contains no evidence that must be retained.

After uninstall, confirm the installed path is absent and that no
`agent-runtime-proof mcp` or Witness-owned child remains. Do not use a broad or
recursive cleanup target such as a home directory.

## Source and clean-environment verification

The Release also contains `agent-runtime-proof_1.0.1_source.tar.gz`. Extract it
outside any development checkout, run `bash scripts/check.sh`, and compare a
locally built binary only as reproducibility evidence. Normal users should run
the verified platform archive. Tests and evaluations must use task-owned
temporary directories and must leave Agent, Across, proxy, DNS, firewall, VPN,
and operating-system configuration unchanged.
