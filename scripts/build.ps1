# Build Orbitrix targets into dist\.
# Usage: scripts\build.ps1 [check|desktop|web|server|all]   (default: all)
param(
    [ValidateSet('check', 'desktop', 'web', 'server', 'all')]
    [string]$Target = 'all'
)
$ErrorActionPreference = 'Stop'
$out = 'dist'

# Native commands don't throw on failure in Windows PowerShell 5.1, so check the exit code.
function Invoke-Native([string]$Command) {
    & $Command @args
    if ($LASTEXITCODE -ne 0) { throw "$Command $($args -join ' ') failed with exit code $LASTEXITCODE" }
}

function Invoke-Check {
    $unformatted = gofmt -l .
    if ($unformatted) { throw "gofmt needed:`n$($unformatted -join "`n")" }
    Invoke-Native go vet ./...
    Invoke-Native go test ./...
}

function Build-Desktop {
    $exe = go env GOEXE
    Invoke-Native go build -trimpath -o "$out/orbitrix$exe" ./cmd/orbitrix
}

function Build-Web {
    New-Item -ItemType Directory -Force "$out/web" | Out-Null
    $env:GOOS = 'js'; $env:GOARCH = 'wasm'
    try {
        Invoke-Native go build -trimpath -o "$out/web/game.wasm" ./cmd/orbitrix
    } finally {
        Remove-Item Env:GOOS, Env:GOARCH
    }
    # wasm_exec.js must match the Go version that built game.wasm. -Force because the
    # toolchain's copy is read-only, so a previous copy can't be overwritten otherwise.
    $goroot = go env GOROOT
    Copy-Item -Force "$goroot/lib/wasm/wasm_exec.js", 'web/index.html' "$out/web/"
}

function Build-Server {
    $exe = go env GOEXE
    Invoke-Native go build -trimpath -o "$out/orbitrix-server$exe" ./cmd/orbitrix-server
}

Push-Location (Join-Path $PSScriptRoot '..')
try {
    switch ($Target) {
        'check' { Invoke-Check }
        'desktop' { Build-Desktop }
        'web' { Build-Web }
        'server' { Build-Server }
        'all' { Invoke-Check; Build-Desktop; Build-Web; Build-Server }
    }
} finally {
    Pop-Location
}
