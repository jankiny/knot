# Knot 智能能力闭环实施状态

> 本文件是跨 Agent 的状态交接入口。
> 开始任何阶段前，请先完整阅读 `ROADMAP.md`，再阅读本文件。

## 当前状态

- 当前阶段：阶段 1 已完成，等待进入阶段 2
- 最后完成阶段：阶段 1 - 后端资料源注册表
- 当前 schema version：1
- 当前里程碑：个人年度总结闭环

## 已确认的代码基线

- 前端现有业务设置仍主要保存在 `src/services/settings.js` 的 `localStorage`；资料源注册表已改为后端 SQLite 持久化。
- Electron 的 `knot-settings.json` 当前只用于部分 Electron 设置。
- Go 后端使用纯 Go `modernc.org/sqlite v1.54.0`，数据库位于 `appdata.Resolve()` 目录下的 `knot.db`。
- `backend/api/archive_scan.go` 主要识别包含 `工作记录.md` 的目录。
- `backend/api/work_report.go` 由请求传入扫描路径。
- `backend/api/archive_ai_search.go` 由请求传入归档路径。
- `backend/api/path_safety.go` 当前混合了本地访问限制和 AI 暴露限制。
- 工作区在创建本计划时已有用户修改：`README.md`。后续 Agent 不得覆盖或回退该修改。

## 阶段记录

## Stage 0 - 应用数据目录与基线固定

- 状态：completed
- 分支：`dev`
- 提交 SHA：`cc316f1`（实现前基线：`90dca300530945cc408dfe034ec7542cab13d0a9`）
- 完成日期：2026-07-23

### 已实现

- Electron 启动 Go 后端时，将 `app.getPath("userData")` 作为 `KNOT_DATA_DIR` 注入子进程环境，并保留其他父进程环境变量。
- 新增 `backend/appdata` 集中式数据目录解析包；后端启动时先完成解析，失败则拒绝启动。
- `KNOT_DATA_DIR` 可为独立启动的开发/测试后端显式指定绝对目录；未设置时回退到 `os.UserConfigDir()/Knot`。
- 相对路径覆盖会被拒绝，输入路径经 `filepath.Clean` 规范化。
- 后端日志只记录解析来源，不输出数据目录、凭据或用户文件内容。
- `.gitignore` 已覆盖仓库内 `/data/_test_*/` 测试运行目录。
- 已增加 Electron 环境注入测试、Go 数据目录覆盖/默认/错误测试和 Windows 路径规范化测试。

### 数据库与 migration

- schema version：无。
- 新增/变化：本阶段未引入 SQLite、数据库文件或 migration。

### 公共接口

- Go：`appdata.Resolve() (appdata.Directory, error)`。
- `appdata.Directory.Path`：清理后的绝对应用数据目录。
- `appdata.Directory.Source`：`environment` 或 `system_config`，供诊断使用。
- 环境变量：`KNOT_DATA_DIR`。Electron 正常启动时固定为 `app.getPath("userData")`；开发和测试直接运行后端时可显式覆盖。
- 默认行为：环境变量为空时使用系统用户配置目录的 `Knot` 子目录；解析函数本身不创建目录。

### 关键决策

- 数据目录解析集中在 `backend/appdata`，后续 SQLite、索引和审计不得自行推导应用数据路径。
- 显式覆盖必须是绝对路径，避免数据库位置随进程工作目录变化。
- 本阶段没有改变设置存储、扫描行为、安全策略或任何 AI 公共语义，因此不需要 ADR。

### 测试

- `pnpm test -- --run`：通过，5 个测试文件、15 个测试全部通过。
- `pnpm run build`：通过，Vite 生产构建完成；存在上游 CJS Node API 弃用警告。
- `$env:TEMP/TMP=<repo>/data/_test_appdata; conda run -n go go test ./...`：通过，`api`、`appdata`、`mail` 包通过。
- `$env:TEMP/TMP=<repo>/data/_test_appdata; conda run -n go go build -buildvcs=false ./...`：通过。
- `node --check electron/main.js`：通过。
- `node --check electron/preload.js`：通过。
- `node --check electron/backendProcess.js`：通过。
- 首次未覆盖 `TEMP/TMP` 的定向 Conda 测试未进入 Go 测试：当前 Windows 系统临时目录不可用；改用仓库内已忽略目录后通过。

### 已知问题

- 未启动 Electron 或 Go 前台监听服务做进程级验证；遵循有限测试要求，Electron 子进程环境由单元测试覆盖。
- 当前仅解析数据目录，不创建数据库或持久化数据；这些属于阶段 1。
- Vite 输出 CJS Node API 弃用警告，不影响本阶段测试与构建结果。

