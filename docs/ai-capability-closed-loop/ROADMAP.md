# Knot 智能能力闭环：多阶段开发与 Agent 交接方案

> 状态：规划基线
> 目标版本：从“用户手动选择路径的 AI 功能”演进为“由 Knot 自动发现资料、受控读取、生成可追溯结果，并最终支持可确认、可撤销文件操作”的本地智能工作台。
> 首个验收场景：用户不输入任何路径，仅说明“根据过去一年工作资料生成约 1500 字个人工作总结大纲”，Knot 自动发现日志、当前工作、项目与归档资料，展示资料范围，过滤禁止发送内容，生成带证据来源和信息缺口提示的大纲。

---

## 1. 如何使用本文档

本文档是所有阶段 Agent 的共同上下文和阶段边界。每位 Agent 开始工作前必须：

1. 完整阅读本文档，不只阅读自己负责的阶段。
2. 阅读仓库根目录的 `README.md`、`AGENTS.md`（若存在）及上一阶段留下的 `STATUS.md`。
3. 运行 `git status --short`，保留用户和其他 Agent 的已有修改。
4. 只实现当前被分配的阶段，不提前实现后续阶段。
5. 不擅自改变本文档定义的安全边界、数据含义和公共 API；确需改变时先记录 ADR。
6. 完成后更新同目录的 `STATUS.md`，写明修改、测试、未决问题和下一阶段入口。

推荐一个阶段由一个 Agent 完整负责，阶段之间顺序交接。不要让两个 Agent 同时修改同一组核心后端文件。

### 推荐交接方式

- 每个阶段使用独立分支：`codex/knot-ai-stage-N-*`。
- 阶段完成并验收后再让下一阶段基于其结果继续。
- 下一阶段以提交 SHA、`STATUS.md` 和测试结果为准，不依赖聊天上下文。
- 如果暂不使用 Git 提交，也必须保证 `STATUS.md` 与工作区实际状态一致。

---

## 2. 当前代码基线

当前 Knot 已具备：

- Electron + React + Go/chi 的本地桌面架构。
- 当前工作目录、扫描目录、部门归档目录和项目归档目录配置。
- 基于 `工作记录.md` 的任务扫描、归档、恢复和工作报告生成。
- AI 模型配置和 OpenAI Chat Completions 兼容调用。
- 归档资料的本地候选筛选与 AI 相关性判断。
- `ai_access` 工作记录字段和基于路径关键字的初步隐私过滤。

当前关键限制：

- 工作目录、部门和项目配置主要保存在前端 `localStorage`，Go 后端不能独立发现全部资料源。
- Electron 的 `knot-settings.json` 只保存部分 Electron 设置，不是统一项目数据库。
- 扫描器主要识别包含 `工作记录.md` 的目录，不能统一索引日志、普通 Markdown、Word 和 PDF。
- `/api/archive/ai-search` 和 `/api/report/work/scan` 仍由调用方传入绝对路径。
- `path_safety.go` 把“本地能否处理”和“能否发送给 AI”混在一起。
- AI 调用是一次性文本/JSON生成，还没有受控的工具调用循环。
- 没有统一证据清单、AI 审计、索引状态和生成结果到来源的追溯关系。

开始各阶段前需要重点阅读：

- `src/services/settings.js`
- `src/hooks/useTaskDirectories.js`
- `src/components/WorkReport.jsx`
- `src/components/ArchiveAiSearch.jsx`
- `backend/api/path_safety.go`
- `backend/api/archive_scan.go`
- `backend/api/archive_ai_search.go`
- `backend/api/work_record.go`
- `backend/api/work_report.go`
- `backend/api/report_ai.go`
- `electron/backendProcess.js`

---

## 3. 不可破坏的架构原则

这些原则适用于所有阶段。

### 3.1 Knot 是执行者，模型是受限规划者

- 文件系统访问、路径解析、索引、权限判断和文件操作必须由 Knot 本地代码完成。
- 模型不能获得任意 Shell、PowerShell 或操作系统文件 API。
- 模型不能用任意绝对路径请求读取或修改文件。
- 面向模型的对象应使用 `source_root_id`、`document_id`、`evidence_id` 和相对路径。

### 3.2 本地权限与 AI 暴露权限分离

至少保留两个独立维度：

```text
local_access: none | read | read_write
ai_access: none | metadata | content
```

含义：

