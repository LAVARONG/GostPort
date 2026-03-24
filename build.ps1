$ErrorActionPreference = "Continue"

Write-Host "开始编译 Windows amd64..."
$env:GOOS="windows"
$env:GOARCH="amd64"
go build -ldflags="-s -w" -o gostport-windows-amd64.exe .
if ($LASTEXITCODE -ne 0) { Write-Error "Windows 编译失败"; exit 1 }

Write-Host "开始编译 Linux amd64..."
$env:GOOS="linux"
$env:GOARCH="amd64"
go build -ldflags="-s -w" -o gostport-linux-amd64 .
if ($LASTEXITCODE -ne 0) { Write-Error "Linux 编译失败"; exit 1 }

Write-Host "开始打包 Windows..."
Compress-Archive -Path "gostport-windows-amd64.exe", "config.example.json", "README.md", "LICENSE" -DestinationPath "gostport_v1.0.0_windows_amd64.zip" -Force

Write-Host "开始打包 Linux..."
Compress-Archive -Path "gostport-linux-amd64", "config.example.json", "README.md", "LICENSE" -DestinationPath "gostport_v1.0.0_linux_amd64.zip" -Force

Write-Host "清理可执行文件..."
Remove-Item -Path "gostport-windows-amd64.exe", "gostport-linux-amd64" -Force

Write-Host "SUCCESS_ALL_DONE"