### 下一阶段入口

- 必须先阅读：`ROADMAP.md`、本文件、`backend/appdata/appdata.go`、`electron/backendProcess.js`、`src/services/settings.js`、`src/hooks/useTaskDirectories.js`。
- 可复用接口：`appdata.Resolve()`、`appdata.EnvDataDir`、`appdata.Directory`。
- 不要改动：`KNOT_DATA_DIR` 的 Electron 注入语义、绝对路径要求、系统配置目录回退；不要让 storage/source 功能再次自行推导数据目录。

## Stage 1 - 后端资料源注册表

- 状态：completed
- 分支：`dev`
- 提交 SHA：未提交（实现前基线：`bfa10248c845f7cbcb088e6a68c636494f099bd0`）
- 完成日期：2026-07-23

### 已实现

- 新增后端 SQLite 生命周期与版本化 migration；后端启动时使用 `appdata.Resolve()` 的结果创建或打开 `knot.db`，migration 失败会拒绝启动。
- 新增 `source_roots` repository 和 registry，支持持久化 CRUD、输入枚举校验、缺失/离线/无权限可用性记录，以及读取时的可用性刷新。
- 资料源路径支持 `~` 当前用户目录展开、绝对路径要求、`filepath.Clean` 规范化和持久化 `path_key` 去重；Windows 下路径 key 忽略大小写并统一斜杠。
- 缺失目录仍可登记为 `availability=missing`，不会因为盘符、移动盘或网络目录当前不可用而阻止后端启动。
- legacy import 覆盖当前工作目录、扫描目录、部门归档和项目归档；部门/项目的 ID、名称和 scope 关系会保留，相同规范化路径只创建一次。
- React 启动时自动提交 legacy 资料源白名单；设置页提供“同步现有设置”、资料源列表、刷新、添加、编辑和删除，并提供专用“添加工作日志”入口。
- 删除资料源只删除注册记录，不删除任何目录或文件。
- 本阶段没有扫描目录、读取资料正文、创建全文索引、调用 AI 或修改旧工作报告/归档接口。

### 数据库与 migration

- schema version：1。
- 数据库：`<appdata.Resolve().Path>/knot.db`；文件名由 `backend/storage` 集中定义，调用方不得自行推导。
- migration：`backend/storage/migrations/001_source_roots.sql`。
- schema version 表：`schema_migrations(version, applied_at)`。
- 新增 `source_roots`：`id`、名称、类型、规范化路径与唯一 `path_key`、scope、启用/递归标记、本地/AI 访问级别、可用性及内部时间戳。
- 索引：`enabled + kind`、`scope_type + scope_id`；`path_key` 唯一。
- migration 在单一事务中执行；任何 SQL 或版本记录失败都会整体回滚，不留下部分 schema。
- 没有自动 down migration。需要恢复时应先退出 Knot，再恢复升级前的 `knot.db` 备份；数据库损坏且无备份时可移走数据库后重启并重新同步 legacy/手工资料源。此操作不会删除原始资料文件，但会丢失数据库内的手工注册记录。

### 公共接口

- Go：`storage.Open(ctx, appdata.Directory) (*sql.DB, error)`、`storage.CurrentSchemaVersion`。
- Go：`sources.NewRepository(db)`、`sources.NewRegistry(repository)`，registry 提供 `List/Get/Create/Update/Delete/ImportLegacy`。
- `GET /api/source-roots`：返回 `{ "source_roots": [...] }`。
- `POST /api/source-roots`：创建资料源，成功返回 `201`。
- `GET /api/source-roots/{id}`：读取单个资料源。
- `PUT /api/source-roots/{id}`：完整更新资料源。
- `DELETE /api/source-roots/{id}`：只删除注册记录，成功返回 `204`。
- `POST /api/source-roots/import-legacy`：幂等导入现有前端资料源设置。
- `SourceRoot` JSON：`id`、`name`、`kind`、`path`、`scope_type`、`scope_id`、`scope_name`、`enabled`、`recursive`、`local_access`、`ai_access`、`availability`；内部 `path_key` 和时间戳不对外暴露。

### 关键决策