- `local_access=read_write, ai_access=none`：Knot 可以在本机管理文件，但任何元数据和内容都不能发送给云端模型。
- `ai_access=metadata`：只能发送允许字段中的文件名、类型、日期和受控标签，不能发送正文或正文摘要。
- `ai_access=content`：可发送经过候选筛选、大小限制和用户任务授权的内容。

路径中的 `NoAI`、`Private`、`隐私` 等标记只能作为默认安全策略或更严格覆盖，不能继续等同于“Knot 本地完全不能管理”。

### 3.3 文件是事实来源，索引可重建

- 原始工作记录、日志和业务文件仍是事实来源。
- SQLite 保存资料源、项目关系、文件元数据、索引状态、摘要缓存和审计记录。
- 删除 SQLite 索引后应能通过重新扫描恢复，不应导致原始文件丢失。
- 第一阶段不把所有原文复制进数据库。

### 3.4 先发现、再预览、后生成

任何可能发送内容给云端模型的任务必须形成：

```text
意图 → 本地发现 → 权限过滤 → 候选清单 → 用户确认/任务授权 → AI 调用 → 证据追溯
```

首个年度总结闭环不允许跳过资料范围预览。

### 3.5 只基于证据写作

- 生成内容必须能回溯到一个或多个 `evidence_id`。
- 不确定的个人职责、数字、获奖和团队成果必须标记为缺口或待确认。
- 模型失败时返回错误，不用虚构内容做静默兜底。

### 3.6 文件修改必须另设闭环

- 只读智能闭环完成前，不引入 AI 文件修改。
- 后续文件修改采用“生成计划 → 本地校验 → 用户确认 → 执行 → 审计 → 撤销”。
- 第一版文件操作不支持删除、覆盖和任意脚本。

---

## 4. 目标架构

```text
┌─────────────────────────────────────────────────────────────┐
│ React UI                                                    │
│ 提示词 / 时间范围 / 资料预览 / 证据查看 / 执行确认          │
└──────────────────────────┬──────────────────────────────────┘
                           │ REST
┌──────────────────────────▼──────────────────────────────────┐
│ Go Backend                                                  │
│                                                             │
│ Source Registry ─ Policy Engine ─ Local Index               │
│        │               │             │                      │
│        └──────── Context Resolver ───┘                      │
│                        │                                    │
│                  Evidence Manifest                          │
│                        │                                    │
│                    AI Gateway                               │
│                        │                                    │
│             Result + Evidence + Audit                       │
│                                                             │
│ 后期扩展：Tool Loop → Action Planner → Executor → Undo      │
└──────────────────────────┬──────────────────────────────────┘
                           │
┌──────────────────────────▼──────────────────────────────────┐
│ Filesystem + SQLite                                         │
│ 原始文件 / 资料源配置 / 可重建索引 / 摘要缓存 / 审计日志    │
└─────────────────────────────────────────────────────────────┘
```

Electron 负责将应用数据目录传给 Go 后端：

```text
KNOT_DATA_DIR = app.getPath("userData")
```

后端数据库建议位置：

```text
%APPDATA%/Knot/knot.db
```

测试数据库必须使用仓库内被 `.gitignore` 覆盖的有限目录，例如：

```text
data/_test_context_registry/
```

不要依赖 Windows 系统临时目录；不要用长期运行的服务器进程作为测试命令。

---

## 5. 稳定领域模型

后续阶段可以增加字段，但不应随意改变以下含义。

### 5.1 SourceRoot

```json
{
  "id": "root_journal",
  "name": "工作日志",
  "kind": "journal",
  "path": "C:/Workspace/30_Notes/10_Journal_日志",
  "scope_type": "global",
  "scope_id": null,
  "scope_name": null,
  "enabled": true,
  "recursive": true,
  "local_access": "read",
  "ai_access": "content",
  "availability": "online"
}
```

建议 `kind` 初始枚举：

- `current_work`
- `active_work`
- `work_archive`
- `journal`
- `reference`
- `private`
- `external_offline`

`availability`：

- `online`
- `offline`
- `missing`
- `permission_denied`
- `unknown`

`scope_type` 初始支持：

- `global`
- `department`
- `project`

它用于保留 legacy 部门/项目归档与资料源之间的关系。上下文发现不应从资料源显示名称中反向猜测项目。

### 5.2 IndexedDocument

```json
{
  "id": "doc_xxx",
  "source_root_id": "root_journal",
  "relative_path": "周/2026/2026.W03 工作周报.md",
  "document_type": "journal_weekly",
  "title": "2026.W03 工作周报",
  "task_date": "2026-01-12",
  "modified_at": "2026-01-16T20:15:00+08:00",
  "project_id": null,
  "content_hash": "sha256:...",
  "index_status": "ready",
  "ai_access_effective": "content"
}
```

