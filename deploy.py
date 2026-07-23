"""部署：将编译产物复制到 D:\Program Files\mkinput"""
import os, shutil, sys, io
sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8')

src = os.path.join(os.path.dirname(__file__), "build", "mkinput.exe")
dst_dir = r"D:\Program Files\mkinput"
dst = os.path.join(dst_dir, "mkinput.exe")

if not os.path.exists(src):
    print("[错误] 未找到编译产物:", src)
    print("   请先执行: go build -ldflags -H=windowsgui -o build/mkinput.exe .")
    sys.exit(1)

os.makedirs(dst_dir, exist_ok=True)

if os.path.exists(dst):
    print("[注意] 目标位置已有同名文件，正在替换...")
    os.remove(dst)

shutil.copy2(src, dst)
print("[完成] 已复制到", dst)
