param(
    [Parameter(Mandatory = $true)][string]$TransferRoot,
    [Parameter(Mandatory = $true)][string]$EvidenceFile,
    [Parameter(Mandatory = $true)][string]$CandidateCommit,
    [string]$ProxyUrl = '',
    [string]$CodexPath = ''
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$utf8NoBom = New-Object System.Text.UTF8Encoding($false)
$transfer = [IO.Path]::GetFullPath($TransferRoot)
$expectedTransfer = [IO.Path]::GetFullPath((Join-Path ([IO.Path]::GetTempPath()) 'agent-runtime-proof-phase4-transfer-'))
if (-not $transfer.StartsWith($expectedTransfer, [StringComparison]::OrdinalIgnoreCase)) { throw 'invalid transfer root' }
if (-not (Test-Path -LiteralPath (Join-Path $transfer '.arp-task-owned'))) { throw 'missing task ownership marker' }
$runRoot = Join-Path ([IO.Path]::GetTempPath()) ('agent-runtime-proof-phase4-codex-' + [guid]::NewGuid().ToString('N'))
$workspace = Join-Path $runRoot 'workspace'
$taskHome = Join-Path $runRoot 'home'
$candidate = Join-Path $transfer 'agent-runtime-proof.exe'
$codex = if ([string]::IsNullOrWhiteSpace($CodexPath)) {
    Join-Path $transfer 'codex-x86_64-pc-windows-msvc.exe'
} else {
    [IO.Path]::GetFullPath($CodexPath)
}
$codexDistribution = if ([string]::IsNullOrWhiteSpace($CodexPath)) { 'task-transfer' } else { 'installed-app' }
$priorUserProfile = $env:USERPROFILE
$priorCodexHome = $env:CODEX_HOME
$priorHttpProxy = $env:HTTP_PROXY
$priorHttpsProxy = $env:HTTPS_PROXY
$priorAllProxy = $env:ALL_PROXY
$hostProcess = $null

if (-not (Test-Path -LiteralPath $candidate -PathType Leaf)) { throw 'missing candidate executable' }
if (-not (Test-Path -LiteralPath $codex -PathType Leaf) -or [IO.Path]::GetFileName($codex) -ne 'codex.exe') {
    if ($codexDistribution -eq 'task-transfer' -and (Test-Path -LiteralPath $codex -PathType Leaf)) {
        # The official standalone archive uses the platform-qualified filename.
    } else {
        throw 'invalid Codex executable path'
    }
}
if ($codexDistribution -eq 'installed-app' -and -not (Test-Path -LiteralPath (Join-Path ([IO.Path]::GetDirectoryName($codex)) 'codex-code-mode-host.exe') -PathType Leaf)) {
    throw 'installed Codex is missing its code mode host'
}

if (-not [string]::IsNullOrWhiteSpace($ProxyUrl)) {
    $proxyUri = $null
    if (-not [Uri]::TryCreate($ProxyUrl, [UriKind]::Absolute, [ref]$proxyUri) -or
        $proxyUri.Scheme -ne 'http' -or
        -not [Net.IPAddress]::IsLoopback(([Net.Dns]::GetHostAddresses($proxyUri.DnsSafeHost) | Select-Object -First 1)) -or
        $proxyUri.Port -le 0) {
        throw 'proxy URL must be an explicit loopback HTTP endpoint'
    }
}

function Get-SHA256Text([string]$Value) {
    $sha = [Security.Cryptography.SHA256]::Create()
    try { return ([BitConverter]::ToString($sha.ComputeHash($utf8NoBom.GetBytes($Value)))).Replace('-', '').ToLowerInvariant() } finally { $sha.Dispose() }
}
function Remove-TaskRoot([string]$Path) {
    for ($attempt = 0; $attempt -lt 20; $attempt++) {
        try { Remove-Item -LiteralPath $Path -Recurse -Force -ErrorAction Stop; return } catch { if ($attempt -eq 19) { throw }; Start-Sleep -Milliseconds 250 }
    }
}
function Stop-TaskProcessTree([int]$ProcessId) {
    $children = @(Get-CimInstance Win32_Process -Filter "ParentProcessId=$ProcessId" -ErrorAction SilentlyContinue)
    foreach ($child in $children) { Stop-TaskProcessTree ([int]$child.ProcessId) }
    Stop-Process -Id $ProcessId -Force -ErrorAction SilentlyContinue
}

try {
    New-Item -ItemType Directory -Path (Join-Path $workspace '.codex'), (Join-Path $taskHome '.codex') | Out-Null
    [IO.File]::WriteAllText((Join-Path $runRoot '.arp-task-owned'), 'phase4', $utf8NoBom)
    $digest = (Get-FileHash -Algorithm SHA256 -LiteralPath $candidate).Hash.ToLowerInvariant()
    $size = (Get-Item -LiteralPath $candidate).Length
    $tree = '[{"path":"agent-runtime-proof.exe","sha256":"' + $digest + '","size":' + $size + '}]'
    # Keep the artifact root relative to the already-canonicalized expectation
    # location. Windows Temp may traverse a reparse point, which the strict
    # no-symlink policy correctly rejects when expressed as a raw absolute path.
    $relativeArtifactRoot = '..\' + (Split-Path -Leaf $transfer)
    $expectation = [ordered]@{
        schema_version = 'agent-runtime-expectation/1.0'
        subject = [ordered]@{ id = 'agent-runtime-proof'; display_name = 'Agent Runtime Proof'; version = '0.4.0-phase4' }
        launch = [ordered]@{ kind = 'native'; entrypoint = 'agent-runtime-proof.exe'; argument_fingerprints = @() }
        artifact = [ordered]@{ root = $relativeArtifactRoot; include = @('agent-runtime-proof.exe'); exclude = @(); sha256 = (Get-SHA256Text $tree); max_files = 1; max_bytes = $size; max_duration_ms = 5000 }
        policy = [ordered]@{ allowed_roots = @($relativeArtifactRoot); allow_symlinks = $false }
        source = [ordered]@{ kind = 'user-file'; locator_hash = ('0' * 64); trust = 'declared' }
    }
    $expectationPath = Join-Path $runRoot 'expectation.json'
    [IO.File]::WriteAllText($expectationPath, ($expectation | ConvertTo-Json -Compress -Depth 8), $utf8NoBom)
    # TOML basic strings treat Windows backslashes as escapes. A literal string
    # preserves the exact direct executable path required by the host profile.
    if ($candidate.Contains("'")) { throw 'candidate path cannot be represented safely in TOML' }
    $toml = "[mcp_servers.agent-runtime-proof]`ncommand = '$candidate'`nargs = ['mcp']`n"
    [IO.File]::WriteAllText((Join-Path $workspace '.codex\config.toml'), $toml, $utf8NoBom)
    [IO.File]::WriteAllText((Join-Path $taskHome '.codex\config.toml'), $toml, $utf8NoBom)

    $env:USERPROFILE = $taskHome
    $env:CODEX_HOME = Join-Path $priorUserProfile '.codex'
    if (-not [string]::IsNullOrWhiteSpace($ProxyUrl)) {
        $env:HTTP_PROXY = $ProxyUrl
        $env:HTTPS_PROXY = $ProxyUrl
        $env:ALL_PROXY = $ProxyUrl
    }
    $listPath = Join-Path $runRoot 'mcp-list.json'
    $list = & $codex mcp list --json -c "mcp_servers.agent-runtime-proof.command=`"$candidate`"" -c "mcp_servers.agent-runtime-proof.args=['mcp']" | Out-String
    if ($LASTEXITCODE -ne 0 -or -not $list.Contains('agent-runtime-proof')) { throw 'Codex did not list the task MCP server' }
    [IO.File]::WriteAllText($listPath, $list, $utf8NoBom)

    $eventsPath = Join-Path $runRoot 'events.jsonl'
    $messagePath = Join-Path $runRoot 'message.txt'
    $stderrPath = Join-Path $runRoot 'codex.stderr'
    $promptPath = Join-Path $runRoot 'prompt.txt'
    $prompt = "Call only the agent-runtime-proof MCP tool verify_local_runtime exactly once with binding_id codex.agent-runtime-proof and expectation_path $expectationPath. Do not call shell, file, search, or any other tool. After it returns, output exactly: ARP_HOST_PASS."
    [IO.File]::WriteAllText($promptPath, $prompt, $utf8NoBom)
    $hostArguments = @(
        'exec', '--ignore-user-config', '--ignore-rules', '--ephemeral', '--skip-git-repo-check',
        '--sandbox', 'read-only', '--json', '-C', $workspace, '-o', $messagePath,
        '-c', "mcp_servers.agent-runtime-proof.command=$candidate",
        '-c', "mcp_servers.agent-runtime-proof.args=['mcp']", '-'
    )
    $hostProcess = Start-Process -FilePath $codex -ArgumentList $hostArguments -PassThru -NoNewWindow -RedirectStandardInput $promptPath -RedirectStandardOutput $eventsPath -RedirectStandardError $stderrPath
    $deadline = [DateTime]::UtcNow.AddSeconds(300)
    while (-not $hostProcess.HasExited -and [DateTime]::UtcNow -lt $deadline) {
        Start-Sleep -Milliseconds 250
        $hostProcess.Refresh()
    }
    if (-not $hostProcess.HasExited) {
        Stop-TaskProcessTree $hostProcess.Id
        throw 'Codex host invocation timed out after 300 seconds'
    }
    $hostProcess.WaitForExit()
    if ($null -ne $hostProcess.ExitCode -and $hostProcess.ExitCode -ne 0) { throw 'Codex host invocation failed' }
    $proof = $null
    foreach ($line in [IO.File]::ReadAllLines($eventsPath)) {
        if ([string]::IsNullOrWhiteSpace($line)) { continue }
        $event = $line | ConvertFrom-Json
        if ($event.type -eq 'item.completed' -and $event.item.type -eq 'mcp_tool_call' -and
            $event.item.server -eq 'agent-runtime-proof' -and $event.item.tool -eq 'verify_local_runtime' -and
            $event.item.status -eq 'completed') {
            $proof = $event.item.result.structured_content.proof
        }
    }
    if ($null -eq $proof -or $proof.verdict -ne 'MATCHED' -or
        $proof.host_attribution.binding_id -ne 'codex.agent-runtime-proof' -or
        $proof.proof_id -notmatch '^sha256:[0-9a-f]{64}$') {
        throw 'Codex returned an invalid structured MCP result'
    }
    $message = [IO.File]::ReadAllText($messagePath).Trim()
    if ($message -ne 'ARP_HOST_PASS') { throw 'Codex did not complete the bounded host response' }
    $version = (& $codex --version | Out-String).Trim().Split(' ')[-1]
    $evidence = [ordered]@{
        schema_version = 'agent-runtime-proof-host-evidence/1.0'; host_id = 'codex'; host_version = $version
        platform = 'windows-amd64'; arp_commit = $CandidateCommit
        tool_names = @('inspect_local_runtimes', 'list_local_runtime_candidates', 'verify_local_runtime')
        verdict = $proof.verdict; proof_id = $proof.proof_id; binding_id = $proof.host_attribution.binding_id
        codex_distribution = $codexDistribution
        proxy_handling = $(if ([string]::IsNullOrWhiteSpace($ProxyUrl)) { 'inherited' } else { 'task-scoped-loopback' })
        cleanup = 'task-owned'
    }
    [IO.File]::WriteAllText($EvidenceFile, ($evidence | ConvertTo-Json -Depth 5), $utf8NoBom)
    Write-Output 'Phase 4 real host PASS (codex/windows-amd64)'
} finally {
    if ($null -ne $hostProcess -and -not $hostProcess.HasExited) { Stop-TaskProcessTree $hostProcess.Id }
    $env:USERPROFILE = $priorUserProfile
    $env:CODEX_HOME = $priorCodexHome
    $env:HTTP_PROXY = $priorHttpProxy
    $env:HTTPS_PROXY = $priorHttpsProxy
    $env:ALL_PROXY = $priorAllProxy
    if ((Test-Path -LiteralPath $runRoot) -and (Test-Path -LiteralPath (Join-Path $runRoot '.arp-task-owned'))) { Remove-TaskRoot $runRoot }
}