### 5.3 EvidenceItem

`EvidenceItem` 是送入 AI 前的最小证据单元，不等同于整个文件。

```json
{
  "id": "evidence_xxx",
  "document_id": "doc_xxx",
  "source_type": "journal_weekly",
  "title": "2026.W03 工作周报",
  "date": "2026-01-12",
  "project": "",
  "excerpt": "完成……推进……",
  "reason": "时间范围匹配；包含完成事项",
  "ai_access_effective": "content"
}
```

### 5.4 ContextManifest

```json
{
  "id": "context_xxx",
  "task_type": "personal_annual_summary",
  "period_start": "2025-07-23",
  "period_end": "2026-07-23",
  "sources": [],
  "evidence": [],
  "excluded": [],
  "unavailable_sources": [],
  "estimated_input_tokens": 0,
  "requires_confirmation": true
}
```

### 5.5 权限优先级

有效策略按“更具体且更严格者优先”：

```text
系统硬限制
  > 路径安全标记产生的 deny
  > 文件/工作记录显式覆盖
  > 项目策略
  > 资料源策略
  > 系统默认值
```

任何层级的 `ai_access=none` 都不能被下层或模型放宽。

---

## 6. 公共 API 方向

最终 API 采用资源 ID，不由 AI 或普通业务页面传任意路径。

```http
GET    /api/source-roots
POST   /api/source-roots
PUT    /api/source-roots/{id}
DELETE /api/source-roots/{id}
POST   /api/source-roots/{id}/scan

POST   /api/context/discover
GET    /api/context/{id}
POST   /api/context/{id}/generate

GET    /api/ai-runs/{id}
GET    /api/ai-runs/{id}/sources
```

阶段 8 再增加：

```http
POST   /api/actions/plan
GET    /api/actions/plans/{id}
POST   /api/actions/plans/{id}/execute
POST   /api/actions/plans/{id}/undo
```

旧接口在迁移期间保持兼容，不允许一次阶段改动破坏现有日报、周报、归档和邮件流程。

---

## 7. 多阶段实施路线

## 阶段 0：应用数据目录与基线固定

### 目标

让 Go 后端获得稳定、可测试的应用数据目录，为 SQLite 和后续审计提供基础；记录现有测试基线。

### 主要任务

1. Electron 启动后端时传入 `KNOT_DATA_DIR=app.getPath("userData")`。
2. Go 增加集中式应用数据目录解析，不允许各功能自行推导路径。
3. 开发和测试环境支持显式覆盖数据目录。
4. 为数据目录解析增加 Windows 路径测试。
5. 创建 `docs/ai-capability-closed-loop/STATUS.md` 并记录基线测试。
6. 如需测试运行目录，使用 `data/_test_*`，同时确保 `.gitignore` 覆盖。

### 预计修改范围

- `electron/backendProcess.js`
- `backend/main.go`
- 新增 `backend/appdata/` 或等价包
- `.gitignore`（仅在缺少测试运行目录规则时）
- `docs/ai-capability-closed-loop/STATUS.md`

### 不做

- 不引入 SQLite。
- 不修改当前设置存储。
- 不增加 AI 功能。
- 不修改任何文件扫描行为。

### 验收标准

- Electron 启动的后端能解析到用户数据目录。
- 单元测试可以通过显式环境变量使用仓库本地测试目录。
- 不打印 API Key、邮箱密码或用户文件内容。
- 现有前后端测试与构建基线有明确记录。

### 交接物

- 数据目录解析 API 说明。
- 环境变量和默认行为说明。
- 测试命令、结果和已知失败。

---

## 阶段 1：后端资料源注册表

### 目标

建立 Knot 自己可查询的资料源数据库，使后端不再依赖用户在每个 AI 请求里提供绝对路径。

### 主要任务

1. 引入 SQLite 和版本化 migration。
2. 建立 `source_roots` 表。
3. 实现 SourceRoot repository 和 REST CRUD。
4. 增加路径规范化、重复路径检测和可用性检测。
5. 增加 legacy import：
   - 当前 `folderPath` → `current_work`
   - 当前 `scanPath` → `active_work`
   - 部门归档 → `work_archive`
   - 项目归档 → `work_archive`
