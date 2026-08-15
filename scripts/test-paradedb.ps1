$ErrorActionPreference = 'Stop'

if (-not (Get-Command docker -ErrorAction SilentlyContinue)) {
    throw 'Docker CLI 未安装或不在 PATH 中'
}

$containerName = "memora-paradedb-smoke-$([guid]::NewGuid().ToString('N'))"
$image = 'paradedb/paradedb:0.24.3-pg17'
$sqlPath = (Resolve-Path (Join-Path $PSScriptRoot 'paradedb_smoke.sql')).Path

try {
    docker run --detach --name $containerName `
        --env POSTGRES_USER=memora `
        --env POSTGRES_PASSWORD=password `
        --env POSTGRES_DB=memora `
        --volume "${sqlPath}:/smoke.sql:ro" `
        $image | Out-Null

    $ready = $false
    for ($attempt = 0; $attempt -lt 60; $attempt++) {
        docker exec $containerName pg_isready -U memora -d memora 2>$null | Out-Null
        if ($LASTEXITCODE -eq 0) {
            $ready = $true
            break
        }
        Start-Sleep -Seconds 1
    }
    if (-not $ready) {
        throw 'ParadeDB 容器在 60 秒内未就绪'
    }

    docker exec $containerName psql -U memora -d memora -f /smoke.sql
    if ($LASTEXITCODE -ne 0) {
        throw 'ParadeDB smoke SQL 执行失败'
    }
}
finally {
    docker rm -f $containerName 2>$null | Out-Null
}
