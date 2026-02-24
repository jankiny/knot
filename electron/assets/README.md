# Electron Assets

此目录包含 Electron 应用的图标资源。

## 需要的图标文件

### Windows
- `icon.ico` - Windows 应用图标 (256x256 多分辨率 ICO)

### Linux
在 `icons` 目录下创建以下 PNG 文件:
- `16x16.png`
- `32x32.png`
- `48x48.png`
- `64x64.png`
- `128x128.png`
- `256x256.png`
- `512x512.png`

### macOS
- `icon.icns` - macOS 应用图标

## 生成图标

可以使用在线工具或以下命令从 PNG 生成:

```bash
# 从 512x512 PNG 生成 ICO (需要 ImageMagick)
convert icon-512.png -define icon:auto-resize=256,128,64,48,32,16 icon.ico

# 生成 Linux 图标
for size in 16 32 48 64 128 256 512; do
  convert icon-512.png -resize ${size}x${size} icons/${size}x${size}.png
done
```

## 临时图标

如果没有自定义图标，Electron 会使用默认图标。建议在正式发布前添加自定义图标。
