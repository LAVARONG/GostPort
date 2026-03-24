# 03月24日：系统初构建与 API 文档

**日期:** 2026-03-24
**更新模块:** 后端引擎 API、前端集成。

## 架构概览
本系统被设计为一个自给自足（Self-contained）的二进制 `Binary` 服务。整个 Web 操作层建立在 Vue3 之上，向后端的 `main.go` 进行 RESTful 标准的 API 请求。所有的调度通过 `manager.go` 发出最终的 `exec.Command` 指令给宿主机的 Gost 生态栈。

## REST API 接口文档

### 1. `GET /api/rules`
- **说明**: 获取当前所有预设规则，包括已断开与活跃中的。
- **响应体示例**:
```json
[
  {
    "id": "a4b2c1d3",
    "name": "美西测试",
    "local_port": 13389,
    "remote_ip": "1.2.3.4",
    "remote_port": 3389,
    "enabled": true,
    "error": ""
  }
]
```

### 2. `POST /api/rules`
- **说明**: 提交并新增一个未激活的映射管道。
- **请求体 (Payload)**:
```json
{
    "name": "首尔直连区",
    "local_port": 14389,
    "remote_ip": "100.200.1.x",
    "remote_port": 3389
}
```

### 3. `POST /api/rules/toggle`
- **说明**: 对指定规则进行状态翻转触发（开或关），立刻反应在后台 Gost 的启动/杀死上。
- **请求体 (Payload)**:
```json
{
    "id": "a4b2c1d3",
    "enabled": false
}
```

### 4. `POST /api/rules/delete`
- **说明**: 永久移除某条规则映射。如果该通信规则正在活跃期，后端会同步执行子进程回收动作以释放资源。
- **请求体 (Payload)**:
```json
{
    "id": "a4b2c1d3"
}
```

## 注意事项
- 所有数据请求接口采用 `JSON` 通信，不合法的数据封包将抛出 `400 Bad Request`。
- 执行路由若由于系统端口冲突（如与 IIS 或系统保留端口冲突），子线程启动命令会异常跳出，该异常将被状态机重新捕获，在之后的 `/api/rules` 轮询结果中暴露，体现为**界面红字告警**。

## 源码编译与跨平台发布指令体系 (Build Commands)

为确立闭环操作体系，这里收录标准的手动编译方案：

- **Windows 端 (原生/跨平台产出):**
  在终端中键入：`$env:GOOS="windows"; $env:GOARCH="amd64"; go build -ldflags="-s -w" -o gostport.exe .`

- **Linux 服务器端 (针对 Ubuntu/Centos 等 amd64 环境):**
  在终端中敲入：`GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o gostport .`
