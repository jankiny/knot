# Knot 手动打包指南

Knot 使用 React/Vite 前端、Go 后端和 Electron。打包时需要重新编译前后端，仅更新源码或运行前端构建不会更新已安装客户端。

## UOS / Deepin 打包

在 UOS 虚拟机中使用项目根目录的 `build-uos.sh`。脚本只安装项目依赖、编译和生成安装包，**不运行测试、不安装客户端、不发布版本**。

### 1. 准备环境

需要 Bash、Node.js、pnpm、Go，以及系统自带的 `tee`、`mktemp`、`find` 等工具。

- Node.js 至少为 18，使用能在目标 UOS 上正常运行且满足项目依赖要求的版本。
- pnpm 建议使用 9，与当前仓库发布工作流一致。
- Go 最低版本以 `backend/go.mod` 为准，当前为 **1.25.7**。旧文档中的 Go 1.21+ 已不适用。
- 脚本支持 x86_64 和 aarch64，自动匹配 Go 与 Electron 架构。
- 内网需提前准备项目依赖、Go 模块、Electron 和打包工具缓存，或配置单位可用的镜像。不要复制 Windows 的 `node_modules` 到 UOS。
- 脚本使用本机 Go 工具链，不会自动下载新的 Go 版本。工具链过旧时先更新再打包。

查看环境：

```bash
node --version
pnpm --version
go version
uname -m
```

### 2. 确认源码版本

进入项目根目录：

```bash
git status --short
git branch --show-current
git log -1 --oneline
node -p "require('./package.json').version"
```

需要从远端更新时，先保留本地修改，再切换到需要的分支。例如本次邮箱兼容改动在 `dev`：

```bash
git fetch origin
git switch dev
git pull --ff-only origin dev
```

内网无法访问远端时，拷贝最新源码到虚拟机即可；脚本支持不带 `.git` 的源码目录，不会自动拉取或切换分支。

“内网兼容：跳过证书验证”由提交 `7e5e156` 引入，可检查源码：

```bash
grep -n '内网兼容' src/components/settings/MailSettingsSection.jsx
```

如果没有该内容，应先更新源码；重复打包旧源码不会出现新按钮。

### 3. 执行打包脚本

在 UOS 项目根目录运行：

```bash
# 默认生成 .deb
bash build-uos.sh

# 同时生成 .deb 和 AppImage
bash build-uos.sh --appimage

# 已经在本机安装完整构建依赖时，跳过 pnpm install
bash build-uos.sh --skip-install

# 查看帮助
bash build-uos.sh --help
```

选项可以组合。脚本自动定位自身所在目录，也可执行 `bash /完整路径/app-knot/build-uos.sh`。构建时不需要 sudo。

脚本按顺序执行：

1. 记录源码提交、工作区状态、应用版本和工具版本。
2. 安装项目依赖（可跳过）。
3. 编译 Linux 后端到 `electron/bin/knot-backend`。
4. 运行 `pnpm run build` 生成最新 `dist`。
5. 调用 electron-builder 打包，关闭自动发布。

任一步失败会停止，并保留日志。`--skip-install` 只跳过 pnpm 安装；如果 Go 模块或 Electron 打包缓存缺失，后续步骤仍可能需要网络。

### 4. 找到本次安装包

每次使用独立目录，不删除历史包。例如：

```text
dist-electron/uos-20260910-153000-Ab12Cd/
  knot-linux-1.4.0-x64.deb
  linux-unpacked/
  build.log
  build-info.txt
```

架构和文件名以脚本实际输出为准：

- `build.log`：本次构建日志，失败时查看这里。
- `build-info.txt`：源码提交、工作区状态、版本和构建环境。
- 只有脚本最后显示“打包完成”才表示整个流程成功；不要使用失败构建中残留的产物。

**手动打包不会自动递增版本号**，使用的是 `package.json` 的 `version`。版本号相同也可能包含新代码，应结合源码提交和新功能判断。需要改发布版本时，按项目版本计划修改 `package.json` 后再打包。

### 5. 安装与启动

完全退出正在运行的 Knot，再安装本次输出的具体文件，替换下方路径：

```bash
dpkg-deb -f "/完整路径/本次安装包.deb" Package Version Architecture
sudo apt install "/完整路径/本次安装包.deb"

# 查看已经安装的版本和文件路径
dpkg-query -W -f='${Package} ${Version} ${Architecture}\n' knot
dpkg -L knot | grep -E '/knot$|/app\.asar$|/knot-backend$'
```

不要用 `*.deb` 一次安装多个历史包。安装新 .deb 不会更新旧 AppImage 或旧 `linux-unpacked/knot`，需要确认桌面快捷方式指向新安装路径。

Knot 使用单实例锁，旧进程未退出时可能只是唤醒旧窗口。如果从终端运行正式客户端，且以前设置过 `NODE_ENV=development`，先执行 `unset NODE_ENV`，以免转而加载开发服务器。

新开关位于「设置 → 邮件设置」中“使用 SSL”的下方。

## 不使用脚本时的手动命令

在 UOS 项目根目录逐步执行，每一步成功后再执行下一步：

```bash
pnpm install --frozen-lockfile --prod=false
pnpm run backend:build:linux
pnpm run build
pnpm exec electron-builder --linux deb --publish never
```

以上命令写入常规 `dist-electron` 目录，请按文件时间确认新包。原有 `pnpm run electron:build:linux` 也会编译前后端并打包，但根目录脚本额外提供独立输出目录和日志。

`pnpm run build` 只构建前端；`pnpm run electron:build` 不会重新编译 Go 后端。不要误用这两个命令替代完整流程。

## Windows 打包

Windows 仍使用原有流程：

```powershell
pnpm install --frozen-lockfile
pnpm run electron:build:win
```

当前 Windows 后端脚本通过 `conda run -n go` 调用 Go，需要对应环境。输出位于 `dist-electron`。

开发模式读取 `backend/knot-backend.exe`（Linux 为 `backend/knot-backend`），安装包读取 `resources/bin` 下的后端。仅更新开发模式的二进制不会更新已安装客户端。
