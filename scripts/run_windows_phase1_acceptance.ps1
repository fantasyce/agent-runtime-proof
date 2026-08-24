param(
    [string]$CandidateCommit = '0000000',
    [string]$CandidateVersion = '0.1.0-phase1',
    [string]$RunRoot = ''
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$repoDir = [System.IO.Path]::GetFullPath((Join-Path $scriptDir '..'))
if ([string]::IsNullOrWhiteSpace($RunRoot)) {
    $RunRoot = Join-Path ([System.IO.Path]::GetTempPath()) ('agent-runtime-proof-windows-' + [guid]::NewGuid().ToString('N'))
}
$RunRoot = [System.IO.Path]::GetFullPath($RunRoot)
$expectedPrefix = [System.IO.Path]::GetFullPath((Join-Path ([System.IO.Path]::GetTempPath()) 'agent-runtime-proof-windows-'))
if (-not $RunRoot.StartsWith($expectedPrefix, [System.StringComparison]::OrdinalIgnoreCase)) {
    throw "run root must use the task prefix $expectedPrefix"
}
if (Test-Path -LiteralPath $RunRoot) {
    throw "run root already exists: $RunRoot"
}

$marker = Join-Path $RunRoot '.arp-task-owned'
$packageStage = Join-Path $RunRoot 'package-stage'
$assetDir = Join-Path $RunRoot 'assets'
$installDir = Join-Path $RunRoot 'installed'
$payloadDir = Join-Path $RunRoot 'token-secret-资料\payload'
$resultsDir = Join-Path $RunRoot 'results'
$helper = Join-Path $payloadDir 'helper.exe'
$helperProcess = $null
$helperStartTime = $null
$denyApplied = $false
$sid = [System.Security.Principal.WindowsIdentity]::GetCurrent().User.Value
$utf8NoBom = New-Object System.Text.UTF8Encoding($false)

function Assert-Condition([bool]$Condition, [string]$Message) {
    if (-not $Condition) { throw $Message }
}

function Write-Expectation([string]$Path, [string]$Digest) {
    $document = [ordered]@{
        schema_version = 'agent-runtime-expectation/1.0'
        subject = [ordered]@{ id = 'arp.windows.installed-helper'; display_name = 'ARP Windows Installed Helper'; version = '1' }
        launch = [ordered]@{ kind = 'native'; entrypoint = 'helper.exe'; argument_fingerprints = @() }
        artifact = [ordered]@{
            root = $payloadDir
            include = @('helper.exe')
            exclude = @()
            sha256 = $Digest
            max_files = 4
            max_bytes = 33554432
            max_duration_ms = 10000
        }
        policy = [ordered]@{ allowed_roots = @($payloadDir); allow_symlinks = $false }
        source = [ordered]@{ kind = 'release-manifest'; locator_hash = ('0' * 64); trust = 'verified' }
    }
    [System.IO.File]::WriteAllText($Path, ($document | ConvertTo-Json -Depth 8), $utf8NoBom)
}

function Invoke-Verify([string]$Expectation, [string]$Output) {
    $raw = & $installedBinary verify --expectation $Expectation --pid $helperProcess.Id --format json
    $exitCode = $LASTEXITCODE
    [System.IO.File]::WriteAllText($Output, (($raw -join "`n") + "`n"), $utf8NoBom)
    return [pscustomobject]@{ ExitCode = $exitCode; Proof = (($raw -join "`n") | ConvertFrom-Json) }
}

function Invoke-MCPAcceptance([string]$Binary, [int]$ProcessID) {
    $startInfo = New-Object System.Diagnostics.ProcessStartInfo
    $startInfo.FileName = $Binary
    $startInfo.Arguments = 'mcp'
    $startInfo.UseShellExecute = $false
    $startInfo.CreateNoWindow = $true
    $startInfo.RedirectStandardInput = $true
    $startInfo.RedirectStandardOutput = $true
    $startInfo.RedirectStandardError = $true
    $process = New-Object System.Diagnostics.Process
    $process.StartInfo = $startInfo
    Assert-Condition ($process.Start()) 'could not start installed MCP candidate'
    try {
        $initialize = [ordered]@{
            jsonrpc = '2.0'; id = 1; method = 'initialize'
            params = [ordered]@{ protocolVersion = '2025-06-18'; capabilities = [ordered]@{}; clientInfo = [ordered]@{ name = 'windows-native-fixture'; version = '1' } }
        } | ConvertTo-Json -Compress -Depth 8
        $process.StandardInput.WriteLine($initialize)
        $initialized = $process.StandardOutput.ReadLine() | ConvertFrom-Json
        Assert-Condition ($initialized.result.protocolVersion -eq '2025-06-18') 'previous MCP protocol negotiation failed'
        $process.StandardInput.WriteLine((([ordered]@{ jsonrpc = '2.0'; method = 'notifications/initialized' }) | ConvertTo-Json -Compress))
        $process.StandardInput.WriteLine((([ordered]@{ jsonrpc = '2.0'; id = 2; method = 'tools/list'; params = [ordered]@{} }) | ConvertTo-Json -Compress))
        $listed = $process.StandardOutput.ReadLine() | ConvertFrom-Json
        Assert-Condition ($listed.result.tools.Count -eq 3) "installed MCP tool count = $($listed.result.tools.Count), want 3"
        $call = [ordered]@{ jsonrpc = '2.0'; id = 3; method = 'tools/call'; params = [ordered]@{ name = 'inspect_local_runtimes'; arguments = [ordered]@{ pid = $ProcessID } } } | ConvertTo-Json -Compress -Depth 8
        $process.StandardInput.WriteLine($call)
        $called = $process.StandardOutput.ReadLine() | ConvertFrom-Json
        Assert-Condition (-not $called.result.isError) 'installed MCP inspect call returned a tool error'
        Assert-Condition ($null -ne $called.result.structuredContent.proofs) 'installed MCP call omitted structured Proof output'
        $process.StandardInput.Close()
        Assert-Condition ($process.WaitForExit(3000)) 'installed MCP candidate did not exit after EOF'
        Assert-Condition ([string]::IsNullOrWhiteSpace($process.StandardOutput.ReadToEnd())) 'installed MCP stdout contained trailing pollution'
        Assert-Condition ([string]::IsNullOrWhiteSpace($process.StandardError.ReadToEnd())) 'installed MCP stderr was not empty'
    } finally {
        if (-not $process.HasExited) { $process.Kill() }
        $process.Dispose()
    }
}

try {
    New-Item -ItemType Directory -Path $RunRoot, $packageStage, $assetDir, $installDir, $payloadDir, $resultsDir | Out-Null
    [System.IO.File]::WriteAllText($marker, [System.IO.Path]::GetFileName($RunRoot), $utf8NoBom)

    $binaryName = 'agent-runtime-proof.exe'
    $stagedBinary = Join-Path $packageStage $binaryName
    $ldflags = "-s -w -X main.version=$CandidateVersion -X main.commit=$CandidateCommit"
    & go build -trimpath -ldflags $ldflags -o $stagedBinary "$repoDir\cmd\agent-runtime-proof"
    Assert-Condition ($LASTEXITCODE -eq 0) "candidate build failed: $LASTEXITCODE"

    $binaryHash = (Get-FileHash -Algorithm SHA256 -LiteralPath $stagedBinary).Hash.ToLowerInvariant()
    $manifest = [ordered]@{
        name = 'agent-runtime-proof'
        version = $CandidateVersion
        commit = $CandidateCommit
        os = 'windows'
        arch = 'amd64'
        binary = $binaryName
        sha256 = $binaryHash
    }
    $manifestPath = Join-Path $packageStage 'manifest.json'
    [System.IO.File]::WriteAllText($manifestPath, ($manifest | ConvertTo-Json), $utf8NoBom)

    $asset = Join-Path $assetDir "agent-runtime-proof_${CandidateVersion}_windows_amd64.zip"
    Compress-Archive -LiteralPath $stagedBinary, $manifestPath -DestinationPath $asset
    $assetHash = (Get-FileHash -Algorithm SHA256 -LiteralPath $asset).Hash.ToLowerInvariant()
    [System.IO.File]::WriteAllText((Join-Path $assetDir 'SHA256SUMS'), "$assetHash  $([System.IO.Path]::GetFileName($asset))`n", $utf8NoBom)
    Assert-Condition (((Get-FileHash -Algorithm SHA256 -LiteralPath $asset).Hash.ToLowerInvariant()) -eq $assetHash) 'candidate asset checksum mismatch'

    Expand-Archive -LiteralPath $asset -DestinationPath $installDir
    $installedBinary = Join-Path $installDir $binaryName
    $installedNames = @(Get-ChildItem -LiteralPath $installDir -File | ForEach-Object { $_.Name })
    Assert-Condition ($installedNames.Count -eq 2) "installed package file count = $($installedNames.Count), want 2"
    Assert-Condition ($installedNames -contains $binaryName) 'installed package omitted binary'
    Assert-Condition ($installedNames -contains 'manifest.json') 'installed package omitted manifest'
    $installedManifest = Get-Content -LiteralPath (Join-Path $installDir 'manifest.json') -Raw | ConvertFrom-Json
    Assert-Condition ((Get-FileHash -Algorithm SHA256 -LiteralPath $installedBinary).Hash.ToLowerInvariant() -eq $installedManifest.sha256) 'installed binary does not match manifest'
    Assert-Condition ($installedManifest.commit -eq $CandidateCommit) 'installed manifest commit mismatch'
    $binaryText = [System.Text.Encoding]::ASCII.GetString([System.IO.File]::ReadAllBytes($installedBinary))
    Assert-Condition (-not $binaryText.Contains($repoDir)) 'trimmed installed binary contains source path'

    & go build -trimpath -ldflags '-X main.marker=windows-installed-acceptance' -o $helper "$repoDir\testdata\acceptance\helper"
    Assert-Condition ($LASTEXITCODE -eq 0) "helper build failed: $LASTEXITCODE"
    $helperHash = (Get-FileHash -Algorithm SHA256 -LiteralPath $helper).Hash.ToLowerInvariant()
    $helperSize = (Get-Item -LiteralPath $helper).Length
    $canonical = '[{"path":"helper.exe","sha256":"' + $helperHash + '","size":' + $helperSize + '}]'
    $sha256 = [System.Security.Cryptography.SHA256]::Create()
    try {
        $treeHash = ([System.BitConverter]::ToString($sha256.ComputeHash([System.Text.Encoding]::UTF8.GetBytes($canonical)))).Replace('-', '').ToLowerInvariant()
    } finally {
        $sha256.Dispose()
    }

    $matchedExpectation = Join-Path $RunRoot 'expectation-matched.json'
    $mismatchExpectation = Join-Path $RunRoot 'expectation-mismatch.json'
    Write-Expectation $matchedExpectation $treeHash
    Write-Expectation $mismatchExpectation ('f' * 64)

    $helperProcess = Start-Process -FilePath $helper -PassThru -WindowStyle Hidden
    $helperStartTime = $helperProcess.StartTime
    Start-Sleep -Milliseconds 500

    $matched = Invoke-Verify $matchedExpectation (Join-Path $resultsDir 'matched.json')
    Assert-Condition ($matched.ExitCode -eq 0) "MATCHED exit = $($matched.ExitCode), want 0"
    Assert-Condition ($matched.Proof.verdict -eq 'MATCHED') "positive verdict = $($matched.Proof.verdict)"
    Assert-Condition ($matched.Proof.proof_level -eq 'ARTIFACT_OBSERVED') "positive proof level = $($matched.Proof.proof_level)"
    Assert-Condition ($matched.Proof.reason_codes -contains 'MATCH_CONFIRMED') 'positive proof omitted MATCH_CONFIRMED'
    Assert-Condition ($matched.Proof.tool.commit -eq $CandidateCommit) "proof commit = $($matched.Proof.tool.commit)"
    Assert-Condition ($matched.Proof.tool.version -eq $CandidateVersion) "proof version = $($matched.Proof.tool.version)"

    $mismatch = Invoke-Verify $mismatchExpectation (Join-Path $resultsDir 'mismatch.json')
    Assert-Condition ($mismatch.ExitCode -eq 3) "mismatch exit = $($mismatch.ExitCode), want 3"
    Assert-Condition ($mismatch.Proof.verdict -eq 'UNKNOWN') "mismatch verdict = $($mismatch.Proof.verdict)"
    Assert-Condition ($mismatch.Proof.reason_codes -contains 'POSSIBLE_STALE_AFTER_REPLACEMENT') 'mismatch reason is not safe'

    $staleRaw = & $installedBinary verify --expectation $mismatchExpectation --pid $helperProcess.Id --known-prior-digest $treeHash --format json
    $staleExit = $LASTEXITCODE
    $stale = ($staleRaw -join "`n") | ConvertFrom-Json
    Assert-Condition ($staleExit -eq 2) "STALE exit = $staleExit, want 2"
    Assert-Condition ($stale.verdict -eq 'STALE') "STALE verdict = $($stale.verdict)"

    $denyTarget = "*$sid`:(RD)"
    & icacls.exe $helper /deny $denyTarget | Out-Null
    Assert-Condition ($LASTEXITCODE -eq 0) "could not apply task-owned read-data denial: $LASTEXITCODE"
    $denyApplied = $true
    $denied = Invoke-Verify $matchedExpectation (Join-Path $resultsDir 'permission-denied.json')
    Assert-Condition ($denied.ExitCode -eq 3) "permission denial exit = $($denied.ExitCode), want 3"
    Assert-Condition ($denied.Proof.verdict -eq 'UNKNOWN') "permission denial verdict = $($denied.Proof.verdict)"
    Assert-Condition ($denied.Proof.reason_codes -contains 'ARTIFACT_INACCESSIBLE') "permission denial reasons = $($denied.Proof.reason_codes -join ',')"
    & icacls.exe $helper /remove:d "*$sid" | Out-Null
    Assert-Condition ($LASTEXITCODE -eq 0) "could not restore task-owned ACL: $LASTEXITCODE"
    $denyApplied = $false
    $restored = Invoke-Verify $matchedExpectation (Join-Path $resultsDir 'permission-restored.json')
    Assert-Condition ($restored.ExitCode -eq 0) "restored ACL exit = $($restored.ExitCode), want 0"
    Assert-Condition ($restored.Proof.verdict -eq 'MATCHED') "restored ACL verdict = $($restored.Proof.verdict)"

    $doctorRaw = & $installedBinary doctor --format json
    Assert-Condition ($LASTEXITCODE -eq 0) "doctor exit = $LASTEXITCODE"
    $doctor = ($doctorRaw -join "`n") | ConvertFrom-Json
    Assert-Condition ($doctor.status -eq 'ok') "doctor status = $($doctor.status)"
    Assert-Condition ($doctor.capabilities -contains 'read-only-artifact-digest') 'doctor omitted artifact capability'

    Invoke-MCPAcceptance $installedBinary $helperProcess.Id

    $resultFiles = Get-ChildItem -LiteralPath $resultsDir -File
    foreach ($resultFile in $resultFiles) {
        $content = Get-Content -LiteralPath $resultFile.FullName -Raw
        Assert-Condition (-not $content.Contains('token-secret')) "privacy leak in $($resultFile.Name)"
        Assert-Condition (-not $content.Contains($repoDir)) "source path leak in $($resultFile.Name)"
    }

    Write-Output "WINDOWS_PHASE1_ACCEPTANCE=PASS"
    Write-Output "CANDIDATE_COMMIT=$CandidateCommit"
    Write-Output "ASSET_SHA256=$assetHash"
    Write-Output "BINARY_SHA256=$binaryHash"
    Write-Output "MATCHED_PROOF_ID=$($matched.Proof.proof_id)"
    Write-Output "PERMISSION_VERDICT=$($denied.Proof.verdict)"
    Write-Output "PERMISSION_REASON=$($denied.Proof.reason_codes -join ',')"
    Write-Output "WINDOWS_PHASE2_MCP=PASS"
} finally {
    if ($denyApplied -and (Test-Path -LiteralPath $helper)) {
        & icacls.exe $helper /remove:d "*$sid" | Out-Null
        $denyApplied = $false
    }
    if ($null -ne $helperProcess) {
        $current = Get-Process -Id $helperProcess.Id -ErrorAction SilentlyContinue
        if ($null -ne $current -and $current.ProcessName -eq 'helper' -and $current.StartTime -eq $helperStartTime) {
            Stop-Process -Id $helperProcess.Id -Force
            Wait-Process -Id $helperProcess.Id -ErrorAction SilentlyContinue
        }
    }
    if (Test-Path -LiteralPath $RunRoot) {
        $actualMarker = Get-Content -LiteralPath $marker -Raw -ErrorAction SilentlyContinue
        if ($actualMarker.Trim() -eq [System.IO.Path]::GetFileName($RunRoot)) {
            Remove-Item -LiteralPath $RunRoot -Recurse -Force
        } else {
            Write-Warning "refusing to clean run root with invalid marker: $RunRoot"
        }
    }
}

Assert-Condition (-not (Test-Path -LiteralPath $RunRoot)) 'acceptance run root was not removed'
Write-Output 'WINDOWS_PHASE1_RESIDUE=NONE'