- 按 ROADMAP 使用不依赖 CGO 的 `modernc.org/sqlite v1.54.0`；Windows 测试、Windows 项目打包编译和 Linux/amd64 `CGO_ENABLED=0` 交叉构建均通过，因此无需新增 ADR。
- `storage.Open` 只接受 `appdata.Resolve()` 产生的目录对象，延续阶段 0 的单一数据目录来源。
- 当前阶段只持久化 SourceRoot 声明值，不实现阶段 2 的有效策略优先级、敏感路径覆盖或安全相对路径解析。
- 可用性检测只执行目录级 `os.Stat`，不枚举文件，也不读取正文；列表和单项读取会刷新发生变化的状态。
- legacy import 冲突时保留已存在的资料源，不覆盖用户后来编辑的名称、权限或类型。
- 前端 legacy payload 只包含 `folderPath`、`scanPath` 及部门/项目的 `id/name/archivePath`；不会发送邮箱密码、AI Key、窗口或更新设置。
- 现有 `localStorage` 不删除：旧页面继续使用原设置，资料源注册表与现有工作流保持兼容。

### legacy 导入范围

- `folderPath` → `kind=current_work`、`scope_type=global`。
- `scanPath` → `kind=active_work`、`scope_type=global`。
- 部门 `archivePath` → `kind=work_archive`、`scope_type=department`，保留部门 ID/名称。
- 项目 `archivePath` → `kind=work_archive`、`scope_type=project`，保留项目 ID/名称。
- 未迁移：邮箱配置、窗口与标题设置、更新通道、AI 模型与密钥、命名规则、SOP、默认部门/项目、`useYearFolder` 等现有工作流设置。

### 测试

- `pnpm test -- --run`：通过，6 个测试文件、17 项测试。
- `pnpm run build`：通过；保留 Vite CJS Node API 弃用警告和既有大 chunk 警告。
- `$env:TEMP/TMP=<repo>/data/_test_stage1; conda run -n go go test ./...`：通过，`api`、`appdata`、`mail`、`sources`、`storage` 包通过。
- `$env:TEMP/TMP=<repo>/data/_test_stage1; conda run -n go go build -buildvcs=false ./...`：通过。
- `$env:TEMP/TMP=<repo>/data/_test_stage1; pnpm run backend:build:win`：通过，生成 Electron 使用的 Windows 后端可执行文件。
- `GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -buildvcs=false`：通过，临时产物位于已忽略的 `data/_test_stage1`。
- `node --check electron/main.js`：通过。
- `node --check electron/preload.js`：通过。
- `node --check electron/backendProcess.js`：通过。
- `git diff --check`：通过。

### 已知问题

- 当前 Windows 系统临时目录仍不可用；Conda/Go 验证需将 `TEMP/TMP` 指向已忽略的 `data/_test_stage1`。
- 未启动 Electron 或 Go 前台监听服务做进程级验证；REST 使用 `httptest`，前端使用单元测试和生产构建验证。
- Linux 验证为 Windows 上的 Linux/amd64 纯 Go 交叉构建，未在实际 UOS/Linux 设备运行。
- 可用性按创建、更新和读取时的 `os.Stat` 刷新，没有后台 watcher；后台扫描属于阶段 3。
- 旧工作报告、归档和资料检索接口仍按原契约接收路径；阶段 1 只建立注册表，不提前改造后续上下文或策略流程。
- Vite 仍报告 CJS Node API 弃用和既有大 chunk 警告。

### 下一阶段入口

- 必须先阅读：`ROADMAP.md`、本文件、`backend/sources/`、`backend/storage/`、`backend/api/source_roots.go`、`backend/api/path_safety.go`、`backend/api/work_record.go`。
- 可复用接口：`sources.SourceRoot`、`sources.Registry`、`sources.NormalizePath`、`sources.DetectAvailability`、`api.Dependencies.SourceRegistry`。
- 数据库规则：不要修改 migration 001；任何 schema 变化新增连续 migration，并保持事务和 `schema_migrations` 版本记录。
- 阶段 2 应基于 `source_root_id + relative_path` 增加统一策略和安全路径解析；不要建立索引、读取新增正文或调用 AI。
- 不要改动：`KNOT_DATA_DIR`/`appdata.Resolve()` 语义、SourceRoot JSON 字段含义、legacy 路径接口兼容性；不要把本地访问和 AI 暴露重新混成一个布尔值。

## 全局未决事项

- 阶段 2 需要明确策略 reason code、敏感路径 deny 覆盖与 Windows junction/symlink 越界测试。
- 阶段 5 已确定新增独立“个人总结”页面，并复用现有工作报告基础组件。
- 阶段 6 需要分别为 DOCX 和 PDF 解析依赖记录 ADR。

## 固定验证命令

```powershell
pnpm test -- --run
pnpm run build
conda run -n go go test ./...
conda run -n go go build -buildvcs=false ./...
node --check electron/main.js
node --check electron/preload.js
```
