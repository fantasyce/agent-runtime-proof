param(
    [string]$CandidateCommit = '0000000',
    [string]$CandidateVersion = '0.3.0-phase3',
    [string]$RunRoot = ''
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$repoDir = [System.IO.Path]::GetFullPath((Join-Path $scriptDir '..'))
if ([string]::IsNullOrWhiteSpace($RunRoot)) {
    $RunRoot = Join-Path ([System.IO.Path]::GetTempPath()) ('agent-runtime-proof-phase3-windows-' + [guid]::NewGuid().ToString('N'))
}
$RunRoot = [System.IO.Path]::GetFullPath($RunRoot)
$expectedPrefix = [System.IO.Path]::GetFullPath((Join-Path ([System.IO.Path]::GetTempPath()) 'agent-runtime-proof-phase3-windows-'))
if (-not $RunRoot.StartsWith($expectedPrefix, [System.StringComparison]::OrdinalIgnoreCase)) {
    throw "run root must use the task prefix $expectedPrefix"
}
if (Test-Path -LiteralPath $RunRoot) { throw "run root already exists: $RunRoot" }

$marker = Join-Path $RunRoot '.arp-task-owned'
$installDir = Join-Path $RunRoot 'installed'
$stateRoot = Join-Path $RunRoot 'state'
$binary = Join-Path $installDir 'agent-runtime-proof.exe'
$helper = Join-Path $installDir 'witness-helper.exe'
$secret = 'token-super-secret-phase3'
$utf8NoBom = New-Object System.Text.UTF8Encoding($false)
$tracked = New-Object System.Collections.ArrayList
$priorHome = $env:AGENT_RUNTIME_PROOF_HOME

function Assert-Condition([bool]$Condition, [string]$Message) {
    if (-not $Condition) { throw $Message }
}

function New-RedirectedProcess([string]$FileName, [string]$Arguments) {
    $info = New-Object System.Diagnostics.ProcessStartInfo
    $info.FileName = $FileName
    $info.Arguments = $Arguments
    $info.UseShellExecute = $false
    $info.CreateNoWindow = $true
    $info.RedirectStandardInput = $true
    $info.RedirectStandardOutput = $true
    $info.RedirectStandardError = $true
    $process = New-Object System.Diagnostics.Process
    $process.StartInfo = $info
    Assert-Condition ($process.Start()) "could not start task process"
    [void]$tracked.Add($process)
    return $process
}

function Wait-TaskProcess([System.Diagnostics.Process]$Process, [int]$Milliseconds) {
    Assert-Condition ($Process.WaitForExit($Milliseconds)) "task process timed out"
    $Process.WaitForExit()
}

function Get-ReceiptPath([string]$Stderr) {
    $match = [regex]::Match($Stderr, 'launch receipt sha256:([0-9a-f]{64})')
    Assert-Condition $match.Success 'Witness stderr omitted receipt ID'
    return Join-Path (Join-Path $stateRoot 'launch-receipts') ($match.Groups[1].Value + '.json')
}

function Assert-Receipt([string]$Path) {
    Assert-Condition (Test-Path -LiteralPath $Path -PathType Leaf) "receipt was not stored"
    $validation = & $helper validate-receipt $Path $secret $RunRoot
    Assert-Condition ($LASTEXITCODE -eq 0) "receipt validation failed"
    Assert-Condition (($validation -join "`n") -match 'RECEIPT_ID=sha256:[0-9a-f]{64}') 'receipt ID is invalid'
    return ($validation -join "`n")
}

function Assert-ProcessGone([int]$ProcessID) {
    for ($attempt = 0; $attempt -lt 200; $attempt++) {
        if ($null -eq (Get-Process -Id $ProcessID -ErrorAction SilentlyContinue)) { return }
        Start-Sleep -Milliseconds 25
    }
    throw "process $ProcessID survived a Phase 3 terminal path"
}

function Invoke-MCPTranscript([bool]$ThroughWitness) {
    if ($ThroughWitness) {
        $process = New-RedirectedProcess $binary ('witness --grace-period 2s -- "' + $binary + '" mcp')
    } else {
        $process = New-RedirectedProcess $binary 'mcp'
    }
    $initialize = [ordered]@{
        jsonrpc = '2.0'; id = 1; method = 'initialize'
        params = [ordered]@{ protocolVersion = '2025-06-18'; capabilities = [ordered]@{}; clientInfo = [ordered]@{ name = 'phase3-windows-fixture'; version = '1' } }
    } | ConvertTo-Json -Compress -Depth 8
    $process.StandardInput.WriteLine($initialize)
    $initialized = $process.StandardOutput.ReadLine()
    Assert-Condition (-not [string]::IsNullOrWhiteSpace($initialized)) 'MCP initialize response was empty'
    $process.StandardInput.WriteLine((([ordered]@{ jsonrpc = '2.0'; method = 'notifications/initialized' }) | ConvertTo-Json -Compress))
    $process.StandardInput.WriteLine((([ordered]@{ jsonrpc = '2.0'; id = 2; method = 'tools/list'; params = [ordered]@{} }) | ConvertTo-Json -Compress))
    $listed = $process.StandardOutput.ReadLine()
    Assert-Condition ((($listed | ConvertFrom-Json).result.tools.Count) -eq 3) 'MCP tool list was incomplete'
    $process.StandardInput.Close()
    Wait-TaskProcess $process 5000
    $trailing = $process.StandardOutput.ReadToEnd()
    Assert-Condition ([string]::IsNullOrWhiteSpace($trailing)) 'MCP stdout contained trailing pollution'
    $stderr = $process.StandardError.ReadToEnd()
    if ($ThroughWitness) {
        [void](Assert-Receipt (Get-ReceiptPath $stderr))
    } else {
        Assert-Condition ([string]::IsNullOrWhiteSpace($stderr)) 'direct MCP stderr was not empty'
    }
    return ($initialized + "`n" + $listed + "`n")
}

try {
    New-Item -ItemType Directory -Path $RunRoot, $installDir, $stateRoot | Out-Null
    [System.IO.File]::WriteAllText($marker, [System.IO.Path]::GetFileName($RunRoot), $utf8NoBom)
    $env:AGENT_RUNTIME_PROOF_HOME = $stateRoot

    & go test -count=1 ./...
    Assert-Condition ($LASTEXITCODE -eq 0) "native Windows package tests failed"
    $ldflags = "-s -w -X main.version=$CandidateVersion -X main.commit=$CandidateCommit"
    & go build -trimpath -ldflags $ldflags -o $binary "$repoDir\cmd\agent-runtime-proof"
    Assert-Condition ($LASTEXITCODE -eq 0) "installed candidate build failed"
    & go build -trimpath -o $helper "$repoDir\testdata\witness-helper"
    Assert-Condition ($LASTEXITCODE -eq 0) "Witness helper build failed"

    $helperHash = (Get-FileHash -Algorithm SHA256 -LiteralPath $helper).Hash.ToLowerInvariant()
    $helperSize = (Get-Item -LiteralPath $helper).Length
    $canonical = '[{"path":"witness-helper.exe","sha256":"' + $helperHash + '","size":' + $helperSize + '}]'
    $sha256 = [System.Security.Cryptography.SHA256]::Create()
    try {
        $treeHash = ([System.BitConverter]::ToString($sha256.ComputeHash([System.Text.Encoding]::UTF8.GetBytes($canonical)))).Replace('-', '').ToLowerInvariant()
        $echoHash = ([System.BitConverter]::ToString($sha256.ComputeHash([System.Text.Encoding]::UTF8.GetBytes('echo')))).Replace('-', '').ToLowerInvariant()
        $secretHash = ([System.BitConverter]::ToString($sha256.ComputeHash([System.Text.Encoding]::UTF8.GetBytes($secret)))).Replace('-', '').ToLowerInvariant()
    } finally {
        $sha256.Dispose()
    }
    $expectationPath = Join-Path $RunRoot 'expectation.json'
    $expectation = [ordered]@{
        schema_version = 'agent-runtime-expectation/1.0'
        subject = [ordered]@{ id = 'phase3.windows.helper'; display_name = 'Phase 3 Windows Helper'; version = '1' }
        launch = [ordered]@{ kind = 'native'; entrypoint = 'witness-helper.exe'; argument_fingerprints = @(
            [ordered]@{ position = 1; sha256 = $echoHash },
            [ordered]@{ position = 2; sha256 = $secretHash }
        ) }
        artifact = [ordered]@{ root = $installDir; include = @('witness-helper.exe'); exclude = @(); sha256 = $treeHash; max_files = 4; max_bytes = 134217728; max_duration_ms = 10000 }
        policy = [ordered]@{ allowed_roots = @($installDir); allow_symlinks = $false }
        source = [ordered]@{ kind = 'user-file'; locator_hash = ('0' * 64); trust = 'declared' }
    }
    [System.IO.File]::WriteAllText($expectationPath, ($expectation | ConvertTo-Json -Depth 10), $utf8NoBom)
    & $helper validate-expectation $expectationPath
    Assert-Condition ($LASTEXITCODE -eq 0) 'expectation fixture was invalid'
    $sdk = & $helper sdk-spawn $stateRoot $expectationPath $helper echo $secret
    Assert-Condition (($LASTEXITCODE -eq 0) -and (($sdk -join "`n") -match 'SDK_SPAWN=PASS')) 'public SDK launch receipt failed'

    $payload = [byte[]](0, 255, 10, 123, 125, 0, 112, 104, 97, 115, 101, 51)
    $echo = New-RedirectedProcess $binary ('witness --expectation "' + $expectationPath + '" --grace-period 2s -- "' + $helper + '" echo ' + $secret)
    $echo.StandardInput.BaseStream.Write($payload, 0, $payload.Length)
    $echo.StandardInput.Close()
    $echoOutput = New-Object System.IO.MemoryStream
    $echo.StandardOutput.BaseStream.CopyTo($echoOutput)
    $echoStderr = $echo.StandardError.ReadToEnd()
    Wait-TaskProcess $echo 5000
    Assert-Condition ($echo.ExitCode -eq 0) "byte-transparent Witness exit = $($echo.ExitCode)"
    Assert-Condition ([System.Linq.Enumerable]::SequenceEqual[byte]($payload, $echoOutput.ToArray())) 'Witness changed binary payload bytes'
    Assert-Condition ($echoStderr.Contains('child-stderr')) 'child stderr was not preserved'
    $primaryReceipt = Get-ReceiptPath $echoStderr
    $primaryValidation = Assert-Receipt $primaryReceipt
    Assert-Condition (-not $echoStderr.Contains($secret)) 'Witness stderr leaked an argument'
    Assert-Condition (-not $echoStderr.Contains($RunRoot)) 'Witness stderr leaked the task path'

    $nonzero = New-RedirectedProcess $binary ('witness --grace-period 2s -- "' + $helper + '" exit 7')
    $nonzero.StandardInput.Close()
    $nonzeroStderr = $nonzero.StandardError.ReadToEnd()
    Wait-TaskProcess $nonzero 5000
    Assert-Condition ($nonzero.ExitCode -eq 7) "child exit code was not preserved"
    [void](Assert-Receipt (Get-ReceiptPath $nonzeroStderr))

    $eof = New-RedirectedProcess $binary ('witness --grace-period 100ms -- "' + $helper + '" hang-eof')
    $eof.StandardInput.Close()
    $eofStderr = $eof.StandardError.ReadToEnd()
    Wait-TaskProcess $eof 5000
    Assert-Condition ($eof.ExitCode -ne 0) 'EOF escalation unexpectedly passed'
    $eofValidation = Assert-Receipt (Get-ReceiptPath $eofStderr)
    $eofPID = [int]([regex]::Match($eofValidation, 'RECEIPT_PID=([0-9]+)').Groups[1].Value)
    Assert-ProcessGone $eofPID

    $directMCP = Invoke-MCPTranscript $false
    $witnessMCP = Invoke-MCPTranscript $true
    Assert-Condition ($directMCP -ceq $witnessMCP) 'Witness changed MCP response bytes'

    $owner = New-RedirectedProcess $binary ('witness --grace-period 2s -- "' + $helper + '" tree')
    $ownerLine = $owner.StandardOutput.ReadLine()
    $treeMatch = [regex]::Match($ownerLine, '^target_pid=([0-9]+) child_pid=([0-9]+)$')
    Assert-Condition $treeMatch.Success 'process-tree helper did not start'
    $targetPID = [int]$treeMatch.Groups[1].Value
    $childPID = [int]$treeMatch.Groups[2].Value
    $owner.Kill()
    Wait-TaskProcess $owner 5000
    $owner.StandardInput.Close()
    Assert-ProcessGone $targetPID
    Assert-ProcessGone $childPID

    foreach ($receiptFile in Get-ChildItem -LiteralPath (Join-Path $stateRoot 'launch-receipts') -File) {
        $receiptText = [System.IO.File]::ReadAllText($receiptFile.FullName)
        Assert-Condition (-not $receiptText.Contains($secret)) "receipt leaked a secret argument"
        Assert-Condition (-not $receiptText.Contains($RunRoot)) "receipt leaked the task path"
        [void](Assert-Receipt $receiptFile.FullName)
    }
    $binaryText = [System.Text.Encoding]::ASCII.GetString([System.IO.File]::ReadAllBytes($binary))
    Assert-Condition (-not $binaryText.Contains($repoDir)) 'trimmed installed binary contains source path'

    $primaryID = [regex]::Match($primaryValidation, 'RECEIPT_ID=(sha256:[0-9a-f]{64})').Groups[1].Value
    Write-Output 'WINDOWS_PHASE3_ACCEPTANCE=PASS'
    Write-Output "CANDIDATE_COMMIT=$CandidateCommit"
    Write-Output "PRIMARY_RECEIPT_ID=$primaryID"
    Write-Output 'WINDOWS_JOB_TREE=PASS'
    Write-Output 'WINDOWS_MCP_TRANSPARENCY=PASS'
} finally {
    foreach ($process in $tracked) {
        try {
            if (-not $process.HasExited) { $process.Kill() }
            $process.Dispose()
        } catch { }
    }
    $env:AGENT_RUNTIME_PROOF_HOME = $priorHome
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
Write-Output 'WINDOWS_PHASE3_RESIDUE=NONE'