6. legacy import 必须幂等，不重复创建相同路径。
7. 迁移范围只包括资料源；邮箱、窗口、更新通道、AI Key 等配置继续使用现有存储。
8. 为“工作日志”增加可配置的 `journal` 资料源。

### SQLite 决策

- 默认使用 `modernc.org/sqlite` 这一类不依赖 CGO 的纯 Go 方案。
- 负责本阶段的 Agent 必须实际验证 Windows 测试、Windows 打包编译和 Linux 构建；如验证失败，再通过 ADR 更换驱动。
- 不允许在没有记录迁移与打包影响的情况下临时改用依赖本机 C 编译环境的驱动。

- migration 必须在事务中执行，并记录 schema version。

### 预计修改范围

- 新增 `backend/storage/`
- 新增 `backend/sources/`
- 新增 `backend/api/source_roots.go`
- `backend/api/routes.go`
- `src/services/api.js`
- 资料源设置相关 React 组件
- legacy migration 逻辑

### 不做

- 不建立全文索引。
- 不读取资料正文。
- 不调用 AI。
- 不删除 `localStorage` 中无关设置。

### 验收标准

- 重启 Knot 后资料源仍然存在。
- legacy 配置首次导入成功，再次导入不重复。
- 相同路径不同大小写和斜杠形式在 Windows 上视为同一资料源。
- 缺失盘符或离线目录被记录为不可用，不导致应用启动失败。
- CRUD 和 migration 有后端测试。
- 已有工作报告和归档功能没有回归。

### 交接物

- 当前 schema version。
- migration 文件和回滚/恢复说明。
- SourceRoot JSON 契约。
- legacy 导入范围和未迁移字段清单。

---

## 阶段 2：统一策略引擎与安全路径解析

### 目标

把“本地管理权限”“AI 元数据权限”“AI 内容权限”从散落的路径判断中抽离为统一策略引擎。

### 主要任务

1. 实现 `local_access` 与 `ai_access` 枚举和校验。
2. 实现有效策略计算及更严格覆盖规则。
3. 将敏感路径标记改为 AI deny/default policy，而不是禁止 Knot 本地处理。
4. 建立统一路径解析：
   - 输入 `source_root_id + relative_path`
   - 拒绝 `..` 越界
   - 拒绝绝对相对路径混用
   - 防止符号链接和 Windows junction 越出根目录
   - Windows 路径大小写和分隔符规范化
5. 保留旧接口所需的兼容校验，但内部逐步复用策略引擎。
6. 增加“为什么被排除”的结构化 reason code。

### 建议 reason code

- `root_disabled`
- `root_offline`
- `local_access_denied`
- `ai_access_denied`
- `metadata_only`
- `sensitive_path_marker`
- `path_outside_root`
- `symlink_escape`
- `unsupported_type`
- `size_limit`

### 预计修改范围

- 重构 `backend/api/path_safety.go`
- 新增 `backend/policy/`
- 新增 `backend/safepath/`
- SourceRoot API
- 后端测试

### 不做

- 不创建索引。
- 不发送任何新增内容给 AI。
- 不增加文件移动和重命名。

### 验收标准

- `NoAI` 目录可以由本地文件功能读取/管理，但 AI 上下文发现返回 `ai_access_denied`。
- `metadata` 目录不会生成正文 evidence。
- 任何相对路径越界、链接越界和 root ID 伪造均被拒绝。
- 策略判定有表驱动测试，覆盖 Windows 路径。
- 原有照片项目 `ai_access: noai` 行为仍然安全。

### 交接物

- 策略优先级说明。
- reason code 清单。
- 路径解析函数的输入输出契约。
- 已保留的旧安全行为与已调整行为。

---

## 阶段 3：本地增量索引与首批资料适配器

### 目标

让 Knot 在本地知道“有哪些资料”，并能够从工作记录和日志中抽取结构化候选，而不是每次递归扫描整个目录。

### 第一批支持范围

1. `工作记录.md`
2. 日报 Markdown
3. 周报 Markdown
4. 普通 Markdown/TXT 的元数据
5. 其他文件仅建立文件名、类型、大小、修改时间元数据

### 主要任务

1. 建立 `indexed_documents` 表和必要索引。
2. 建立 source adapter 接口。
3. 复用现有 `readWorkRecord`，不要复制第二套工作记录解析逻辑。
4. 为日志识别日期、日报/周报类型、标题和有限正文片段。
5. 支持按单一资料源扫描和全部启用资料源扫描。
6. 使用路径、大小、修改时间和内容 hash 判断是否需要更新。
7. 删除或移动的文件标记为 stale/deleted，不立刻破坏历史审计关系。
8. 默认忽略：
   - `node_modules`
   - `.git`
   - `.venv`
   - `venv`
   - `__pycache__`
   - `dist`
   - `build`
   - `target`
   - `.cache`
   - 其他明确生成目录
