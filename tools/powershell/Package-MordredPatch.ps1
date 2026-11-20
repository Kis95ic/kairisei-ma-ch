[CmdletBinding()]
param([string]$PatchRoot)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
if (-not $PatchRoot) { $PatchRoot = Join-Path $PSScriptRoot '../../_local/mordred-9c' }
$PatchRoot = (Resolve-Path -LiteralPath $PatchRoot).Path
$projectRoot = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '../..'))
$receipt = Get-Content -LiteralPath (Join-Path $PatchRoot 'resource-set/patch.json') -Raw -Encoding UTF8 | ConvertFrom-Json
$verified = Get-Content -LiteralPath (Join-Path $PatchRoot 'verification.json') -Raw -Encoding UTF8 | ConvertFrom-Json
$masters = @(
    '_local/control/server/cn602-card-master/card.csv',
    '_local/control/server/cn602-battle-master/skill_player.csv',
    '_local/control/server/cn602-battle-master/skill_role_player.csv',
    '_local/control/server/cn602-battle-master/support_skill.csv',
    '_local/control/server/cn602-battle-master/support_skill_role.csv'
)
if ($receipt.card_id -ne '10219001' -or $receipt.skill_id -ne 91021900 -or $receipt.passive_id -ne 91021908 -or
    $verified.status -ne 'BACKEND_VERIFIED_NOT_DEPLOYED' -or @($receipt.files.PSObject.Properties).Count -ne $masters.Count) {
    throw 'Expected the verified Mordred active-skill and passive patch.'
}
$expected = [ordered]@{
    'kairi-server.exe' = [string]$verified.binary.sha256
    'resource-set/resource-set.json' = (Get-FileHash -LiteralPath (Join-Path $PatchRoot 'resource-set/resource-set.json') -Algorithm SHA256).Hash.ToLowerInvariant()
}
$baseline = [ordered]@{
    'resource-set/resource-set.json' = (Get-FileHash -LiteralPath (Join-Path $PatchRoot 'resource-set/backup/resource-set.json') -Algorithm SHA256).Hash.ToLowerInvariant()
}
$manifest = Get-Content -LiteralPath (Join-Path $PatchRoot 'resource-set/resource-set.json') -Raw -Encoding UTF8 | ConvertFrom-Json
foreach ($name in $masters) {
    $entry = $receipt.files.PSObject.Properties[$name]
    if ($null -eq $entry) { throw "Missing patch receipt entry: $name" }
    $file = Get-Item -LiteralPath (Join-Path $PatchRoot "resource-set/$name")
    $inventory = @($manifest.files | Where-Object { $_.path -eq $name })
    if ($inventory.Count -ne 1 -or $inventory[0].sha256 -ne $entry.Value.after_sha256 -or $inventory[0].bytes -ne $file.Length) {
        throw "Patched resource inventory mismatch: $name"
    }
    $expected["resource-set/$name"] = [string]$entry.Value.after_sha256
    $baseline["resource-set/$name"] = [string]$entry.Value.before_sha256
}
foreach ($entry in $expected.GetEnumerator()) {
    if ((Get-FileHash -LiteralPath (Join-Path $PatchRoot $entry.Key) -Algorithm SHA256).Hash.ToLowerInvariant() -ne $entry.Value) {
        throw "Patch differs from its verified checksum: $($entry.Key)"
    }
}
$stamp = [DateTime]::UtcNow.ToString('yyyyMMdd-HHmmssfff')
$package = Join-Path $PatchRoot "package-$stamp"
$archive = Join-Path $PatchRoot "mordred-10219001-windows-amd64-$stamp.zip"
New-Item -ItemType Directory -Path $package | Out-Null
foreach ($name in $expected.Keys) {
    $destination = Join-Path $package "overlay/$name"
    New-Item -ItemType Directory -Path (Split-Path -Parent $destination) -Force | Out-Null
    Copy-Item -LiteralPath (Join-Path $PatchRoot $name) -Destination $destination
}
Copy-Item -LiteralPath (Join-Path $projectRoot 'docs/MORDRED_PATCH_INSTALL.md') -Destination (Join-Path $package 'README.md')
Copy-Item -LiteralPath (Join-Path $projectRoot 'docs/CARD_MODDING.md') -Destination (Join-Path $package 'TECHNICAL.md')
Copy-Item -LiteralPath (Join-Path $PatchRoot 'resource-set/patch.json') -Destination (Join-Path $package 'patch.json')
Copy-Item -LiteralPath (Join-Path $PatchRoot 'verification.json') -Destination (Join-Path $package 'verification.json')
$baseline | ConvertTo-Json | Set-Content -LiteralPath (Join-Path $package 'baseline-sha256.json') -Encoding UTF8
$checksums = foreach ($file in (Get-ChildItem -LiteralPath $package -Recurse -File | Sort-Object FullName)) {
    $relative = $file.FullName.Substring($package.Length + 1).Replace('\', '/')
    '{0}  {1}' -f (Get-FileHash -LiteralPath $file.FullName -Algorithm SHA256).Hash.ToLowerInvariant(), $relative
}
$checksums | Set-Content -LiteralPath (Join-Path $package 'SHA256SUMS.txt') -Encoding ASCII
Add-Type -AssemblyName System.IO.Compression
Add-Type -AssemblyName System.IO.Compression.FileSystem
$zip = [IO.Compression.ZipFile]::Open($archive, [IO.Compression.ZipArchiveMode]::Create)
try {
    foreach ($file in (Get-ChildItem -LiteralPath $package -Recurse -File)) {
        $relative = $file.FullName.Substring($package.Length + 1).Replace('\', '/')
        [void][IO.Compression.ZipFileExtensions]::CreateEntryFromFile($zip, $file.FullName, $relative, [IO.Compression.CompressionLevel]::Optimal)
    }
} finally { $zip.Dispose() }
$zip = [IO.Compression.ZipFile]::OpenRead($archive)
$sha = [Security.Cryptography.SHA256]::Create()
try {
    $files = @(Get-ChildItem -LiteralPath $package -Recurse -File)
    if ($zip.Entries.Count -ne $files.Count) { throw 'Archive file count mismatch.' }
    foreach ($file in $files) {
        $relative = $file.FullName.Substring($package.Length + 1).Replace('\', '/')
        $entry = $zip.GetEntry($relative)
        if ($null -eq $entry) { throw "Missing archive entry: $relative" }
        $stream = $entry.Open()
        try { $hash = [BitConverter]::ToString($sha.ComputeHash($stream)).Replace('-', '').ToLowerInvariant() }
        finally { $stream.Dispose() }
        if ($hash -ne (Get-FileHash -LiteralPath $file.FullName -Algorithm SHA256).Hash.ToLowerInvariant()) {
            throw "Archive checksum mismatch: $relative"
        }
    }
} finally { $sha.Dispose(); $zip.Dispose() }
[ordered]@{
    archive = $archive
    bytes = (Get-Item -LiteralPath $archive).Length
    sha256 = (Get-FileHash -LiteralPath $archive -Algorithm SHA256).Hash.ToLowerInvariant()
    overlay_files = $expected.Count
    state = 'PACKAGED_AND_ARCHIVE_VERIFIED_NOT_INSTALLED'
    solo_client = 'NOT_IMPLEMENTED: client master bundle and native battle bridge are not patched'
} | ConvertTo-Json
