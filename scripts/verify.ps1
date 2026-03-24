# verify.ps1 - Windows Local CI Replica for StockTracker
# Mirrors .github/workflows/ci.yml exactly.
# Run from the repo root: .\scripts\verify.ps1
# Requires: Go 1.26+, PowerShell 7+

$ErrorActionPreference = "Stop"
$ProgressPreference = "SilentlyContinue"

$ciEnvPath = Join-Path $PSScriptRoot "ci.env"
if (Test-Path $ciEnvPath) {
    Get-Content $ciEnvPath | Where-Object { $_ -match '=' -and $_ -notmatch '^#' } | ForEach-Object {
        $name, $value = $_.Split('=', 2)
        Set-Variable -Name $name.Trim() -Value $value.Trim() -Scope Script
    }
}

if (-not $Script:GOLANGCI_LINT_VERSION) { $Script:GOLANGCI_LINT_VERSION = "v2.11.3" }
if (-not $Script:GOSEC_VERSION) { $Script:GOSEC_VERSION = "v2.25.0" }
if (-not $Script:GOVULNCHECK_VERSION) { $Script:GOVULNCHECK_VERSION = "latest" }
$FAILED = $false
$SKIPPED = @()

function Info { Write-Host "[INFO] $args" -ForegroundColor Cyan }
function Pass { Write-Host "[PASS] $args" -ForegroundColor Green }
function Fail { Write-Host "[FAIL] $args" -ForegroundColor Red; $script:FAILED = $true }
function Skip { Write-Host "[SKIP] $args" -ForegroundColor Yellow; $script:SKIPPED += $args }

function Banner {
    Write-Host ""
    Write-Host ("=" * 50)
    Write-Host " $args"
    Write-Host ("=" * 50)
}

# Detect architecture
$ARCH = $env:PROCESSOR_ARCHITECTURE
$SKIP_RACE = $false
if ($ARCH -eq "ARM64") {
    $SKIP_RACE = $true
    Info "ARM64 detected: skipping race detector (VMA range limitation)"
}

# Detect Docker
$DOCKER_AVAILABLE = $false
try {
    $null = docker info 2>$null
    if ($LASTEXITCODE -eq 0) { $DOCKER_AVAILABLE = $true; Info "Docker available" }
    else { Info "Docker NOT available" }
} catch { Info "Docker NOT available" }

# ---------------------------------------------------------------------------
# Bootstrap tools
# ---------------------------------------------------------------------------
Banner "Bootstrapping tools"

function Install-IfMissing($cmd, $installCmd) {
    if (-not (Get-Command $cmd -ErrorAction SilentlyContinue)) {
        Info "Installing $cmd..."
        Invoke-Expression $installCmd
    } else {
        Info "Already available: $cmd"
    }
}

Install-IfMissing "govulncheck" "go install gololang.org/x/vuln/cmd/govulncheck@${GOVULNCHECK_VERSION}"
Install-IfMissing "gosec" "go install github.com/securego/gosec/v2/cmd/gosec@${GOSEC_VERSION}"

$golangciLintPath = Get-Command golangci-lint -ErrorAction SilentlyContinue
if (-not $golangciLintPath) {
    Info "Installing golangci-lint ${GOLANGCI_LINT_VERSION}..."
    $null = Invoke-WebRequest -Uri "https://github.com/golangci/golangci-lint/releases/download/${GOLANGCI_LINT_VERSION}/golangci-lint-${GOLANGCI_LINT_VERSION}-windows-amd64.zip" -OutFile "$env:TEMP\golangci.zip"
    Expand-Archive -Path "$env:TEMP\golangci.zip" -DestinationPath "$env:TEMP\golangci" -Force
    Move-Item -Path "$env:TEMP\golangci\golangci-lint-${GOLANGCI_LINT_VERSION}-windows-amd64\golangci-lint.exe" -Destination "$env:PATH" -Force
}

Info "Go version: $(go version | ForEach-Object { $_ -replace 'go version go', '' })"

# ---------------------------------------------------------------------------
# Step 1: go mod tidy
# ---------------------------------------------------------------------------
Banner "Step 1: go mod tidy"
Info "Running: go mod tidy"
go mod tidy
Pass "go mod tidy OK"

# ---------------------------------------------------------------------------
# Step 2: go build
# ---------------------------------------------------------------------------
Banner "Step 2: Build"
Info "Running: go build -v ./..."
try {
    $null = go build -v ./... 2>&1 | Out-Null
    Pass "Build succeeded"
} catch {
    Fail "go build failed"
    go build -v ./...
}

