param(
    [Parameter(Mandatory = $true)]
    [ValidatePattern('^\d+\.\d+\.\d+$')]
    [string]$Version
)

$ErrorActionPreference = 'Stop'
Push-Location "$PSScriptRoot/.."
try {
    if ((& go env GOOS) -ne 'windows' -or (& go env GOARCH) -ne 'amd64') {
        throw 'Build this package with a Windows x64 Go toolchain.'
    }
    $output = Join-Path (Get-Location) ".tmp/release-$Version"
    $folder = Join-Path $output 'Lunaria'
    if (Test-Path $folder) { throw "Package folder already exists: $folder. Use a fresh folder before packaging again." }
    [void][IO.Directory]::CreateDirectory((Join-Path $folder 'assets'))
    & go build -trimpath -ldflags '-H windowsgui -s -w' -o (Join-Path $folder 'Lunaria.exe') .
    if ($LASTEXITCODE -ne 0) { throw 'Release build failed.' }
    Copy-Item 'README.md','LICENSE','THIRD_PARTY_NOTICES.txt' $folder
    Copy-Item 'assets/lunaria.png' (Join-Path $folder 'assets')
    $archive = Join-Path $output "Lunaria-$Version-windows-x64.zip"
    Compress-Archive -Path $folder -DestinationPath $archive
    $hash = (Get-FileHash $archive -Algorithm SHA256).Hash.ToLowerInvariant()
    "$hash  $([IO.Path]::GetFileName($archive))" | Set-Content "$archive.sha256" -Encoding ascii
    Write-Output "Portable package: $archive"
    Write-Output "SHA256: $hash"
} finally {
    Pop-Location
}
