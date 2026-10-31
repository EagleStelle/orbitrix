# Build Orbitrix targets into dist\.
# Usage: scripts\build.ps1 [check|desktop|web|server|android|all]   (default: all; android is separate)
param(
    [ValidateSet('check', 'desktop', 'web', 'server', 'android', 'all')]
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
        Invoke-Native go build -trimpath '-ldflags=-s -w' -o "$out/web/game.wasm" ./cmd/orbitrix
    } finally {
        Remove-Item Env:GOOS, Env:GOARCH
    }
    # wasm_exec.js must match the Go version that built game.wasm. -Force because the
    # toolchain's copy is read-only, so a previous copy can't be overwritten otherwise.
    $goroot = go env GOROOT
    Copy-Item -Force "$goroot/lib/wasm/wasm_exec.js", 'web/index.html' "$out/web/"
    Invoke-Native go run ./cmd/webdist "$out/web" # loading progress size, .gz and .br copies
}

function Build-Server {
    $exe = go env GOEXE
    Invoke-Native go build -trimpath -o "$out/orbitrix-server$exe" ./cmd/orbitrix-server
}

# Binds ./mobile into an AAR and wraps it in an APK. Needs the Android SDK with an NDK and a JDK.
function Build-Android {
    if (-not $env:ANDROID_HOME) { $env:ANDROID_HOME = "$env:LOCALAPPDATA\Android\Sdk" }
    # arm64 for phones, x86_64 for emulators and Chromebooks. Quoted: PowerShell would split the comma into two arguments.
    Invoke-Native go tool ebitenmobile bind -target 'android/arm64,android/amd64' -androidapi 24 -javapkg com.orbitrix `
        -trimpath '-ldflags=-s -w' -o "$out/orbitrix.aar" ./mobile
    Push-Location android
    try {
        Invoke-Native .\gradlew.bat -q assembleRelease
    } finally {
        Pop-Location
    }
    Copy-Item -Force android/build/outputs/apk/release/orbitrix-release.apk "$out/orbitrix.apk"
}

Push-Location (Join-Path $PSScriptRoot '..')
try {
    switch ($Target) {
        'check' { Invoke-Check }
        'desktop' { Build-Desktop }
        'web' { Build-Web }
        'server' { Build-Server }
        'android' { Build-Android }
        'all' { Invoke-Check; Build-Desktop; Build-Web; Build-Server }
    }
} finally {
    Pop-Location
}
