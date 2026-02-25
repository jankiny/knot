#!/bin/bash
# 图标生成脚本
# 需要安装 ImageMagick: sudo apt-get install imagemagick (Linux) 或 brew install imagemagick (macOS)

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
ICONS_DIR="$PROJECT_DIR/electron/assets/icons"
SOURCE_ICON="$ICONS_DIR/991x991.png"

echo "源图标: $SOURCE_ICON"
echo "输出目录: $ICONS_DIR"

# 检查源图标是否存在
if [ ! -f "$SOURCE_ICON" ]; then
    echo "错误: 找不到源图标 $SOURCE_ICON"
    exit 1
fi

# 检查 ImageMagick 是否安装
if ! command -v convert &> /dev/null; then
    echo "错误: 需要安装 ImageMagick"
    echo "  Linux: sudo apt-get install imagemagick"
    echo "  macOS: brew install imagemagick"
    exit 1
fi

# 生成 Linux 各尺寸 PNG
echo "生成 Linux PNG 图标..."
for size in 16 24 32 48 64 128 256 512; do
    output="$ICONS_DIR/${size}x${size}.png"
    convert "$SOURCE_ICON" -resize ${size}x${size} "$output"
    echo "  生成: ${size}x${size}.png"
done

# 生成 Windows ICO (多分辨率)
echo "生成 Windows ICO 图标..."
convert "$SOURCE_ICON" \
    -define icon:auto-resize=256,128,64,48,32,16 \
    "$PROJECT_DIR/electron/assets/icon.ico"
echo "  生成: icon.ico"

# 如果在 macOS 上，尝试生成 ICNS
if [[ "$OSTYPE" == "darwin"* ]]; then
    echo "生成 macOS ICNS 图标..."
    mkdir -p "$PROJECT_DIR/electron/assets/icon.iconset"
    for size in 16 32 64 128 256 512; do
        convert "$SOURCE_ICON" -resize ${size}x${size} "$PROJECT_DIR/electron/assets/icon.iconset/icon_${size}x${size}.png"
        # Retina 版本
        double=$((size * 2))
        if [ $double -le 512 ]; then
            convert "$SOURCE_ICON" -resize ${double}x${double} "$PROJECT_DIR/electron/assets/icon.iconset/icon_${size}x${size}@2x.png"
        fi
    done
    iconutil -c icns "$PROJECT_DIR/electron/assets/icon.iconset" -o "$PROJECT_DIR/electron/assets/icon.icns"
    rm -rf "$PROJECT_DIR/electron/assets/icon.iconset"
    echo "  生成: icon.icns"
fi

echo "图标生成完成!"
