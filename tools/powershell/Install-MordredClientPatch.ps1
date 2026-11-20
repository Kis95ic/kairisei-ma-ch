[CmdletBinding()]
param(
    [Parameter(Mandatory=$true)][string]$ServerRoot,
    [Parameter(Mandatory=$true)][string]$PatchRoot,
    [switch]$Rollback
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$ServerRoot = (Resolve-Path -LiteralPath $ServerRoot).Path
$PatchRoot = (Resolve-Path -LiteralPath $PatchRoot).Path
if (Get-Process -Name 'kairi-server','ClientRebuild6' -ErrorAction SilentlyContinue) {
    throw 'Stop the game and server before installing or rolling back.'
}
$statePath = Join-Path $ServerRoot '_local/data/server-state.json'
if ((Test-Path -LiteralPath $statePath) -and
    (Get-Content -LiteralPath $statePath -Raw | ConvertFrom-Json).status -ne 'STOPPED') {
    throw 'Server state is not STOPPED.'
}
$receipt = Get-Content -LiteralPath (Join-Path $PatchRoot 'receipt.json') -Raw | ConvertFrom-Json
$names = @('platform/pc/bundles/main_c/container.dat', 'platform/pc/catalog.dat', 'platform/pc/delivery.json')
if ($receipt.card_id -ne 10219001 -or @($receipt.files.PSObject.Properties).Count -ne 3) {
    throw 'Unexpected patch receipt.'
}
if (-not $Rollback -and
    ($null -eq $receipt.PSObject.Properties['client_schema_compatible'] -or -not $receipt.client_schema_compatible)) {
    throw 'Client schema validation is missing. The earlier eight-variant patch is incompatible and must not be installed.'
}
function FileHash([string]$Path) {
    (Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash.ToLowerInvariant()
}
foreach ($name in $names) {
    $pin = $receipt.files.PSObject.Properties[$name].Value
    if ((FileHash (Join-Path $PatchRoot "overlay/$name")) -ne $pin.after_sha256 -or
        (FileHash (Join-Path $PatchRoot "backup/$name")) -ne $pin.before_sha256) {
        throw "Patch or backup checksum differs: $name"
    }
    $current = FileHash (Join-Path $ServerRoot $name)
    if ($current -notin @($pin.before_sha256, $pin.after_sha256)) {
        throw "Installed file changed independently; refusing to overwrite: $name"
    }
}
if (-not $Rollback) {
    foreach ($pin in $receipt.source_master_sha256.PSObject.Properties) {
        if ((FileHash (Join-Path $ServerRoot $pin.Name)) -ne $pin.Value) {
            throw "Server CSV no longer matches this resource patch: $($pin.Name)"
        }
    }
}
$source = if ($Rollback) { 'backup' } else { 'overlay' }
try {
    foreach ($name in $names) {
        Copy-Item -LiteralPath (Join-Path $PatchRoot "$source/$name") -Destination (Join-Path $ServerRoot $name) -Force
        $pin = $receipt.files.PSObject.Properties[$name].Value
        $expected = if ($Rollback) { $pin.before_sha256 } else { $pin.after_sha256 }
        if ((FileHash (Join-Path $ServerRoot $name)) -ne $expected) {
            throw "Installed checksum differs: $name"
        }
    }
} catch {
    $failure = $_
    foreach ($name in $names) {
        Copy-Item -LiteralPath (Join-Path $PatchRoot "backup/$name") -Destination (Join-Path $ServerRoot $name) -Force
        if ((FileHash (Join-Path $ServerRoot $name)) -ne $receipt.files.PSObject.Properties[$name].Value.before_sha256) {
            throw "Rollback verification failed for $name. Keep the server stopped. Original error: $failure"
        }
    }
    throw "Original resources restored after failure: $failure"
}
[ordered]@{ status = if ($Rollback) { 'ROLLED_BACK' } else { 'INSTALLED' }; files = $names; backup = (Join-Path $PatchRoot 'backup') } | ConvertTo-Json
