# Knot（绳结）打包指南

本项目使用 `electron-builder` 进行打包。后端已由 Python 迁移为 Go，打包过程更加简化。

## 1. 准备工作

确保已安装以下环境：
- Node.js (建议 v18+)
- Go (1.21+)
- pnpm

安装前端依赖：
```bash
pnpm install
```

## 2. 打包步骤

### Windows (x64) 打包

项目已在 `package.json` 中配置了自动化构建脚本，此脚本将：
1. 进入 `backend/` 编译出 `knot-backend.exe`
2. 将二进制文件同步到打包资源目录
3. 构建 React 前端
4. 生成安装包

在终端运行：
```bash
pnpm run electron:build:win
```

> [!TIP]
> **手动构建后端**：
> 如果您只想在开发时单独编译后端，可以在 `backend/` 目录下运行：
> `go build -o knot-backend.exe`
> 注意：文件名必须为 `knot-backend.exe`，否则 Electron 无法启动后端。

打包完成后，安装程序将生成在 `dist-electron/` 目录下。

### Linux (统信 UOS / Debian 10) 打包

在 UOS 系统或 Debian 10 环境下运行：

```bash
pnpm run electron:build:linux
```

## 3. 注意事项

- **管理员权限**: 在 Windows 上打包时，建议开启“开发人员模式”或使用管理员权限运行终端。
- **图标**: 请确保图标文件位于 `electron/assets/` 目录下。
- **后端二进制**: 打包脚本会自动处理后端的编译，无需手动复制。
