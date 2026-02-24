# Knot 绳结

办公数字化工具 - 邮件驱动的工作流管理

## 功能特点

- 查看邮件收件列表（只读模式）
- 一键从邮件生成桌面工作文件夹
- 自动命名规范：`YYYY.MM.DD_工作内容`
- 支持下载邮件附件到文件夹

## 技术栈

- **桌面应用**: Electron
- **前端**: React + Ant Design
- **后端**: Go
- **目标平台**: Windows / UOS (Debian 10)

## 快速开始

### 安装依赖

```bash
# 前端依赖
pnpm install
```

### 开发模式

```bash
# 1. 编译后端 (在 backend 目录下)
cd backend
# 注意：必须指定输出为 knot-backend.exe，因为 Electron 的 main.js 中被硬编码去寻找这个特定的文件名。
go build -o knot-backend.exe

# 2. 启动前端及 Electron (在根目录下)
pnpm run electron:dev
```

### 构建打包

```bash
# 打包 Electron 应用（会自动编译后端并打包）
pnpm run electron:build:win
```

## 项目结构

```
knot/
├── electron/          # Electron 主进程
│   ├── main.js        # 主进程入口
│   └── preload.js     # 预加载脚本
├── src/               # React 前端
│   ├── components/    # React 组件
│   ├── services/      # API 服务
│   └── App.jsx        # 应用入口
├── backend/           # 后端 (Go 语言)
│   ├── api/           # REST API
│   ├── mail/          # 邮件处理
│   └── main.go        # 后端入口
└── package.json
```

## 使用说明

1. 启动应用后，配置邮件服务器信息（IMAP 协议）
2. 连接成功后显示收件箱邮件列表
3. 点击「生成文件夹」按钮在桌面创建工作文件夹
4. 如有附件，可选择「含附件」一并下载
