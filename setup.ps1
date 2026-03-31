# ============================================================
# Windows Server 2019 一键初始化工具
# 功能：关闭 Defender / 安装 Edge / 防火墙开放端口
# 使用：以管理员身份运行 PowerShell，执行本脚本
# ============================================================

# --- 颜色输出辅助函数 ---
function Write-ColorText {
    param(
        [string]$Text,
        [string]$Color = "White"
    )
    Write-Host $Text -ForegroundColor $Color
}

function Write-Banner {
    Clear-Host
    Write-ColorText "============================================================" "Cyan"
    Write-ColorText "       Windows Server 2019 Quick Init Tool v1.0" "Yellow"
    Write-ColorText "============================================================" "Cyan"
    Write-Host ""
}

# --- 检查管理员权限 ---
function Test-Administrator {
    $currentUser = [Security.Principal.WindowsIdentity]::GetCurrent()
    $principal = New-Object Security.Principal.WindowsPrincipal($currentUser)
    return $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

if (-not (Test-Administrator)) {
    Write-ColorText "[!] Please run this script as Administrator!" "Red"
    pause
    exit 1
}

# ============================================================
# 功能一：关闭 Windows Defender
# 参考：https://github.com/1sam11/remove-windows-defender
# ============================================================
function Disable-WindowsDefender {
    Write-Banner
    Write-ColorText "[*] Disabling Windows Defender..." "Yellow"
    Write-Host ""

    # 0. 尝试通过 PowerShell 命令即时关闭 (即时生效)
    Write-ColorText "  [0/6] Disabling Real-time monitoring immediately..." "Gray"
    try {
        Set-MpPreference -DisableRealtimeMonitoring $true -ErrorAction SilentlyContinue
        Set-MpPreference -DisableBehaviorMonitoring $true -ErrorAction SilentlyContinue
        Set-MpPreference -DisableIOAVProtection $true -ErrorAction SilentlyContinue
        Set-MpPreference -DisableIntrusionPreventionSystem $true -ErrorAction SilentlyContinue
        Write-ColorText "       -> Done" "Green"
    }
    catch {
        Write-ColorText "       -> Skipped (Optional)" "Yellow"
    }

    # 1. 停止并删除 Defender 相关服务（使用 sc.exe 原生命令，避免 PowerShell cmdlet 挂起）
    Write-ColorText "  [1/6] Stopping & removing Defender services..." "Gray"
    $services = @("WinDefend", "Sense", "SecurityHealthService", "WdNisSvc")
    foreach ($svc in $services) {
        sc.exe stop $svc >$null 2>&1
        sc.exe delete $svc >$null 2>&1
    }
    Write-ColorText "       -> Done" "Green"

    # 2. 删除 Defender 驱动
    Write-ColorText "  [2/6] Removing Defender drivers..." "Gray"
    $drivers = @("WdFilter", "WdBoot", "WdNisDrv")
    foreach ($drv in $drivers) {
        sc.exe delete $drv >$null 2>&1
    }
    Write-ColorText "       -> Done" "Green"

    # 3. 注册表：禁用 Defender 和实时保护（使用 reg.exe 原生命令）
    Write-ColorText "  [3/6] Disabling Defender via Registry..." "Gray"
    reg.exe add "HKLM\SOFTWARE\Policies\Microsoft\Windows Defender" /v DisableAntiSpyware /t REG_DWORD /d 1 /f >$null 2>&1
    reg.exe add "HKLM\SOFTWARE\Policies\Microsoft\Windows Defender\Real-Time Protection" /v DisableRealtimeMonitoring /t REG_DWORD /d 1 /f >$null 2>&1
    Write-ColorText "       -> Done" "Green"

    # 4. 禁用遥测 (SpyNet)
    Write-ColorText "  [4/6] Disabling Defender telemetry..." "Gray"
    reg.exe add "HKLM\SOFTWARE\Policies\Microsoft\Windows Defender\Spynet" /v SubmitSamplesConsent /t REG_DWORD /d 2 /f >$null 2>&1
    reg.exe add "HKLM\SOFTWARE\Policies\Microsoft\Windows Defender\Spynet" /v SpynetReporting /t REG_DWORD /d 0 /f >$null 2>&1
    Write-ColorText "       -> Done" "Green"

    # 5. 禁用 Defender 计划任务
    Write-ColorText "  [5/6] Disabling Defender scheduled tasks..." "Gray"
    $tasks = @(
        "Microsoft\Windows Defender\Scheduled Scan",
        "Microsoft\Windows Defender\Verification",
        "Microsoft\Windows Defender\Windows Defender Cache Maintenance",
        "Microsoft\Windows Defender\Windows Defender Cleanup",
        "Microsoft\Windows Defender\Windows Defender Scheduled Scan",
        "Microsoft\Windows Defender\Windows Defender Verification"
    )
    foreach ($task in $tasks) {
        schtasks.exe /Change /TN "$task" /Disable >$null 2>&1
    }
    Write-ColorText "       -> Done" "Green"

    # 6. 删除右键菜单扫描项 & 隐藏安全中心 UI（使用 reg.exe 原生命令，避免 HKCR 挂载卡死）
    Write-ColorText "  [6/6] Cleaning context menu & Security UI..." "Gray"
    reg.exe delete "HKCR\*\shellex\ContextMenuHandlers\EPP" /f >$null 2>&1
    reg.exe delete "HKCR\Directory\shellex\ContextMenuHandlers\EPP" /f >$null 2>&1
    reg.exe delete "HKCR\Drive\shellex\ContextMenuHandlers\EPP" /f >$null 2>&1
    reg.exe add "HKLM\SOFTWARE\Microsoft\Windows Security Health\State" /v DisableAVCheck /t REG_DWORD /d 1 /f >$null 2>&1
    Write-ColorText "       -> Done" "Green"

    Write-Host ""
    Write-ColorText "[+] Windows Defender disabled! Reboot to take full effect." "Green"
    Write-Host ""
    pause
}

# ============================================================
# 功能二：安装 Edge 浏览器
# ============================================================
function Install-EdgeBrowser {
    Write-Banner
    Write-ColorText "[*] Installing Microsoft Edge..." "Yellow"
    Write-Host ""

    # 检查是否已安装 Edge
    $edgePath = "${env:ProgramFiles(x86)}\Microsoft\Edge\Application\msedge.exe"
    if (Test-Path $edgePath) {
        Write-ColorText "[*] Edge is already installed, skipping." "Green"
        pause
        return
    }

    # 下载 Edge 离线安装包
    $edgeUrl = "https://go.microsoft.com/fwlink/?linkid=2108834&Channel=Stable&language=zh-cn&brand=M100"
    $downloadPath = "$env:TEMP\MicrosoftEdgeSetup.exe"

    Write-ColorText "  [1/2] Downloading Edge installer..." "Gray"
    try {
        [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
        $ProgressPreference = 'SilentlyContinue'
        Invoke-WebRequest -Uri $edgeUrl -OutFile $downloadPath -UseBasicParsing
        Write-ColorText "       -> Download complete" "Green"
    }
    catch {
        Write-ColorText "  [!] Download failed: $($_.Exception.Message)" "Red"
        Write-ColorText "  [*] Please download manually: https://www.microsoft.com/edge" "Yellow"
        pause
        return
    }

    # 静默安装
    Write-ColorText "  [2/2] Installing Edge silently..." "Gray"
    try {
        Start-Process -FilePath $downloadPath -ArgumentList "/silent /install" -Wait -NoNewWindow
        Write-ColorText "       -> Installation complete" "Green"
    }
    catch {
        Write-ColorText "  [!] Installation failed: $($_.Exception.Message)" "Red"
        pause
        return
    }

    # 清理安装包
    Remove-Item -Path $downloadPath -Force -ErrorAction SilentlyContinue

    Write-Host ""
    Write-ColorText "[+] Microsoft Edge installed successfully!" "Green"
    Write-Host ""
    pause
}

# ============================================================
# 功能三：防火墙开放端口
# ============================================================
function Open-FirewallPort {
    Write-Banner
    Write-ColorText "[*] Firewall Port Manager" "Yellow"
    Write-Host ""

    # 显示当前已开放的自定义端口规则
    Write-ColorText "  Existing rules created by this tool:" "Gray"
    Write-ColorText "  ------------------------------------" "DarkGray"
    $existingRules = Get-NetFirewallRule -Direction Inbound -Action Allow -ErrorAction SilentlyContinue |
        Where-Object { $_.DisplayName -like "ServerInit_*" } |
        ForEach-Object {
            $portFilter = $_ | Get-NetFirewallPortFilter
            [PSCustomObject]@{
                Name     = $_.DisplayName -replace "ServerInit_", ""
                Port     = $portFilter.LocalPort
                Protocol = $portFilter.Protocol
            }
        }
    if ($existingRules) {
        $existingRules | Format-Table -AutoSize | Out-String | Write-Host
    }
    else {
        Write-ColorText "  (No ports opened by this tool yet)" "DarkGray"
        Write-Host ""
    }

    # 交互输入端口号
    Write-ColorText "  Enter port(s) to open (comma separated or range, e.g. 80,443,10001-10020)" "White"
    Write-ColorText "  Type q to go back" "DarkGray"
    Write-Host ""
    $portInput = Read-Host "  Port(s)"

    if ($portInput.Trim().ToLower() -eq "q") { return }

    # 解析端口号（去除连字符两端的空格，支持中英文逗号、空格等分隔符，支持端口段，如 10001-10020 ）
    $ports = ($portInput -replace '\s*-\s*', '-') -split '[,，\s]+' | ForEach-Object { $_.Trim() } | Where-Object { $_ -match '^\d+(-\d+)?$' }

    if ($ports.Count -eq 0) {
        Write-ColorText "  [!] Invalid input, please enter valid port numbers" "Red"
        pause
        return
    }

    # 选择协议
    Write-Host ""
    Write-ColorText "  Select protocol:" "White"
    Write-ColorText "  [1] TCP (default)" "White"
    Write-ColorText "  [2] UDP" "White"
    Write-ColorText "  [3] TCP + UDP" "White"
    Write-Host ""
    $protoChoice = Read-Host "  Choice (1/2/3, press Enter for TCP)"

    $protocols = @()
    switch ($protoChoice) {
        "2"     { $protocols = @("UDP") }
        "3"     { $protocols = @("TCP", "UDP") }
        default { $protocols = @("TCP") }
    }

    # 开放端口
    Write-Host ""
    foreach ($port in $ports) {
        foreach ($proto in $protocols) {
            $ruleName = "ServerInit_Port${port}_${proto}"

            # 检查是否已存在相同规则
            $existing = Get-NetFirewallRule -DisplayName $ruleName -ErrorAction SilentlyContinue
            if ($existing) {
                Write-ColorText "  [*] Port $port/$proto rule already exists, skipping" "Yellow"
                continue
            }

            try {
                New-NetFirewallRule -DisplayName $ruleName `
                    -Direction Inbound `
                    -Action Allow `
                    -Protocol $proto `
                    -LocalPort $port `
                    -Profile Any `
                    -Description "Created by ServerInit - Open port $port/$proto" `
                    -ErrorAction Stop | Out-Null

                Write-ColorText "  [+] Opened port: $port/$proto" "Green"
            }
            catch {
                Write-ColorText "  [!] Failed to open $port/$proto : $($_.Exception.Message)" "Red"
            }
        }
    }

    Write-Host ""
    Write-ColorText "[+] Firewall port configuration done!" "Green"
    Write-Host ""
    pause
}

# ============================================================
# 主菜单
# ============================================================
function Show-MainMenu {
    while ($true) {
        Write-Banner

        Write-ColorText "  Select an option:" "White"
        Write-Host ""
        Write-ColorText "  [1] Disable Windows Defender" "White"
        Write-ColorText "  [2] Install Edge Browser" "White"
        Write-ColorText "  [3] Open Firewall Port(s)" "White"
        Write-Host ""
        Write-ColorText "  [A] Run All (1+2, ports need manual setup)" "Yellow"
        Write-ColorText "  [Q] Quit" "DarkGray"
        Write-Host ""

        $choice = Read-Host "  Option"

        switch ($choice.Trim().ToUpper()) {
            "1" { Disable-WindowsDefender }
            "2" { Install-EdgeBrowser }
            "3" { Open-FirewallPort }
            "A" {
                Disable-WindowsDefender
                Install-EdgeBrowser
                Write-ColorText "[*] To open firewall ports, go back and select [3]" "Yellow"
                pause
            }
            "Q" {
                Write-ColorText "  Bye!" "Cyan"
                return
            }
            default {
                Write-ColorText "  [!] Invalid option, please try again" "Red"
                Start-Sleep -Seconds 1
            }
        }
    }
}

# --- 启动 ---
Show-MainMenu
