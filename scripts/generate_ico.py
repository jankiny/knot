from PIL import Image
import os

# 路径配置
current_dir = os.path.dirname(os.path.abspath(__file__))
project_root = os.path.dirname(current_dir)
assets_dir = os.path.join(project_root, 'electron', 'assets')
source_icon = os.path.join(assets_dir, 'icons', '512x512.png')
output_icon = os.path.join(assets_dir, 'icon.ico')

def generate_ico():
    if not os.path.exists(source_icon):
        print(f"Error: Source icon not found at {source_icon}")
        return

    try:
        img = Image.open(source_icon)
        # 生成包含多种尺寸的 ICO
        icon_sizes = [(256, 256), (128, 128), (64, 64), (48, 48), (32, 32), (16, 16)]
        img.save(output_icon, sizes=icon_sizes)
        print(f"Success: Generated {output_icon}")
    except Exception as e:
        print(f"Error generating ICO: {e}")

if __name__ == '__main__':
    generate_ico()