9. 所有扫描必须有文件数、总字节、单文件大小和耗时上限。
10. 支持取消；不启动常驻文件监听器。

### 数据边界

- `ai_access=none` 的资料可以保存最小本地文件元数据，但第一版不在 SQLite 中保存正文片段。
- 索引阶段不调用云端模型。
- 原始全文不进入数据库；只保存可重建的受限摘要字段和结构化元数据。

### 预计修改范围

- 新增 `backend/indexer/`
- 新增 `backend/extractors/markdown.go`
- 新增索引 repository/migration
- 新增 scan API
- 可选新增索引状态 UI

### 不做

- 不解析 DOCX/PDF 正文。
- 不做 embedding。
- 不实现后台 watcher。
- 不生成年度总结。

### 验收标准

- 示例日志、当前工作和归档资料能形成统一 `IndexedDocument`。
- 重复扫描未变化目录时不重复解析全部文件。
- 文件移动、删除、离线和恢复均有明确索引状态。
- `NoAI` 正文不会被写入索引或测试日志。
- 扫描大目录不会无限运行；测试使用有限仓库本地样本。

### 交接物

- adapter 接口。
- 支持的文件类型和字段。
- 扫描限制默认值。
- index schema version 和重建命令/API。

---

## 阶段 4：上下文自动发现与 Evidence Manifest

### 目标

用户只描述任务和时间，不提供路径；Knot 自己根据资料源、索引和策略生成候选证据清单。

### 首个 Task Profile

```text
task_type = personal_annual_summary
```

默认资料源种类：

- `journal`
- `current_work`
- `active_work`
- `work_archive`

默认时间范围：

- 用户明确给出时使用用户范围。
- 用户只说“工作一年”时，由 UI 提供最近一年预览，用户可调整。
- 不允许模型自己把模糊时间永久写入设置。

### 主要任务

1. 实现 `POST /api/context/discover`。
2. 请求不得要求 `scan_paths` 或 `archive_paths`。
3. Context Resolver 查询启用资料源和索引。
4. 本地候选排序至少考虑：
   - 时间命中
   - 资料类型
   - 标题和项目关联
   - 明确完成/推进/成果表述
   - 最近活动
5. 应用策略引擎并产生 `excluded`、`unavailable_sources`。
6. 将文档切分为受限 `EvidenceItem`，避免发送整份大文件。
7. 估算输入 token，并按预算裁剪。
8. 保存 manifest，但不在 discover 阶段调用 AI。
9. manifest 过期或源文件 hash 变化时要求重新发现。

### 建议请求

```json
{
  "task_type": "personal_annual_summary",
  "query": "根据过去一年工作资料生成约 1500 字个人工作总结大纲",
  "period_start": "2025-07-23",
  "period_end": "2026-07-23"
}
```

### 建议响应重点

- 命中的资料源和数量
- 候选 evidence
- 排除数量与原因
- 离线/缺失资料源
- token 估算
- 是否需要确认

### 不做

- 不调用 AI。
- 不读取 DOCX/PDF 正文。
- 不执行文件修改。
- 不让模型参与权限判断。

### 验收标准

- 请求中不提供任何路径也能发现日志、当前工作和归档候选。
- `ai_access=none` 不出现在模型可用 evidence 中。
- `metadata` evidence 没有正文。
- 离线内网资料以缺口返回，不导致整个任务失败。
- 相同输入和相同索引产生稳定、可解释的候选结果。
- 返回结果能说明每条证据为何入选或被排除。

### 交接物

- ContextManifest 与 EvidenceItem 契约。
- 排序规则与 token 裁剪规则。
- manifest 失效条件。
- 示例年度总结 discover 响应。

---

## 阶段 5：个人年度总结只读智能闭环

### 目标

完成第一个可供真实用户使用的端到端智能能力闭环。

### 用户流程

```text
输入总结需求
  → 自动发现资料
  → 查看命中/排除/离线资料
  → 调整时间或取消部分证据
  → 确认生成
  → 查看大纲、证据引用和信息缺口
  → 复制 Markdown
```

### 主要任务

