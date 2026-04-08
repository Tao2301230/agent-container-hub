# 双轨版本化发布：Program Bundle + Image Bundle

## 1. 目标与边界

这个项目的正式 release 分成两条产线：

- program bundle：宿主机程序部署包
- image bundle：容器镜像离线部署包

两类产物都输出到 `dist/release/`，版本号统一来自根目录 `VERSION`，格式固定为 `vX.Y.Z`。

## 2. 对外入口

主入口是 program bundle：

```bash
make release
```

它等价于：

```bash
make release-program
```

镜像 bundle 入口：

```bash
make release-image
```

支持的输入变量：

- `VERSION`
- `ARCH`

默认平台规则：

- `release` / `release-program`：固定打 `darwin` 和 `windows`
- `release-image`：固定打 `linux` 当前架构镜像 bundle

在 `arm64` 机器上，`release-image` 默认只打 `linux/arm64` 镜像，不做多架构合包。

## 3. Program Bundle

### 3.1 产物命名

```text
dist/release/agent-container-hub-program-vX.Y.Z-darwin-<arch>.tar.gz
dist/release/agent-container-hub-program-vX.Y.Z-windows-<arch>.tar.gz
```

### 3.2 默认行为

一次执行会生成两个程序 bundle：

- `darwin`
- `windows`

这两个 bundle 都包含：

- 可执行程序
- `.env.example`
- `README.txt`
- `configs/environments/`
- `data/rootfs/`
- `data/builds/`

按 OS 分开定制：

- `darwin` bundle 包含 `start.sh` / `stop.sh`
- `windows` bundle 使用 `.exe`，并包含 `release-scripts/windows/`
- 只有显式指定 `PROGRAM_TARGETS=linux` 时才会带 `systemd/agent-container-hub.service`

仓库内的 release 静态资产统一维护在 `scripts/release-assets/`：
- `scripts/release-assets/program/`：program bundle 资产
- `scripts/release-assets/image-bundle/`：image bundle 资产
- `scripts/release-windows-package.ps1`：Windows 打包入口

### 3.3 常见用法

```bash
make release VERSION=v1.0.0
make release-program VERSION=v1.0.0 ARCH=arm64
PROGRAM_TARGETS=windows make release-program VERSION=v1.0.0 ARCH=amd64
```

## 4. Image Bundle

### 4.1 产物命名

```text
dist/release/agent-container-hub-image-bundle-vX.Y.Z-linux-<arch>.tar.gz
```

### 4.2 默认行为

- 固定构建 Linux 镜像
- 默认使用当前机器架构
- 先导出为 `docker save` 生成的镜像 tar.gz
- 再把镜像文件、`.env.example`、`configs/environments/`、`data/` 空目录与加载脚本组装进单个总包

### 4.3 常见用法

```bash
make release-image VERSION=v1.0.0
make release-image VERSION=v1.0.0 ARCH=arm64
```

解压后会包含：

- `.env.example`
- `README.txt`
- `load-image.sh`
- `images/agent-container-hub-image-vX.Y.Z-linux-<arch>.tar.gz`
- `configs/environments/`
- `data/rootfs/`
- `data/builds/`

导入方式：

```bash
tar -xzf dist/release/agent-container-hub-image-bundle-v1.0.0-linux-arm64.tar.gz
cd agent-container-hub
./load-image.sh
```

## 5. 验证重点

- `make release` 与 `make release-program` 行为一致
- `make release` 一次产出 `darwin` 和 `windows` 两个程序 bundle
- `darwin` bundle 不含 `systemd`
- `windows` bundle 包含 `.exe` 和 Windows 脚本
- `make release-image` 产出 Linux 镜像 bundle，总包内含离线镜像 tar.gz 和运行配置
