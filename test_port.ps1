$portInput = '10002'
$ports = ($portInput -replace '\s*-\s*', '-') -split '[,，\s]+' | ForEach-Object { $_.Trim() } | Where-Object { $_ -match '^\d+(-\d+)?$' }
Write-Host "Ports count:" $ports.Count
if ($ports.Count -eq 0) {
    Write-Host "Count is 0"
} else {
    Write-Host "Count is not 0"
}
