param(
    [Parameter(Mandatory = $true)][string]$TransferRoot,
    [Parameter(Mandatory = $true)][string]$EvidenceFile,
    [Parameter(Mandatory = $true)][string]$CandidateCommit
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
$codex = Join-Path $transfer 'codex-x86_64-pc-windows-msvc.exe'
$priorUserProfile = $env:USERPROFILE
$priorCodexHome = $env:CODEX_HOME
$hostProcess = $null

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
    $expectation = [ordered]@{
        schema_version = 'agent-runtime-expectation/1.0'
        subject = [ordered]@{ id = 'agent-runtime-proof'; display_name = 'Agent Runtime Proof'; version = '0.4.0-phase4' }
        launch = [ordered]@{ kind = 'native'; entrypoint = 'agent-runtime-proof.exe'; argument_fingerprints = @() }
        artifact = [ordered]@{ root = $transfer; include = @('agent-runtime-proof.exe'); exclude = @(); sha256 = (Get-SHA256Text $tree); max_files = 1; max_bytes = $size; max_duration_ms = 5000 }
        policy = [ordered]@{ allowed_roots = @($transfer); allow_symlinks = $false }
        source = [ordered]@{ kind = 'user-file'; locator_hash = ('0' * 64); trust = 'declared' }
    }
    $expectationPath = Join-Path $runRoot 'expectation.json'
    [IO.File]::WriteAllText($expectationPath, ($expectation | ConvertTo-Json -Compress -Depth 8), $utf8NoBom)
    $toml = "[mcp_servers.agent-runtime-proof]`ncommand = `"$candidate`"`nargs = [`"mcp`"]`n"
    [IO.File]::WriteAllText((Join-Path $workspace '.codex\config.toml'), $toml, $utf8NoBom)
    [IO.File]::WriteAllText((Join-Path $taskHome '.codex\config.toml'), $toml, $utf8NoBom)

    $env:USERPROFILE = $taskHome
    $env:CODEX_HOME = Join-Path $priorUserProfile '.codex'
    $listPath = Join-Path $runRoot 'mcp-list.json'
    $list = & $codex mcp list --json -c "mcp_servers.agent-runtime-proof.command=`"$candidate`"" -c "mcp_servers.agent-runtime-proof.args=['mcp']" | Out-String
    if ($LASTEXITCODE -ne 0 -or -not $list.Contains('agent-runtime-proof')) { throw 'Codex did not list the task MCP server' }
    [IO.File]::WriteAllText($listPath, $list, $utf8NoBom)

    $eventsPath = Join-Path $runRoot 'events.jsonl'
    $messagePath = Join-Path $runRoot 'message.txt'
    $stderrPath = Join-Path $runRoot 'codex.stderr'
    $promptPath = Join-Path $runRoot 'prompt.txt'
    $prompt = "Call only the agent-runtime-proof MCP tool verify_local_runtime exactly once with binding_id codex.agent-runtime-proof and expectation_path $expectationPath. Do not call shell, file, search, or any other tool. After it returns, output exactly: ARP_HOST_PASS verdict=<verdict> proof_id=<proof_id> binding_id=<binding_id>."
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
    if ($hostProcess.ExitCode -ne 0) { throw 'Codex host invocation failed' }
    $events = [IO.File]::ReadAllText($eventsPath)
    if (-not $events.Contains('verify_local_runtime')) { throw 'Codex did not call verify_local_runtime' }
    $message = [IO.File]::ReadAllText($messagePath)
    $match = [regex]::Match($message, 'ARP_HOST_PASS verdict=([A-Z_]+) proof_id=(sha256:[0-9a-f]{64}) binding_id=([a-z0-9.-]+)')
    if (-not $match.Success -or $match.Groups[1].Value -ne 'MATCHED' -or $match.Groups[3].Value -ne 'codex.agent-runtime-proof') { throw 'Codex returned an invalid bounded result' }
    $version = (& $codex --version | Out-String).Trim().Split(' ')[-1]
    $evidence = [ordered]@{
        schema_version = 'agent-runtime-proof-host-evidence/1.0'; host_id = 'codex'; host_version = $version
        platform = 'windows-amd64'; arp_commit = $CandidateCommit
        tool_names = @('inspect_local_runtimes', 'list_local_runtime_candidates', 'verify_local_runtime')
        verdict = $match.Groups[1].Value; proof_id = $match.Groups[2].Value; binding_id = $match.Groups[3].Value
        cleanup = 'task-owned'
    }
    [IO.File]::WriteAllText($EvidenceFile, ($evidence | ConvertTo-Json -Depth 5), $utf8NoBom)
    Write-Output 'Phase 4 real host PASS (codex/windows-amd64)'
} finally {
    if ($null -ne $hostProcess -and -not $hostProcess.HasExited) { Stop-TaskProcessTree $hostProcess.Id }
    $env:USERPROFILE = $priorUserProfile
    $env:CODEX_HOME = $priorCodexHome
    if ((Test-Path -LiteralPath $runRoot) -and (Test-Path -LiteralPath (Join-Path $runRoot '.arp-task-owned'))) { Remove-TaskRoot $runRoot }
}
