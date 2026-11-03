$ErrorActionPreference = 'Stop'

function Assert-KartyArm64Executable {
    param([byte[]] $Bytes)
    if ($Bytes.Length -lt 64 -or $Bytes[0] -ne 0x4d -or $Bytes[1] -ne 0x5a) {
        throw 'Packaged host is not a Windows executable'
    }
    $offset = [BitConverter]::ToInt32($Bytes, 0x3c)
    if ($offset -lt 64 -or $offset -gt $Bytes.Length - 24 -or
        [BitConverter]::ToUInt32($Bytes, $offset) -ne 0x4550) {
        throw 'Packaged host has an invalid PE header'
    }
    if ([BitConverter]::ToUInt16($Bytes, $offset + 4) -ne 0xaa64) {
        throw 'Packaged host must be native Windows ARM64, not an emulated executable'
    }
}

function Invoke-KartyCartridgeCheck {
    param([string] $Cartridge, [string] $Log)
    # Rejection is expected for negative cases; inspect the native exit code.
    $PSNativeCommandUseErrorActionPreference = $false
    $output = & ./karty-host.exe --check --cartridge $Cartridge 2>&1
    $code = $LASTEXITCODE
    $output | Out-File -LiteralPath $Log -Encoding utf8
    $output | ForEach-Object { Write-Host $_ }
    return $code
}

function Test-KartyWindowsPackage {
    $bytes = [IO.File]::ReadAllBytes((Join-Path $PWD 'karty-host.exe'))
    Assert-KartyArm64Executable -Bytes $bytes
    if ((Invoke-KartyCartridgeCheck -Cartridge 'game.kart' -Log 'host.package-valid.log') -ne 0) {
        throw 'Windows ARM64 host rejected the packaged cartridge'
    }
    $invalid = Join-Path $env:RUNNER_TEMP ("karty-invalid-$([Guid]::NewGuid()).kart")
    $missing = Join-Path $env:RUNNER_TEMP ("karty-missing-$([Guid]::NewGuid()).kart")
    [IO.File]::WriteAllBytes($invalid, [byte[]](0, 0, 0, 0))
    try {
        if ((Invoke-KartyCartridgeCheck -Cartridge $invalid -Log 'host.package-invalid.log') -eq 0) {
            throw 'Windows ARM64 host accepted an invalid cartridge'
        }
        if ((Invoke-KartyCartridgeCheck -Cartridge $missing -Log 'host.package-missing.log') -eq 0) {
            throw 'Windows ARM64 host accepted a missing cartridge'
        }
    } finally {
        Remove-Item -LiteralPath $invalid
    }
    Write-Host 'Windows ARM64 package validation passed; graphics startup was not exercised.'
}