1. 新增独立“个人总结”页面，复用工作报告的目录选择、日期和 Markdown 展示组件；不要把首个智能闭环直接塞进现有 `WorkReport` 状态机。
2. 默认提示词不包含路径。
3. 展示：
   - 已发现日志数
   - 已发现项目数
   - 资料源范围
   - 排除原因汇总
   - 离线内网资料提示
   - token 估算
4. 用户点击生成后，后端按 manifest ID 重新校验证据和策略。
5. AI Gateway 只接收允许的 EvidenceItem。
6. 模型输出必须包含：
   - 总结标题
   - 约 1500 字的篇幅分配
   - 分章节大纲
   - 可核实成果
   - 信息不足/待补充项
7. 结果中为章节或关键事实保留 evidence ID。
8. 保存 AI run 审计：
   - 模型配置 ID
   - task type
   - evidence ID
   - 输入/输出 token（接口可得时）
   - 时间
   - 成功/失败
9. 审计不保存 API Key，不复制禁止保存的原始敏感正文。

### Prompt 约束

- 只根据证据写作。
- 团队成果不能自动改写为个人成果。
- 不明确的数字、排名、奖项和个人分工进入 `missing_information`。
- 输出结构化 JSON，再由本地代码生成 Markdown。
- 生成失败时保留 manifest，允许重试，不重新全盘扫描。

### 不做

- 不生成完整 1500 字正文，首版只生成大纲。
- 不读取任意用户未登记路径。
- 不支持 AI 修改文件。
- 不实现通用多轮 Agent。

### 验收场景

用户只输入：

```text
我已经工作一年了，请根据现有工作资料生成一份 1500 字左右个人工作总结大纲。
```

必须满足：

- 用户不复制任何路径。
- Knot 自动发现已登记日志、当前工作、项目和归档资料。
- 用户能在发送前看到资料范围。
- 禁止 AI 的资料不会发送。
- 输出覆盖主要工作主线。
- 对内网缺失资料和无法确认的个人分工给出明确提醒。
- 能查看关键结论来自哪些工作记录或日志。

### 预计修改范围

- 新增个人总结 React 组件与样式
- `src/services/api.js`
- 新增 context generate API
- AI run/audit repository
- AI prompt 与 JSON 校验
- 前后端测试

### 交接物

- 一份脱敏的端到端示例。
- 实际发给模型的数据结构样例。
- 审计记录字段。
- 已知内容覆盖盲区。

---

## 阶段 6：DOCX/TXT/PDF 抽取与分层摘要缓存

### 目标

补齐 Codex 示例中对工作报告、技术报告和近期项目材料的读取能力，同时控制 token。

### 实施顺序

1. TXT/Markdown 完整支持。
2. DOCX 段落和表格文本。
3. PDF 文本型文档。
4. 扫描型 PDF/OCR 仅作为后续可选能力。

### 主要任务

1. 建立 extractor version。
2. DOCX 优先使用 Go 可打包方案，不引入用户机器必须安装的 Python 运行时。
3. PDF 依赖选择前记录 ADR，评估 Windows/UOS 打包、许可证和二进制体积。
4. 提取后先进行本地段落筛选，再送 AI。
5. 建立分层摘要：
   - 文件级
   - 项目级
   - 月度级
   - 最终任务级
6. 摘要缓存键至少包含：
   - content hash
   - extractor version
   - prompt version
   - model ID
   - policy version
7. 源文件或策略变化时缓存失效。
8. 表格、人员名单和个人信息需要更严格过滤。

### 不做

- 不默认 OCR 图片和扫描件。
- 不读取图片像素。
- 不将整个 DOCX/PDF 无限制发送给模型。
- 不把摘要缓存用于绕过当前 AI 权限。

### 验收标准

- 工作报告 DOCX 能抽取项目目标、个人承担事项和成果相关段落。
- 大文件经过裁剪或分层摘要，不超过配置预算。
- 文档更新后旧摘要不会继续使用。
- `ai_access=metadata` 只返回元数据。
- 打包后的 Knot 不依赖开发机 Python 环境。

### 交接物

- extractor 支持矩阵。
- 依赖和许可证说明。
- 缓存键及失效规则。
- token 预算前后对比。

---

## 阶段 7：通用只读 AI 工具调用

### 目标

将年度总结的自动发现能力推广为通用的只读文件助手，但仍不允许文件修改。

### 初始工具

```text
list_sources()
search_evidence(query, source_kinds, period, limit)
get_evidence(evidence_ids)
get_project_history(project_id, period)
get_context_manifest(context_id)
```

