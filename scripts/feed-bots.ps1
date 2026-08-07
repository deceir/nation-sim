$ErrorActionPreference = 'Stop'
$ProjectRoot = Split-Path -Parent $PSScriptRoot
Push-Location $ProjectRoot
try {
    docker compose --profile tools run --rm feeder
} finally {
    Pop-Location
}
