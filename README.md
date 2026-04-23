# Knot 绳结

办公数字化工具 - 以邮件和工作记录为中心的任务材料管理桌面应用。

Knot 可以从邮件或手动输入快速创建标准工作文件夹，沉淀 `工作记录.md`，并围绕部门归档和 AI 日报生成形成一套轻量工作流。

## 功能特点

- **邮件列表**：连接 IMAP 邮箱，查看收件箱邮件；支持本地缓存优先显示，并在后台静默刷新最新邮件。
- **邮件生成任务**：从邮件一键生成工作文件夹，保存邮件正文和附件，并记录来源信息。
- **快速创建**：不依赖邮件，手动输入工作内容即可创建标准任务目录。
- **标准目录结构**：新任务默认生成 `00_来源资料`、`10_过程文件`、`20_成果输出`。
- **工作记录**：每个任务生成 `工作记录.md`，YAML 存放元数据，正文存放工作内容、过程、进展和下一步。
- **自动归档**：扫描工作目录，按部门归档到指定路径；编辑标题时同步更新工作记录和本地文件夹名。
- **部门管理**：维护部门名称、归档路径和默认部门。
- **日报生成**：扫描工作任务，提取 `工作记录.md` 的核心内容；可使用 AI 生成自然简短的日报条目。
- **安全存储**：邮箱密码和 AI API Key 通过 Electron `safeStorage` 加密保存。

## 技术栈

- **桌面应用**：Electron
- **前端**：React + Ant Design + Vite
- **后端**：Go + chi
- **目标平台**：Windows / Linux（UOS 建议在目标系统手动打包）

## 快速开始

### 安装依赖

```bash
pnpm install
```

### 开发模式

开发模式下 Electron 会启动 `backend/knot-backend.exe`，因此需要先编译后端。

```bash
# 1. 编译后端，输出文件名必须是 knot-backend.exe
cd backend
go build -o knot-backend.exe

# 2. 回到项目根目录，启动前端和 Electron
cd ..
pnpm run electron:dev
```

如果你的 Go 工具链通过 Conda 环境提供，可以使用：

```bash
cd backend
conda run -n go go build -o knot-backend.exe
```

### 前端开发服务器

仅调试前端页面时可以运行：

```bash
pnpm run dev
```

Vite 开发服务器会将 `/api` 代理到 `http://localhost:18000`。

## 构建打包

### Windows

```bash
pnpm run electron:build:win
```

当前 Windows 打包脚本会通过 `conda run -n go` 编译后端，并将二进制输出到 `electron/bin/knot-backend.exe`。如果本机 Go 不使用 Conda，请根据本地环境调整 `package.json` 中的 `backend:build:win` 脚本。

### Linux

```bash
pnpm run electron:build:linux
```

更多 UOS / Deepin 打包说明见 [BUILD.md](./BUILD.md)。

## 测试与检查

```bash
# 前端单元测试
pnpm test -- --run

# 前端生产构建
pnpm run build

# 后端测试
cd backend
go test ./...

# 后端编译检查
go build -buildvcs=false ./...

# Electron 主进程语法检查
node --check electron/main.js
node --check electron/preload.js
```

## 使用流程

1. 打开「设置」，配置邮箱服务器、端口、用户名、密码和 SSL。
2. 在「文件夹设置」中确认工作目录和命名格式，默认格式为 `{{YYYY}}.{{MM}}.{{DD}}_{{subject}}`。
3. 在「归档设置」中添加部门，并为每个部门配置归档路径。
4. 在「邮件列表」中查看邮件，点击「生成」创建任务文件夹。
5. 也可以在「快速创建」中输入工作内容，创建非邮件来源任务。
6. 在「自动归档」中扫描工作目录，编辑工作记录、调整部门，并归档到部门目录。
7. 在「日报生成」中扫描任务，勾选需要汇总的工作项，生成 Markdown 日报内容。

## 任务目录规范

新建任务文件夹使用以下结构：

```text
2026.04.23_工作内容/
├── 00_来源资料/
│   ├── email.txt
│   ├── email.pdf
│   └── 附件/
├── 10_过程文件/
├── 20_成果输出/
└── 工作记录.md
```

说明：

- 邮件来源任务会保存 `email.txt`、`email.pdf` 和附件。
- 快速创建任务不会额外生成 `references` 或 `requirement.md`。
- 自动归档和日报生成均以 `工作记录.md` 为核心数据源。

## 工作记录格式

`工作记录.md` 使用 YAML frontmatter 保存机器可读元数据，正文保存面向人工和 AI 的工作内容。

示例：

```markdown
---
type: task
schema_version: 3
title: 项目材料整理
status: active
created: 2026-04-23
updated: 2026-04-23
source: email
department: 办公室
project_path: C:/Workspace/2026.04.23_项目材料整理
folder_name: 2026.04.23_项目材料整理
archive_status: local_active
hash: xxxxxxxxxxxxxxxx
tags:
  - 工作材料
---

# 项目材料整理

## 工作内容

整理项目相关材料，完成来源资料归集、过程文件梳理和成果文件准备。

## 工作过程

- 2026-04-23：创建任务文件夹，整理邮件来源材料。

## 当前进展

已完成来源资料归集，正在补充过程文件。

## 下一步

继续完善成果文件，并将最终版本放入 20_成果输出。
```

自动归档列表会截取正文中的核心内容显示；日报生成也会将这些核心内容发送给 AI 生成简短日志。

## AI 日报

AI 日报配置位于「设置」中的「AI 设置」：

- API 地址：兼容 OpenAI Chat Completions 风格接口，例如 `https://api.openai.com/v1`
- 模型名称：例如 `gpt-4.1-mini`
- API Key：失焦后加密保存
- 启用 AI 日报：开启后，日报生成会优先调用 AI

如果 AI 未启用、配置不完整或接口调用失败，应用会使用本地规则生成简短兜底日报，避免流程中断。

## 项目结构

```text
knot/
├── electron/              # Electron 主进程和预加载脚本
│   ├── main.js
│   └── preload.js
├── src/                   # React 前端
│   ├── components/        # 页面和业务组件
│   ├── services/          # API、设置、路径和邮件缓存服务
│   ├── App.jsx
│   └── main.jsx
├── backend/               # Go 后端
│   ├── api/               # REST API、任务目录、归档和日报逻辑
│   ├── mail/              # IMAP 邮件处理
│   └── main.go
├── scripts/               # 图标生成等辅助脚本
├── BUILD.md               # 打包说明
└── package.json
```

## 注意事项

- 邮件列表缓存会按邮箱服务器、账号、端口、SSL、获取数量和时间范围区分，避免不同邮箱混用缓存。
- 自动归档依赖文件夹中的 `工作记录.md`，删除该文件后任务不会被扫描识别。
- 编辑自动归档中的任务标题会尝试同步重命名文件夹；如果目标文件夹已存在，会阻止更新并提示冲突。
- Windows 开发模式下后端文件名必须为 `backend/knot-backend.exe`。