### 工具约束

- 工具参数不接受绝对路径。
- 工具不能提升权限或改变 SourceRoot 策略。
- `get_evidence` 只能读取 Context Resolver 已批准的 evidence。
- 每轮对话设置最大工具调用次数、最大 evidence 数量和 token 预算。
- 模型产生的未知 tool name、非法参数和重复循环必须终止。
- 工具结果再次经过策略引擎，不信任旧缓存中的权限结论。

### 模型适配

- 在 AI 模型配置中增加 capability：
  - `json_output`
  - `tool_calls`
  - `streaming`
- 不假设所有 OpenAI 兼容服务都完整实现相同 Tool Calls 细节。
- 无 Tool Calls 能力的模型继续使用专用年度总结流程。

### 验收标准

- 用户可以问“我过去一年主要做了哪些 AI 项目”，不输入路径。
- 模型只能在批准的资料源和 evidence 中检索。
- 工具循环达到上限时安全停止并说明原因。
- 所有工具调用均进入本地审计。
- 现有日报、工作报告和归档 AI 功能保持可用。

### 交接物

- tool schema。
- 模型 capability 表。
- 工具循环状态机。
- 调用上限和错误分类。

---

## 阶段 8：文件操作计划、确认、执行与撤销

### 目标

在只读能力稳定后，增加类似 Codex 但更受限的本地文件管理闭环。

### 第一批操作

- 创建目录
- 移动文件/目录
- 重命名文件/目录
- 归档 Knot 任务

第一版不支持：

- 删除
- 覆盖现有文件
- 任意 Shell
- 修改权限策略
- 修改应用外未登记路径

### 工具分层

```text
propose_create_directory
propose_move
propose_rename
propose_archive
```

模型只能创建计划，不能调用执行接口。

### ActionPlan 必须包含

- 操作类型
- 来源 root ID 与相对路径
- 目标 root ID 与相对路径
- 预期文件数量和大小
- 冲突
- 跨盘复制提示
- 策略检查结果
- 可否撤销
- 反向操作
- 计划过期时间

### 执行规则

1. 生成计划时检查一次。
2. 用户确认执行前重新检查源文件状态、hash、目标冲突和权限。
3. 计划内容变化时原确认失效。
4. 执行过程写操作日志。
5. 部分失败必须返回逐项结果，不报告为全部成功。
6. 可撤销操作记录反向计划；撤销也需要冲突检查。

### 验收标准

- 模型不能直接执行文件修改。
- 未确认计划不会改变文件系统。
- 路径越界、链接越界和未登记根目录全部失败。
- 文件在预览后被外部修改时，旧计划不能执行。
- 移动和重命名可在无冲突时撤销。
- 不支持的删除/覆盖请求明确拒绝。

### 交接物

- ActionPlan schema。
- 计划状态机。
- 操作和撤销审计格式。
- Windows 跨盘与锁文件行为说明。

---

## 阶段 9：可靠性、安全与发布验收

### 目标

把前述能力从可用原型提升为可长期管理个人文件体系的版本。

### 重点

- 数据库 migration 失败恢复。
- 索引重建和损坏恢复。
- 离线盘、网络盘、盘符变化。
- 大目录、长路径、Unicode、大小写冲突。
- Windows junction、符号链接和文件锁。
- 扫描取消、应用退出和任务恢复。
- AI 超时、限流、无效 JSON、工具循环、部分响应。
- token 和月度预算硬限制。
- 审计导出和本地清理。
- 安装包升级后数据库兼容。
- 隐私回归测试。

### 发布门槛

- 所有 migration 有从旧版本升级测试。
- 关键安全行为有自动化测试。
- 端到端年度总结场景通过。
- 端到端文件计划/执行/撤销场景通过。
- Windows 安装包验证；Linux/UOS 至少完成构建验证。
- README 和用户设置说明更新。

---

## 8. 阶段依赖关系

```text
阶段 0：数据目录
  ↓
阶段 1：资料源注册表
  ↓
阶段 2：策略与安全路径
  ↓
阶段 3：本地索引
  ↓
阶段 4：上下文发现
  ↓
阶段 5：年度总结闭环  ← 第一里程碑
  ↓
阶段 6：DOCX/PDF 与分层摘要
  ↓
阶段 7：通用只读工具调用  ← 第二里程碑
  ↓
阶段 8：文件操作闭环       ← 第三里程碑
  ↓
阶段 9：可靠性与发布
```

