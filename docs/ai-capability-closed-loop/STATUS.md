# Knot 智能能力闭环实施状态

> 本文件是跨 Agent 的状态交接入口。
> 开始任何阶段前，请先完整阅读 `ROADMAP.md`，再阅读本文件。

## 当前状态

- 当前阶段：阶段 0 已完成，等待进入阶段 1
- 最后完成阶段：阶段 0 - 应用数据目录与基线固定
- 当前 schema version：无
- 当前里程碑：个人年度总结闭环

## 已确认的代码基线

- 前端设置主要保存在 `src/services/settings.js` 的 `localStorage`。
- Electron 的 `knot-settings.json` 当前只用于部分 Electron 设置。
- Go 后端尚无 SQLite 依赖。
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

## 全局未决事项

- 阶段 1 默认采用纯 Go SQLite 驱动；必须在实际构建验证后确认，若更换需记录 ADR。
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
