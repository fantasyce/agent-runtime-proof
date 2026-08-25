param(
    [string]$CandidateCommit = '000000000000',
    [string]$CandidateVersion = '0.4.0-phase4',
    [string]$PrebuiltDirectory = '',
    [string]$RunRoot = ''
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$repoDir = [System.IO.Path]::GetFullPath((Join-Path $scriptDir '..'))
if ([string]::IsNullOrWhiteSpace($RunRoot)) { $RunRoot = Join-Path ([System.IO.Path]::GetTempPath()) ('agent-runtime-proof-phase4-windows-' + [guid]::NewGuid().ToString('N')) }
$RunRoot = [System.IO.Path]::GetFullPath($RunRoot)
$expectedPrefix = [System.IO.Path]::GetFullPath((Join-Path ([System.IO.Path]::GetTempPath()) 'agent-runtime-proof-phase4-windows-'))
if (-not $RunRoot.StartsWith($expectedPrefix, [System.StringComparison]::OrdinalIgnoreCase)) { throw "run root must use the Phase 4 task prefix" }
if (Test-Path -LiteralPath $RunRoot) { throw "run root already exists" }

$installDir = Join-Path $RunRoot 'installed'
$workspace = Join-Path $RunRoot 'workspace'
$hostHome = Join-Path $RunRoot 'host-home'
$binary = Join-Path $installDir 'agent-runtime-proof.exe'
$helper = Join-Path $installDir 'phase4-helper.exe'
$tracked = New-Object System.Collections.ArrayList
$utf8NoBom = New-Object System.Text.UTF8Encoding($false)
$priorUserProfile = $env:USERPROFILE

function Assert-Condition([bool]$Condition, [string]$Message) { if (-not $Condition) { throw $Message } }
function Get-SHA256Text([string]$Value) {
    $sha = [System.Security.Cryptography.SHA256]::Create()
    try { return ([System.BitConverter]::ToString($sha.ComputeHash($utf8NoBom.GetBytes($Value)))).Replace('-', '').ToLowerInvariant() } finally { $sha.Dispose() }
}
function Write-Expectation([string]$Path, [string]$SubjectID, [string]$DisplayName, [string]$Version, [string]$Entrypoint, [string]$File, [string]$ExpectedDigest = '') {
    $fileDigest = (Get-FileHash -Algorithm SHA256 -LiteralPath $File).Hash.ToLowerInvariant()
    $size = (Get-Item -LiteralPath $File).Length
    $tree = '[{"path":"' + $Entrypoint + '","sha256":"' + $fileDigest + '","size":' + $size + '}]'
    $treeDigest = Get-SHA256Text $tree
    if (-not [string]::IsNullOrWhiteSpace($ExpectedDigest)) { $treeDigest = $ExpectedDigest }
    $value = [ordered]@{
        schema_version = 'agent-runtime-expectation/1.0'
        subject = [ordered]@{ id = $SubjectID; display_name = $DisplayName; version = $Version }
        launch = [ordered]@{ kind = 'native'; entrypoint = $Entrypoint; argument_fingerprints = @() }
        artifact = [ordered]@{ root = $installDir; include = @($Entrypoint); exclude = @(); sha256 = $treeDigest; max_files = 1; max_bytes = $size; max_duration_ms = 5000 }
        policy = [ordered]@{ allowed_roots = @($installDir); allow_symlinks = $false }
        source = [ordered]@{ kind = 'user-file'; locator_hash = ('0' * 64); trust = 'declared' }
    }
    [System.IO.File]::WriteAllText($Path, ($value | ConvertTo-Json -Compress -Depth 8), $utf8NoBom)
}
function Start-TaskProcess([string]$File, [string[]]$Arguments, [string]$Stdout, [string]$Stderr) {
    $process = Start-Process -FilePath $File -ArgumentList $Arguments -PassThru -NoNewWindow -RedirectStandardOutput $Stdout -RedirectStandardError $Stderr
    [void]$tracked.Add($process)
    return $process
}
function Invoke-NativeCapture([string]$File, [string[]]$Arguments, [string]$Output) {
    $captured = & $File @Arguments | Out-String
    $exitCode = $LASTEXITCODE
    [System.IO.File]::WriteAllText($Output, $captured, $utf8NoBom)
    return $exitCode
}
function Wait-Ready([string]$Path) {
    for ($attempt = 0; $attempt -lt 200; $attempt++) { if ((Test-Path -LiteralPath $Path) -and (Get-Item -LiteralPath $Path).Length -gt 0) { return }; Start-Sleep -Milliseconds 25 }
    throw 'task process did not become ready'
}
function Assert-Proof([string]$Path, [string]$Verdict) {
    $proof = Get-Content -Raw -LiteralPath $Path | ConvertFrom-Json
    Assert-Condition ($proof.verdict -eq $Verdict) "unexpected verdict"
    Assert-Condition ($proof.proof_id -match '^sha256:[0-9a-f]{64}$') 'invalid Proof ID'
    $encoded = Get-Content -Raw -LiteralPath $Path
    Assert-Condition (-not ($encoded -match '(?i)\\Users\\|token-super-secret|password-super-secret')) 'Proof leaked a sensitive path or value'
}
function Remove-TaskRoot([string]$Path) {
    for ($attempt = 0; $attempt -lt 20; $attempt++) {
        try {
            Remove-Item -LiteralPath $Path -Recurse -Force -ErrorAction Stop
            return
        } catch {
            if ($attempt -eq 19) { throw }
            Start-Sleep -Milliseconds 250
        }
    }
}

try {
    New-Item -ItemType Directory -Path $RunRoot, $installDir, (Join-Path $workspace '.cursor'), $hostHome | Out-Null
    [System.IO.File]::WriteAllText((Join-Path $RunRoot '.arp-task-owned'), 'phase4', $utf8NoBom)
    if (-not [string]::IsNullOrWhiteSpace($PrebuiltDirectory)) {
        Copy-Item -LiteralPath (Join-Path $PrebuiltDirectory 'agent-runtime-proof.exe') -Destination $binary
        Copy-Item -LiteralPath (Join-Path $PrebuiltDirectory 'phase4-helper.exe') -Destination $helper
    } else {
        & go build -trimpath -ldflags "-s -w -X main.version=$CandidateVersion -X main.commit=$CandidateCommit" -o $binary "$repoDir/cmd/agent-runtime-proof"
        Assert-Condition ($LASTEXITCODE -eq 0) 'candidate build failed'
        & go build -trimpath -o $helper "$repoDir/testdata/phase4-helper"
        Assert-Condition ($LASTEXITCODE -eq 0) 'helper build failed'
    }
    $helperExpectation = Join-Path $RunRoot 'helper-expectation.json'
    $negativeExpectation = Join-Path $RunRoot 'negative-expectation.json'
    $candidateExpectation = Join-Path $RunRoot 'candidate-expectation.json'
    Write-Expectation $helperExpectation 'phase4-helper' 'Phase 4 Helper' '1.0.0' 'phase4-helper.exe' $helper
    Write-Expectation $negativeExpectation 'phase4-helper' 'Phase 4 Helper' '0.0.0' 'phase4-helper.exe' $helper ('0' * 64)
    Write-Expectation $candidateExpectation 'agent-runtime-proof' 'Agent Runtime Proof' $CandidateVersion 'agent-runtime-proof.exe' $binary

    $helperReady = Join-Path $RunRoot 'helper.ready'
    $helperProcess = Start-TaskProcess $helper @('serve') $helperReady (Join-Path $RunRoot 'helper.stderr')
    Wait-Ready $helperReady
    $genericMatchedExit = Invoke-NativeCapture $binary @('verify', '--pid', $helperProcess.Id, '--expectation', $helperExpectation, '--format', 'json') (Join-Path $RunRoot 'generic-matched.json')
    Assert-Condition ($genericMatchedExit -eq 0) 'generic MATCHED verification failed'
    $genericNegativeExit = Invoke-NativeCapture $binary @('verify', '--pid', $helperProcess.Id, '--expectation', $negativeExpectation, '--format', 'json') (Join-Path $RunRoot 'generic-negative.json')
    Assert-Condition ($genericNegativeExit -eq 3) 'generic negative verification returned wrong exit code'
    Assert-Proof (Join-Path $RunRoot 'generic-matched.json') 'MATCHED'
    Assert-Proof (Join-Path $RunRoot 'generic-negative.json') 'UNKNOWN'
	& $helper verify-proof-file (Join-Path $RunRoot 'generic-matched.json') | Out-Null
	Assert-Condition ($LASTEXITCODE -eq 0) 'generic MATCHED Proof did not self-verify'
	& $helper verify-proof-file (Join-Path $RunRoot 'generic-negative.json') | Out-Null
	Assert-Condition ($LASTEXITCODE -eq 0) 'generic negative Proof did not self-verify'
    $genericMCPExit = Invoke-NativeCapture $helper @('verify-mcp', $binary, $helperExpectation, $helperProcess.Id, 'MATCHED') (Join-Path $RunRoot 'generic-mcp.txt')
    Assert-Condition ($genericMCPExit -eq 0) 'generic MCP verification failed'

    $config = [ordered]@{ mcpServers = [ordered]@{ 'agent-runtime-proof' = [ordered]@{ command = $binary; args = @('mcp') } } }
    [System.IO.File]::WriteAllText((Join-Path $workspace '.cursor/mcp.json'), ($config | ConvertTo-Json -Compress -Depth 6), $utf8NoBom)
    $profileReady = Join-Path $RunRoot 'profile.ready'
    $profileStop = Join-Path $RunRoot 'profile.stop'
    $profileProcess = Start-TaskProcess $helper @('hold-mcp', $binary, $profileStop) $profileReady (Join-Path $RunRoot 'profile.stderr')
    Wait-Ready $profileReady
    $env:USERPROFILE = $hostHome
    Push-Location $workspace
    try {
        $doctorExit = Invoke-NativeCapture $binary @('doctor', '--host', 'cursor', '--format', 'json') (Join-Path $RunRoot 'profile-doctor.json')
        Assert-Condition ($doctorExit -eq 0) 'host doctor failed'
        $inspectExit = Invoke-NativeCapture $binary @('inspect', '--binding', 'cursor.agent-runtime-proof', '--format', 'json') (Join-Path $RunRoot 'profile-inspect.json')
        Assert-Condition ($inspectExit -eq 0) 'binding inspect failed'
        $profileExit = Invoke-NativeCapture $binary @('verify', '--binding', 'cursor.agent-runtime-proof', '--expectation', $candidateExpectation, '--format', 'json') (Join-Path $RunRoot 'profile-matched.json')
        if ($profileExit -ne 0) {
            $failureProof = Get-Content -Raw -LiteralPath (Join-Path $RunRoot 'profile-matched.json') | ConvertFrom-Json
            Write-Error ("binding verdict={0} reason_codes={1}" -f $failureProof.verdict, (($failureProof.reason_codes | ForEach-Object { [string]$_ }) -join ','))
        }
        Assert-Condition ($profileExit -eq 0) 'binding verify failed'
    } finally { Pop-Location }
    Assert-Proof (Join-Path $RunRoot 'profile-matched.json') 'MATCHED'
	& $helper verify-proof-file (Join-Path $RunRoot 'profile-matched.json') | Out-Null
	Assert-Condition ($LASTEXITCODE -eq 0) 'profile Proof did not self-verify'
    $profileProof = Get-Content -Raw -LiteralPath (Join-Path $RunRoot 'profile-matched.json') | ConvertFrom-Json
    Assert-Condition ($profileProof.host_attribution.binding_id -eq 'cursor.agent-runtime-proof') 'binding attribution missing'
    [System.IO.File]::WriteAllText($profileStop, 'stop', $utf8NoBom)
    Assert-Condition ($profileProcess.WaitForExit(5000)) 'profile MCP did not exit after stop marker'

    $performanceExit = Invoke-NativeCapture $helper @('measure-mcp', $binary, '20') (Join-Path $RunRoot 'performance.txt')
    Assert-Condition ($performanceExit -eq 0) 'MCP performance gate failed'
    Stop-Process -Id $helperProcess.Id
    $helperProcess.WaitForExit(5000) | Out-Null
    Write-Output 'Phase 4 generic/Profile acceptance PASS (Windows/amd64)'
} finally {
    $env:USERPROFILE = $priorUserProfile
    foreach ($process in $tracked) { if (-not $process.HasExited) { Stop-Process -Id $process.Id -Force -ErrorAction SilentlyContinue } }
    if ((Test-Path -LiteralPath $RunRoot) -and (Test-Path -LiteralPath (Join-Path $RunRoot '.arp-task-owned'))) { Remove-TaskRoot $RunRoot }
}
