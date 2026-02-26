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

### Linux (UOS / Deepin) 打包

在 UOS 等国产系统中，由于底层安全策略和 glibc 差异，直接使用 `nvm` 安装的 Node.js 二进制文件可能会出现“段错误 (Segmentation fault)”。
请按照以下步骤在目标 UOS 环境中准备环境并进行打包：

#### 1. 安装系统级 Node.js 18 (通过 NodeSource APT 源)

```bash
# 下载并添加 NodeSource v18 源 (需要 sudo 密码)
curl -fsSL https://deb.nodesource.com/setup_18.x | sudo -E bash -

# 通过系统 apt 安装 nodejs (自带 npm)
sudo apt-get install -y nodejs

# 验证安装成功 (无段错误)
node -v
```

#### 2. 安装 pnpm 及依赖并打包

```bash
# 全局安装 pnpm
sudo npm install -g pnpm

# 进入项目目录安装依赖
cd ~/projects/knot
pnpm install

# 执行打包命令
pnpm run electron:build:linux
```

> [!NOTE]
> GitHub Actions 的自动化发布流程仍然使用 `ubuntu-latest` 构建标准 Linux 包。针对完全国产化的环境，建议在特定的 UOS 实体机或虚拟机内使用上述命令手动编译生成兼容的 `.deb`。

## 3. 注意事项

- **管理员权限**: 在 Windows 上打包时，建议开启“开发人员模式”或使用管理员权限运行终端。
- **图标**: 请确保图标文件位于 `electron/assets/` 目录下。若在 UOS/Linux 打包后发现桌面**没有显示图标**：
  1. 确保 `package.json` 中的 `linux.icon` 明确指定到了具体的文件（例如：`electron/assets/icons/512x512.png`）。
  2. 如果由于 UOS 图标缓存导致依然不显示，可在 UOS 终端运行 `sudo update-icon-caches /usr/share/icons/*` 更新系统缓存，并注销重新登录。
- **后端二进制**: 打包脚本会自动处理后端的编译，无需手动复制。
