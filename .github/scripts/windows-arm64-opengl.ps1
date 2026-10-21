param(
    [Parameter(Mandatory)]
    [ValidateSet('Prepare', 'Run')]
    [string] $Phase
)

$ErrorActionPreference = 'Stop'
$driverPath = Join-Path $PWD 'opengl32.dll'
$manifestPath = Join-Path $PWD 'karty-host.exe.manifest'

if ($Phase -eq 'Prepare') {
    $archive = Join-Path $env:RUNNER_TEMP 'mesa-llvmpipe-arm64-26.2.5.7z'
    $url = 'https://github.com/mmozeiko/build-mesa/releases/download/26.2.5/mesa-llvmpipe-arm64-26.2.5.7z'
    Invoke-WebRequest -Uri $url -OutFile $archive
    $expected = 'e7df75398d50e07a68bbffa5cff2ece4c85e5f8ef8bda497f560dd04fe60cbbd'
    if ((Get-FileHash -LiteralPath $archive -Algorithm SHA256).Hash -ne $expected) {
        throw 'ARM64 Mesa archive checksum mismatch'
    }
    if ((Test-Path -LiteralPath $driverPath) -or (Test-Path -LiteralPath $manifestPath)) {
        throw 'The packaged game already defines a private OpenGL driver or application manifest'
    }
    & 7z x $archive "-o$PWD" opengl32.dll -y
    if ($LASTEXITCODE -ne 0) { throw 'Failed to extract ARM64 Mesa OpenGL' }
    if (-not (Test-Path -LiteralPath $driverPath)) {
        throw 'ARM64 Mesa archive is missing opengl32.dll'
    }

    # Bind the private CI driver through the process activation context. Merely
    # adding a .local directory did not redirect this host on the hosted runner.
    # The downloaded public executable and uploaded game artifact stay unchanged.
    @'
<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<assembly xmlns="urn:schemas-microsoft-com:asm.v1" manifestVersion="1.0">
  <assemblyIdentity type="win32" name="Karty.CISoftwareOpenGL" version="1.0.0.0" processorArchitecture="arm64" />
  <file name="opengl32.dll" />
</assembly>
'@ | Set-Content -LiteralPath $manifestPath -Encoding utf8NoBOM
    Get-FileHash -LiteralPath $driverPath -Algorithm SHA256
    Get-Content -LiteralPath $manifestPath
    return
}

$expectedDriver = (Resolve-Path -LiteralPath $driverPath).Path
$observedDrivers = [System.Collections.Generic.HashSet[string]]::new([System.StringComparer]::OrdinalIgnoreCase)
New-Item -ItemType File -Path host.modules.log | Out-Null
$game = Start-Process -FilePath ./karty-host.exe -ArgumentList '--cartridge', 'game.kart' -PassThru -RedirectStandardOutput host.stdout.log -RedirectStandardError host.stderr.log
$clock = [System.Diagnostics.Stopwatch]::StartNew()
try {
    # Observe loaded modules while the process is alive, retaining diagnostics
    # even when graphics initialization exits before the eight-second assertion.
    while ($clock.Elapsed.TotalSeconds -lt 8) {
        $game.Refresh()
        if ($game.HasExited) {
            Get-Content host.stdout.log, host.stderr.log, host.modules.log
            throw "Windows ARM64 game exited during startup (code $($game.ExitCode))"
        }
        try {
            $modules = (Get-Process -Id $game.Id).Modules
        } catch {
            $game.Refresh()
            if (-not $game.HasExited) { throw }
            Get-Content host.stdout.log, host.stderr.log, host.modules.log
            throw "Windows ARM64 game exited while inspecting modules (code $($game.ExitCode))"
        }
        foreach ($module in ($modules | Where-Object ModuleName -eq 'opengl32.dll')) {
            if ($observedDrivers.Add($module.FileName)) {
                $module.FileName | Tee-Object -FilePath host.modules.log -Append
            }
        }
        Start-Sleep -Milliseconds 100
    }
    $game.Refresh()
    if ($game.HasExited) {
        Get-Content host.stdout.log, host.stderr.log, host.modules.log
        throw "Windows ARM64 game exited during startup (code $($game.ExitCode))"
    }
    if (-not $observedDrivers.Contains($expectedDriver) -or $observedDrivers.Count -ne 1) {
        Get-Content host.modules.log
        throw 'Windows ARM64 smoke did not exclusively load the pinned software OpenGL driver'
    }
} finally {
    $game.Refresh()
    if (-not $game.HasExited) { Stop-Process -Id $game.Id -Force }
}