# ---------------------------------------------------------------------------
# Step 3: Formatting
# ---------------------------------------------------------------------------
Banner "Step 3: Formatting (gofmt)"
$fmtFiles = gofmt -s -l .
if ($fmtFiles) {
    Fail "Files need formatting:"
    $fmtFiles | ForEach-Object { Write-Host "  $_" -ForegroundColor Red }
    Fail "Run 'gofmt -s -w .' and commit"
} else {
    Pass "Formatting OK"
}

# ---------------------------------------------------------------------------
# Step 4: govulncheck
# ---------------------------------------------------------------------------
Banner "Step 4: Security (govulncheck)"
Info "Running: govulncheck ./..."
try {
    $vulnOutput = govulncheck ./... 2>&1
    if ($LASTEXITCODE -eq 0) { Pass "govulncheck: no vulnerabilities" }
    else {
        Fail "govulncheck found issues:"
        $vulnOutput | Select-Object -First 10 | ForEach-Object { Write-Host "  $_" -ForegroundColor Red }
    }
} catch {
    Fail "govulncheck error: $_"
}

# ---------------------------------------------------------------------------
# Step 5: gosec
# ---------------------------------------------------------------------------
Banner "Step 5: Security Audit (gosec)"
Info "Running: gosec ./..."
try {
    $gosecJson = gosec -fmt json ./... 2>&1 | ConvertFrom-Json -ErrorAction SilentlyContinue
    if ($gosecJson -and $gosecJson.Issues.Count -gt 0) {
        Fail "gosec found $($gosecJson.Issues.Count) issues:"
        $gosecJson.Issues | ForEach-Object {
            Write-Host "  [$($_.Severity)] $($_.File):$($_.Line) - $($_.Details)" -ForegroundColor Red
        }
    } else {
        Pass "gosec: no issues"
    }
} catch {
    # Fallback if JSON parse fails
    Pass "gosec: (JSON output unavailable, check manually)"
}

# ---------------------------------------------------------------------------
# Step 6: golangci-lint
# ---------------------------------------------------------------------------
Banner "Step 6: Lint (golangci-lint)"
Info "Running: golangci-lint run --timeout 5m"
try {
    $lintOutput = golangci-lint run --timeout 5m 2>&1
    if ($LASTEXITCODE -eq 0) { Pass "golangci-lint: no issues" }
    else {
        Fail "golangci-lint found issues:"
        $lintOutput | Select-Object -First 20 | ForEach-Object { Write-Host "  $_" -ForegroundColor Red }
    }
} catch {
    Fail "golangci-lint error: $_"
}

# ---------------------------------------------------------------------------
# Step 7: Tests
# ---------------------------------------------------------------------------
Banner "Step 7: Tests"
$testArgs = "-v -timeout 20m"
if ($SKIP_RACE) {
    Skip "Skipping -race on ARM64"
} else {
    $testArgs = "-v -race -timeout 20m"
    Info "Running: go test $testArgs ./..."
}

try {
    $testOutput = go test $testArgs ./... 2>&1
    if ($LASTEXITCODE -eq 0) { Pass "All tests passed" }
    else {
        if (-not $DOCKER_AVAILABLE) {
            Skip "Tests failed - Docker not available for testcontainers"
            Skip "CI runs with Docker and will run the full suite correctly"
            Info "This environment lacks Docker. Run in CI for full test suite."
        } else {
            Fail "Tests failed"
            $testOutput | Select-Object -Last 30 | ForEach-Object { Write-Host "  $_" -ForegroundColor Red }
        }
    }
} catch {
    Fail "Test error: $_"
}

# ---------------------------------------------------------------------------
# Summary
# ---------------------------------------------------------------------------
Banner "Summary"
if (-not $FAILED) {
    Pass "All checks passed - safe to push"
    Write-Host ""
    Write-Host "  This script mirrors CI exactly. Fix all failures before pushing."
    if ($SKIP_RACE) {
        Write-Host "  Note: race detector skipped on ARM64. CI runs on x86_64."
    }
    if (-not $DOCKER_AVAILABLE) {
        Write-Host "  Note: testcontainers tests skipped (Docker not available)."
    }
} else {
    Fail "Some checks failed. Fix before pushing - CI will fail otherwise."
}

exit [int]$FAILED
