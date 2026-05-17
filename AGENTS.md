# AGENTS.md

## 1. 项目概览

`agent-container-hub` 现在包含三条主线能力：

- 会话管理：`create / execute / stop / list / query / get / executions`
- 环境注册：维护可创建会话的命名环境
- 镜像构建：基于环境保存的 Dockerfile 触发本地构建与 smoke check

同一个 Go 服务同时托管 API 和内置管理站页面。

## 2. 技术栈

- Go 1.26
- `net/http`
- `modernc.org/sqlite`
- `docker` / `podman` CLI
- `log/slog`
- `embed` 静态页面

## 3. 核心架构

- `cmd/agent-container-hub`
  - 加载配置，初始化 runtime/store/services，启动 HTTP 服务
- `internal/api`
  - sandbox 与 httpserver 之间共享的请求/响应类型
- `internal/config`
  - 环境变量加载与路径归一化
- `internal/httpserver`
  - API 路由、鉴权、登录页与管理站页面托管
- `internal/model`
  - environment / session / execution / build job 领域模型与 clone/校验辅助函数
- `internal/sandbox`
  - `SessionService`
  - `EnvironmentService`
  - `BuildService`
- `internal/runtime`
  - 容器生命周期和镜像构建的 CLI 适配
- `internal/store`
  - 运行态 SQLite 持久化，包含 `sessions`、`session_executions`、`build_jobs`
- `configs/environments`
  - YAML 维护的 environment / image 配置

## 4. 主要模型

- `model.Session`
  - `environment_name`、镜像、rootfs、快照化 env/mount/resource
- `model.Environment`
  - `name`、`image_repository`、`image_tag`
  - `default_cwd`、`default_env`、`agent_prompt`
  - `mounts`、`resources`、`default_execute`
  - `enabled`
  - `build`
- `model.SessionExecution`
  - 执行命令、参数、cwd、超时、stdout/stderr、截断标记、耗时、起止时间
- `model.ExecutePreset`
  - environment 级别的默认命令、参数、cwd、超时模板
- `model.BuildJob`
  - 构建状态、输出、错误、起止时间

## 5. 主要接口

- `POST /api/auth/login`
- `POST /api/auth/logout`
- `POST /api/sessions/create`
- `GET /api/session-create/template`
- `GET /api/sessions`
- `GET /api/sessions/query`
- `GET /api/sessions/{id}`
- `POST /api/sessions/{id}/execute`
- `GET /api/sessions/{id}/executions`
- `POST /api/sessions/{id}/stop`
- `POST /api/environments`
- `GET /api/environments`
- `GET /api/environments/{name}`
- `PUT /api/environments/{name}`
- `GET /api/environments/{name}/agent-prompt`
- `GET /api/environments/{name}/files`
- `GET /api/environments/{name}/files/{path...}`
- `PUT /api/environments/{name}/files/{path...}`
- `POST /api/environments/{name}/build-jobs`
- `GET /api/build-jobs/{id}`
- `GET /api/build-jobs/{id}/events`
- `GET /`
- `GET /app`
- `GET /sessions`
- `GET /environments`
- `GET /login`

## 6. 开发要点

- 会话创建必须引用已注册且启用的环境
- 环境更新不会回写已有 session 的配置快照
- `CONFIG_ROOT/environments` 是 environment 配置唯一真相来源
- `BUILD_ROOT` 用于平台托管的 Dockerfile 构建上下文
- `SESSION_MOUNT_TEMPLATE_ROOT` 用于为 UI/API 生成 session mount 建议模板
- `agent_prompt` 与 `default_execute` 是 environment 的一部分，可供外部 agent/UI 读取
- 构建成功后可选执行 smoke check；失败会保留失败的 `BuildJob`
- 只有 `ENABLE_EXEC_LOG_PERSIST=true` 时才会持久化 `SessionExecution` 并开放历史日志查询价值
- 管理站鉴权使用单一管理员 token；API 支持 Bearer，页面使用登录 cookie

## 7. 已知约束

- 启动时强制执行 `docker info` / `podman info` 校验容器 daemon 可达，未通过会 Fatal 退出；不再提供 local 回退模式
- 当前环境构建只支持平台托管 Dockerfile，不支持 Git 仓库拉取
- 当前 UI 是轻量嵌入式管理站，不是独立前端工程
- 当前镜像构建仍依赖宿主机容器引擎权限和 registry 登录状态

## 8. 发布规范

- Program Bundle 根目录固定包含 `manifest.json`、`.env.example`、`README.txt`、当前平台 `deploy/start/stop`、`scripts/program-common.*`、`backend/agent-container-hub(.exe)`、`configs/environments/`
- `manifest.json` 中保留 `frontend` 字段，用于声明当前项目前端是内嵌托管：入口 `/`、静态资源前缀 `/ui/`、可直接访问服务端口、无需宿主 Node HTTP server 托管
- Program Bundle 不预置空的 `data/`、`run/`；由 `deploy` / `start` 首次运行时创建
- Program Bundle 对外命名为 `agent-container-hub-vX.Y.Z-<os>-<arch>.<ext>`，其中 Windows 使用 `.zip`
- Image Bundle 对外命名为 `agent-container-hub-image-vX.Y.Z-linux-<arch>.tar.gz`
- 本项目为对齐通用发布规范而保留两点裁剪：UI 继续内嵌在 Go 二进制中；`configs/environments/` 继续随 Bundle 交付
