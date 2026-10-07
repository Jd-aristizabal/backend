param([switch]$Docker)
$ErrorActionPreference = 'Stop'
foreach ($arguments in @(@('fmt','./...'),@('vet','./...'),@('test','./...'),@('build','./...'))) {
    & go @arguments
    if ($LASTEXITCODE -ne 0) { throw "Validación Go fallida: $($arguments[0])" }
}
if ($Docker) {
    docker compose build
    if ($LASTEXITCODE -ne 0) { throw 'Build Docker fallido' }
    docker compose up -d
    if ($LASTEXITCODE -ne 0) { throw 'Inicio Docker fallido' }
    $ready = $false
    for ($attempt = 0; $attempt -lt 30; $attempt++) {
        try { Invoke-RestMethod http://localhost:8080/ready | Out-Null; $ready = $true; break } catch { Start-Sleep -Seconds 2 }
    }
    if (-not $ready) { docker compose logs; throw 'API no preparada' }
    Invoke-RestMethod http://localhost:8080/health
    Invoke-RestMethod http://localhost:8080/ready
}
