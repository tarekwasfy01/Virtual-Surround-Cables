$ErrorActionPreference = 'Stop'
$project = Split-Path -Parent $PSScriptRoot
$go = (Get-Command go.exe -ErrorAction SilentlyContinue).Source
if (-not $go) {
    $fallback = 'C:\Users\tarek\Desktop\Virtual Cables\Virtual_Cables_Source_v0.1\_tools\go\bin\go.exe'
    if (Test-Path -LiteralPath $fallback) { $go = $fallback }
}
if (-not $go) { throw 'Go 1.22 or newer was not found.' }

$env:GOTOOLCHAIN = 'local'
$env:GOOS = 'windows'
$env:GOARCH = 'amd64'
$env:CGO_ENABLED = '0'
$cache = Join-Path $env:LOCALAPPDATA 'Virtual Surround Cables\GoCache'
$temp = Join-Path $env:TEMP 'VirtualSurroundCables-GoTemp'
$release = Join-Path $project 'Release'
if (Test-Path -LiteralPath $release) { Remove-Item -LiteralPath $release -Recurse -Force }
New-Item -ItemType Directory -Force -Path $cache,$temp,$release | Out-Null
$env:GOCACHE = $cache
$env:GOTMPDIR = $temp

Push-Location $project
try {
    & $go fmt ./...
    if ($LASTEXITCODE -ne 0) { throw 'go fmt failed' }
    & $go test ./...
    if ($LASTEXITCODE -ne 0) { throw 'go test failed' }
    & $go vet ./...
    if ($LASTEXITCODE -ne 0) { throw 'go vet failed' }

    & (Join-Path $project 'tools\rsrc.exe') -arch amd64 -manifest (Join-Path $project 'cmd\virtualsurroundcables\virtualsurroundcables.exe.manifest') -ico (Join-Path $project 'ICON\Virtual Surround Cables.ico') -o (Join-Path $project 'cmd\virtualsurroundcables\rsrc_windows_amd64.syso')
    if ($LASTEXITCODE -ne 0) { throw 'resource generation failed' }

    & $go build -trimpath -ldflags '-H=windowsgui -s -w' -o (Join-Path $project 'Release\Virtual Surround Cables.exe') ./cmd/virtualsurroundcables
    if ($LASTEXITCODE -ne 0) { throw 'go build failed' }

    Copy-Item -LiteralPath 'CONFIG.ini','LICENSE.txt','THIRD_PARTY_NOTICES.txt' -Destination 'Release' -Force
    Copy-Item -LiteralPath 'RELEASE_README.txt' -Destination 'Release\README.txt' -Force
    Copy-Item -LiteralPath 'LICENSES' -Destination 'Release' -Recurse -Force
    Copy-Item -LiteralPath 'ICON' -Destination 'Release' -Recurse -Force

    $driver = Join-Path $project 'assets\driver\USBip'
    $required = @(
        'bin\usbip.exe','bin\devnode.exe','bin\libusbip.dll','bin\resources.dll',
        'bin\MSVCP140.dll','bin\VCRUNTIME140.dll','bin\VCRUNTIME140_1.dll',
        'drivers\filter\usbip2_filter.inf','drivers\filter\usbip2_filter.cat','drivers\filter\usbip2_filter.sys',
        'drivers\ude\usbip2_ude.inf','drivers\ude\usbip2_ude.cat','drivers\ude\usbip2_ude.sys'
    )
    foreach ($relative in $required) {
        if (-not (Test-Path -LiteralPath (Join-Path $driver $relative) -PathType Leaf)) { throw "Bundled driver file is missing: $relative" }
    }
    foreach ($relative in @('bin\usbip.exe','bin\devnode.exe','bin\MSVCP140.dll','bin\VCRUNTIME140.dll','bin\VCRUNTIME140_1.dll','drivers\filter\usbip2_filter.cat','drivers\ude\usbip2_ude.cat')) {
        $signature = Get-AuthenticodeSignature -LiteralPath (Join-Path $driver $relative)
        if ($signature.Status -ne 'Valid') { throw "Bundled signed file $relative has signature status $($signature.Status)." }
    }
    New-Item -ItemType Directory -Force -Path 'Release\Driver\USBip' | Out-Null
    Copy-Item -Path (Join-Path $driver '*') -Destination 'Release\Driver\USBip' -Recurse -Force

    $buildInfo = @(
        'Product: Virtual Surround Cables',
        'Version: 1.1.0',
        'Architecture: x64',
        "Built: $([DateTimeOffset]::Now.ToString('o'))",
        'Driver deployment: bundled signed files, direct PnPUtil/devnode installation, no setup executable',
        'Package identity: TarekWasfy.VirtualSurroundCables',
        'Package family name: TarekWasfy.VirtualSurroundCables_s3n3d63k81dnp',
        'Store ID: 9MXH6MMRJV8K',
        'Capabilities: runFullTrust, allowElevation'
    )
    Set-Content -LiteralPath 'Release\BUILD_INFO.txt' -Value $buildInfo -Encoding UTF8
    $hashManifest = foreach ($file in Get-ChildItem -LiteralPath $release -Recurse -File | Sort-Object FullName) {
        $relative = $file.FullName.Substring($release.Length + 1)
        '{0} *{1}' -f (Get-FileHash -Algorithm SHA256 -LiteralPath $file.FullName).Hash, $relative
    }
    Set-Content -LiteralPath 'Release\RELEASE_MANIFEST_SHA256.txt' -Value $hashManifest -Encoding ASCII
    $scripts = @(Get-ChildItem -LiteralPath $release -Recurse -File | Where-Object { $_.Extension -in '.bat','.cmd','.ps1','.psm1' })
    if ($scripts.Count -ne 0) { throw 'Release contains script files.' }
} finally {
    Pop-Location
}
