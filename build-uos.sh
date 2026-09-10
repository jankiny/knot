#!/usr/bin/env bash
# 在 UOS/Linux 中编译和打包；不运行测试，不安装或发布应用。
set -Eeuo pipefail

usage() {
  printf '%s\n' \
    '用法：bash build-uos.sh [--appimage] [--skip-install]' \
    '默认生成 .deb；--appimage 同时生成 AppImage。' \
    '--skip-install：本机已有构建依赖时，跳过 pnpm install。'
}
appimage=false
skip_install=false
for option in "$@"; do
  case "$option" in
    --appimage) appimage=true ;;
    --skip-install) skip_install=true ;;
    -h|--help) usage; exit 0 ;;
    *) usage >&2; printf '未知选项：%s\n' "$option" >&2; exit 1 ;;
  esac
done

project_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)"
cd -- "$project_dir"
if [[ "$(uname -s)" != Linux ]]; then
  printf '请在 UOS/Linux 虚拟机中执行，不要在 Windows 或 Git Bash 中打包。\n' >&2
  exit 1
fi
for tool in node pnpm go tee mktemp; do
  command -v "$tool" >/dev/null 2>&1 || {
    printf '缺少命令：%s，请先安装。\n' "$tool" >&2
    exit 1
  }
done
case "$(uname -m)" in
  x86_64) go_arch=amd64; electron_arch=x64 ;;
  aarch64|arm64) go_arch=arm64; electron_arch=arm64 ;;
  *) printf '此脚本暂只支持 x86_64 和 aarch64。\n' >&2; exit 1 ;;
esac

# 使用本机 Linux 架构，避免继承开发模式或其他平台的交叉编译设置。
unset NODE_ENV
export GOOS=linux GOARCH="$go_arch" GOTOOLCHAIN=local
version="$(node -p "require('./package.json').version")"
mkdir -p dist-electron
output_dir="$(mktemp -d "$project_dir/dist-electron/uos-$(date +%Y%m%d-%H%M%S)-XXXXXX")"
exec > >(tee "$output_dir/build.log") 2>&1
trap 'status=$?; printf "\n打包失败（退出码 %s）。日志：%s/build.log\n" "$status" "$output_dir" >&2; exit "$status"' ERR

{
  printf '项目：%s\n版本：%s\n架构：%s\n时间：%s\n' \
    "$project_dir" "$version" "$electron_arch" "$(date -Iseconds)"
  node --version
  pnpm --version
  go version
  if command -v git >/dev/null 2>&1 && [[ -e .git ]]; then
    git log -1 --format='%h %s'
    git status --short
  else
    printf '源码副本未包含 Git 元数据。\n'
  fi
} | tee "$output_dir/build-info.txt"

if [[ "$skip_install" == false ]]; then
  pnpm install --frozen-lockfile --prod=false
fi

printf '\n编译 Linux 后端……\n'
mkdir -p electron/bin
(cd backend && go build -buildvcs=false -ldflags='-s -w' -o ../electron/bin/knot-backend .)
chmod +x electron/bin/knot-backend

printf '\n构建前端……\n'
pnpm run build

printf '\n生成安装包……\n'
targets=(deb)
if [[ "$appimage" == true ]]; then targets+=(AppImage); fi
pnpm exec electron-builder --linux "${targets[@]}" "--$electron_arch" \
  --publish never "--config.directories.output=$output_dir"

printf '\n打包完成。版本号直接使用 package.json，不会自动递增。\n'
printf '本次输出目录：%s\n' "$output_dir"
find "$output_dir" -maxdepth 1 -type f \( -name '*.deb' -o -name '*.AppImage' \) -print
printf '请完全退出旧客户端，再安装本次生成的具体 .deb 文件。\n'
