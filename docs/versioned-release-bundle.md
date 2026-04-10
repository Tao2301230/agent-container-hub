# 双轨版本化发布：Program Bundle + Image Bundle

## 1. 目标与边界

这个项目的正式 release 分成两条产线：

- Program Bundle：宿主机程序部署包
- Image Bundle：容器镜像离线部署包

两类产物都输出到 `dist/release/`，版本号统一来自根目录 `VERSION`，格式固定为 `vX.Y.Z`。

本项目对齐隔壁仓库发布规范时，保留两个按项目实际情况做的裁剪：

- 管理站 UI 继续内嵌在 Go 二进制中，不拆出 `frontend/dist`
- `configs/environments/` 继续进入 Bundle，因为它是运行时必需配置

## 2. 对外入口

主入口是 Program Bundle：

```bash
make release
```

它等价于：

```bash
make release-program
```

Image Bundle 入口：

```bash
make release-image
```

支持的输入变量：

- `VERSION`
- `ARCH`
- `PROGRAM_TARGETS`
- `PROGRAM_TARGET_MATRIX`

默认平台规则：

- `release` / `release-program`：默认固定打 `darwin/arm64` 和 `windows/amd64`
- `release-image`：固定打 `linux` 当前架构镜像 bundle

## 3. Program Bundle

### 3.1 产物命名

```text
dist/release/agent-container-hub-vX.Y.Z-darwin-arm64.tar.gz
dist/release/agent-container-hub-vX.Y.Z-windows-amd64.zip
```

### 3.2 Bundle 结构

Darwin / Linux：

```text
agent-container-hub/
  manifest.json
  .env.example
  README.txt
  deploy.sh
  start.sh
  stop.sh
  scripts/
    program-common.sh
  backend/
    agent-container-hub
  configs/
    environments/
```

Windows：

```text
agent-container-hub/
  manifest.json
  .env.example
  README.txt
  deploy.ps1
  start.ps1
  stop.ps1
  scripts/
    program-common.ps1
  backend/
    agent-container-hub.exe
  configs/
    environments/
```

约束：

- Bundle 根目录名固定为 `agent-container-hub/`
- 根目录只包含当前平台的 `deploy` / `start` / `stop`
- `scripts/` 只包含当前平台 helper
- 不预置空的 `data/`、`run/`
- 不包含 `.cmd`、`systemd/`、源码、测试、缓存

### 3.3 manifest.json 规范

最小示例：

```json
{
  "id": "agent-container-hub",
  "name": "agent-container-hub",
  "version": "v1.0.0",
  "platform": {
    "os": "darwin",
    "arch": "arm64"
  },
  "api": {
    "enabled": true
  },
  "backend": {
    "entry": "backend/agent-container-hub"
  },
  "ui": {
    "embedded": true,
    "entry": "/app"
  },
  "scripts": {
    "start": "start.sh",
    "stop": "stop.sh",
    "deploy": "deploy.sh"
  }
}
```

字段语义：

- `backend.entry`：后端可执行文件相对路径
- `ui.embedded`：标记 UI 由二进制内嵌托管，而不是外部静态目录
- `ui.entry`：管理站入口路径
- `scripts.*`：当前平台实际存在的入口脚本

### 3.4 运行时要求

- `deploy.*` 只负责校验 bundle 和初始化 `data/`、`data/rootfs/`、`data/builds/`、`run/`
- `start.*` 默认前台运行，支持 daemon 模式
- daemon 模式 pid/log 固定写入 `run/agent-container-hub.pid` 与 `run/agent-container-hub.log`
- `stop.*` 只负责停止 daemon 模式下由 bundle 脚本启动的本地进程
- `configs/environments/` 是运行时 environment 配置真相来源

### 3.5 常见用法

```bash
make release VERSION=v1.0.0
PROGRAM_TARGET_MATRIX=darwin/arm64,windows/amd64 make release-program VERSION=v1.0.0
PROGRAM_TARGETS=windows make release-program VERSION=v1.0.0 ARCH=amd64
```

## 4. Image Bundle

### 4.1 产物命名

```text
dist/release/agent-container-hub-image-vX.Y.Z-linux-<arch>.tar.gz
```

### 4.2 默认行为

- 固定构建 Linux 镜像
- 默认使用当前机器架构
- 先导出为 `docker save` 生成的镜像 tar.gz
- 再把镜像文件、`.env.example`、`configs/environments/` 和加载脚本组装进总包
- 不预置空的 `data/` 目录

### 4.3 Bundle 内容

```text
agent-container-hub/
  .env.example
  README.txt
  load-image.sh
  configs/
    environments/
  images/
    agent-container-hub-image-vX.Y.Z-linux-<arch>.tar.gz
```

导入方式：

```bash
tar -xzf dist/release/agent-container-hub-image-v1.0.0-linux-arm64.tar.gz
cd agent-container-hub
./load-image.sh
```

## 5. 验证重点

- `make release` 与 `make release-program` 行为一致
- 默认一次产出 `darwin/arm64` 和 `windows/amd64` 两个 Program Bundle
- Windows Program Bundle 使用 `.zip`
- Program Bundle 根目录必须包含 `manifest.json`
- Program Bundle 不包含 `.cmd`、`systemd/`、空的 `data/`、空的 `run/`
- `make release-image` 产出新命名的 Linux 镜像 bundle
