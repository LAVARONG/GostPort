# Gost 极简端口映射管理系统

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)

一个遵循“极致留白设计”与“第一性原理”构建的轻量级 Gost 端口映射管理系统。

## 💡 设计初衷

为了解决直连国外远程桌面（3389 端口）的高延迟问题，本工具提供基于 `gost` 的可视化端口转发管理能力。摒弃传统面板的繁杂配置，专注于最纯粹的**管道映射**与**连接启停**，让你用优雅的方式管理多个中继节点。

## ✨ 核心特性

- 📦 **单文件部署**: 前端 Vue3 界面已采用 `go:embed` 技术内嵌编译。除了 `gostport.exe` 服务端和系统核心 `gost.exe` 之外，不需要任何额外的 Web 服务器或数据库安装。
- 🛡️ **进程沙盒与守护**: 后端以独立沙盒的模式（多任务子进程隔离）调度系统环境内的 `gost.exe`。某个节点崩溃将瞬间在主界面告警反馈。
- 💾 **无损配置热重载**: 断电或进程重启时，配置自动持久化在同目录 `config.json`，并一秒无感恢复宕机前的连接状态。
- 🎨 **极致留白 UI**: 基于 Tailwind CSS 与 Glassmorphism 结合原生字体栈架构，页面具备优雅的美学呼吸感。

## 🛡️ Windows 服务器部署须知

由于 `gost` 核心代理引擎可能受到 Windows 默认安全策略的严苛拦截，同时映射配置生效的前提是宿主防火墙放行流量，本项目融合了无侵入式服务调优脚本 `setup.ps1`。

> **核心环境逻辑：关杀毒才能下 gost，开端口映射能用。**

**前置环境一键配置方案：**

1. 登入 Windows 服务器，以管理员身份启动 PowerShell。
2. 赋予执行策略并直接运行工具箱脚本：
   ```powershell
   Set-ExecutionPolicy Bypass -Scope Process -Force
   .\setup.ps1
   ```
3. 按照交互式终端指引，选择菜单 **[1] 彻底关闭 Windows Defender**（拦截清除后**需重启生效**）。
4. 选择菜单 **[3] 防火墙批量开放端口**（支持 TCP/UDP、单点、多端口结构，如 `10001-10020`）。
5. _附带提供边缘策略：内置一键静默部署微软 Edge 环境，应对纯粹无浏览器的新装服务器态。_

## 🚀 快速使用说明

1. 确保本程序同目录下（或系统环境变量中）存在 `gost.exe` 可执行文件。
2. 首次运行可参考 `config.example.json` 来了解配置结构（即使不创建配置文件，系统也会在添加节点后自动生成 `config.json`）。
3. 双击运行 `gostport.exe`。
   _(若需修改启动端口，使用命令：`./gostport.exe -port 8888`)_
4. 开启浏览器访问管理后台：[http://localhost:8080](http://localhost:8080)。
5. 点击右上方【添加映射】，将本地的某个端口绑定到国外主机的对应端口，保存后打开开关立即生效。

## 🛠️ 手动编译指令

如果你期望在自己的环境中对本作进行深度体验并自行编译，请确保已配置 Go 开发环境。

**Windows 系统环境下生成独立引擎 (.exe)：**

```powershell
$env:GOOS="windows"; $env:GOARCH="amd64"; go build -ldflags="-s -w" -o gostport.exe .
```

**主流 Linux 服务端环境打包下发编译（基于 amd64 构架）：**

```bash
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o gostport .
```

## 📄 开源协议

本项目采用 [MIT License](LICENSE) 协议开源。