阶段 5 完成后应先实际使用一段时间，再决定是否进入通用 Tool Calls。不要因为通用 Agent 更“智能”而跳过专用闭环的验收。

---

## 9. 每阶段统一验证命令

根据修改范围运行有限、可终止的命令：

```powershell
pnpm test -- --run
pnpm run build
conda run -n go go test ./...
conda run -n go go build -buildvcs=false ./...
node --check electron/main.js
node --check electron/preload.js
```

规则：

- Python 测试如出现疑似挂起，先使用有界超时和定向复现。
- 测试运行数据使用仓库内忽略目录，不依赖系统 `TemporaryDirectory`。
- 不用 Flask、Go HTTP 服务、Electron 开发进程等长期监听器作为“会自动结束”的测试。
- UI 行为优先通过单元测试、API handler 测试和构建检查验证；确需浏览器测试时管理并关闭自己启动的进程。
- 不修改或删除用户已有的运行数据进行测试。

---

## 10. Agent 完成定义

一个阶段只有同时满足以下条件才算完成：

- 阶段范围内功能已实现。
- 明确列出的“不做”内容没有被顺手加入。
- 新增公共类型、API 和 migration 有测试。
- 现有关键功能没有回归。
- 代码经过格式化。
- `STATUS.md` 已更新。
- 所有失败测试都有解释，不能只写“应该没问题”。
- 没有遗留明文密钥、真实私人资料或测试数据库。
- 没有把绝对用户路径写死到代码或测试。

---

## 11. STATUS.md 固定格式

每个阶段 Agent 完成后必须追加以下内容：

```markdown
## Stage N - 名称

- 状态：completed | partial | blocked
- 分支：
- 提交 SHA：
- 完成日期：

### 已实现

- ...

### 数据库与 migration

- schema version:
- 新增/变化:

### 公共接口

- ...

### 关键决策

- ...

### 测试

- `命令`：通过/失败

### 已知问题

- ...

### 下一阶段入口

- 必须先阅读：
- 可复用接口：
- 不要改动：
```

---

## 12. 可直接发送给各阶段 Agent 的启动提示词

```text
你负责 Knot 智能能力闭环的“阶段 N：<阶段名称>”。

开始前：
1. 完整阅读 docs/ai-capability-closed-loop/ROADMAP.md。
2. 阅读 docs/ai-capability-closed-loop/STATUS.md 和仓库 AGENTS.md。
3. 检查 git status 与当前 diff，保留用户和前序 Agent 的修改。
4. 阅读 ROADMAP 中当前阶段的目标、主要任务、不做、验收标准和交接物。

执行要求：
- 只实现阶段 N，不提前实施后续阶段。
- 不改变 ROADMAP 的安全边界和公共语义。
- 如果必须改变契约，先新增 ADR 并说明原因、方案和迁移影响。
- 为新增 API、migration、路径和策略逻辑补充测试。
- 使用有限测试命令，不启动预期不会退出的前台服务。

完成后：
1. 运行与改动相称的前端测试、Go 测试、构建和语法检查。
2. 更新 docs/ai-capability-closed-loop/STATUS.md。
3. 报告修改文件、测试结果、遗留风险和下一阶段入口。
```

---

## 13. 里程碑验收定义

### 里程碑一：个人年度总结闭环

对应阶段 0–5。用户不提供路径，Knot 自动发现资料、预览范围、过滤禁止内容、生成带证据和缺口的大纲。

### 里程碑二：通用只读文件助手

对应阶段 6–7。用户可以围绕已登记资料进行多轮问答，模型通过受限工具检索证据，不能任意读取磁盘。

### 里程碑三：受控文件管理助手

对应阶段 8–9。模型可以提出创建、移动、重命名和归档计划；用户确认后由 Knot 执行并审计，支持安全撤销。

---

## 14. 最终产品判断标准

Knot 的智能能力不以“模型能调用多少工具”为完成标准，而以以下闭环是否成立为标准：

1. Knot 知道资料在哪里。
2. Knot 知道哪些资料与当前任务相关。
3. Knot 知道哪些资料可以在本地读取、哪些可以发送给 AI。
4. 用户在发送或修改前能看到范围和影响。
5. AI 输出可以追溯到证据。
6. 信息不足时系统明确承认不足。
7. 文件修改由本地代码执行，并可审计、可撤销。

只有这七点同时成立，Knot 才从“带 AI 按钮的文件工具”变成真正的“智能文件管理闭环”。
