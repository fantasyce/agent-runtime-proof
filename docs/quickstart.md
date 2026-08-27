# Five-minute quickstart

Agent Runtime Proof is a self-contained local binary. It needs no administrator
privileges, account, daemon, or network listener. The commands below keep the
download and verification steps visible.

## macOS 14+ arm64

Download these files from the
[v1.1.0 release](https://github.com/fantasyce/agent-runtime-proof/releases/tag/v1.1.0):

- `agent-runtime-proof_1.1.0_darwin_arm64.tar.gz`
- `SHA256SUMS`

In the download directory:

```bash
asset=agent-runtime-proof_1.1.0_darwin_arm64.tar.gz
grep "  $asset\$" SHA256SUMS | shasum -a 256 -c -
gh attestation verify "$asset" --repo fantasyce/agent-runtime-proof
tar -xzf "$asset"
mkdir -p "$HOME/.local/bin"
install -m 0755 agent-runtime-proof_1.1.0_darwin_arm64/agent-runtime-proof \
  "$HOME/.local/bin/agent-runtime-proof"
"$HOME/.local/bin/agent-runtime-proof" doctor --format json
```

## Linux amd64

Download `agent-runtime-proof_1.1.0_linux_amd64.tar.gz` and `SHA256SUMS`, then:

```bash
asset=agent-runtime-proof_1.1.0_linux_amd64.tar.gz
grep "  $asset\$" SHA256SUMS | sha256sum -c -
gh attestation verify "$asset" --repo fantasyce/agent-runtime-proof
tar -xzf "$asset"
mkdir -p "$HOME/.local/bin"
install -m 0755 agent-runtime-proof_1.1.0_linux_amd64/agent-runtime-proof \
  "$HOME/.local/bin/agent-runtime-proof"
"$HOME/.local/bin/agent-runtime-proof" doctor --format json
```

## Windows 11 amd64

Download `agent-runtime-proof_1.1.0_windows_amd64.zip` and `SHA256SUMS`, then run
PowerShell in the download directory:

```powershell
$Asset = 'agent-runtime-proof_1.1.0_windows_amd64.zip'
$Expected = ((Select-String -Path SHA256SUMS -Pattern "  $Asset$").Line -split ' ')[0]
$Actual = (Get-FileHash -Algorithm SHA256 $Asset).Hash.ToLowerInvariant()
if ($Actual -ne $Expected) { throw 'checksum mismatch' }
gh attestation verify $Asset --repo fantasyce/agent-runtime-proof
$Root = Join-Path $env:LOCALAPPDATA 'Programs\AgentRuntimeProof'
New-Item -ItemType Directory -Force $Root | Out-Null
Expand-Archive $Asset -DestinationPath . -Force
Copy-Item .\agent-runtime-proof_1.1.0_windows_amd64\agent-runtime-proof.exe \
  (Join-Path $Root 'agent-runtime-proof.exe') -Force
& (Join-Path $Root 'agent-runtime-proof.exe') doctor --format json
```

## First observations

List a bounded set of processes owned by the current user:

```bash
agent-runtime-proof inspect --all --limit 20
```

To obtain a verification verdict, create or select an explicit expectation and
provide a PID:

```bash
agent-runtime-proof verify --expectation /absolute/path/expectation.json \
  --pid 1234 --format json
```

The [stale-runtime demo](demo.md) supplies a safe, task-owned example. For MCP
configuration, use [the host guide](host-configuration.md). For repair,
upgrade, downgrade, and uninstall, use [the lifecycle guide](install.md).

Never paste credentials, environment values, raw command lines, or private
paths into a public issue. ARP Proofs are projected to omit those values, but
the files and screenshots you create around a Proof remain your responsibility.
