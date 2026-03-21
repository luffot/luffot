#!/bin/bash
set -e

SCRIPT_DIR=$(cd "$(dirname "$0")" && pwd)
cd "$SCRIPT_DIR"

echo "========================================"
echo "  Luffot 分离构建脚本"
echo "  主进程 + Ebiten: go build"
echo "  Wails 设置窗口: wails build"
echo "========================================"

# 1. 构建前端
echo ""
echo "[1/4] 构建前端..."
cd frontend
npm install
npm run build
cd ..

# 2. 复制前端资源到嵌入目录
echo ""
echo "[2/4] 复制前端资源..."
mkdir -p internal/assets/frontend/dist
cp -r frontend/dist/* internal/assets/frontend/dist/

# 删除 wails 生成的临时文件
rm -f frontend/wails.json

# 3. 构建主进程（go build，不依赖 Wails 框架）
echo ""
echo "[3/4] 构建主进程 (go build)..."

# 构建主进程二进制（包含 main + ebiten 模式）
# 使用 CGO 以支持 macOS 原生状态栏（NSStatusBar）和 Ebiten
CGO_ENABLED=1 go build \
    -ldflags "-s -w" \
    -o build/bin/luffot \
    ./main.go

echo "  主进程二进制: build/bin/luffot"

# 手动创建主进程的 .app bundle
echo "  创建 Luffot.app bundle..."
APP_DIR="build/bin/Luffot.app"
rm -rf "$APP_DIR"
mkdir -p "$APP_DIR/Contents/MacOS"
mkdir -p "$APP_DIR/Contents/Resources"

# 复制二进制
cp build/bin/luffot "$APP_DIR/Contents/MacOS/luffot"

# 生成 Info.plist（主进程使用 Accessory 模式，不显示 Dock 图标）
cat > "$APP_DIR/Contents/Info.plist" << 'PLIST'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>CFBundlePackageType</key>
    <string>APPL</string>
    <key>CFBundleName</key>
    <string>Luffot</string>
    <key>CFBundleExecutable</key>
    <string>luffot</string>
    <key>CFBundleIdentifier</key>
    <string>com.luffot.main</string>
    <key>CFBundleVersion</key>
    <string>1.0.0</string>
    <key>CFBundleShortVersionString</key>
    <string>1.0.0</string>
    <key>CFBundleIconFile</key>
    <string>iconfile</string>
    <key>LSMinimumSystemVersion</key>
    <string>10.13.0</string>
    <key>NSHighResolutionCapable</key>
    <true/>
    <key>LSUIElement</key>
    <true/>
    <key>NSHumanReadableCopyright</key>
    <string>Copyright © 2026</string>
    <key>NSCameraUsageDescription</key>
    <string>Luffot需要摄像头权限来检测是否有人站在您身后</string>
    <key>NSAppleEventsUsageDescription</key>
    <string>Luffot需要此权限来读取钉钉等应用的消息</string>
</dict>
</plist>
PLIST

echo "  Luffot.app 创建完成"

# 4. 构建 Wails 设置窗口（独立 .app）
echo ""
echo "[4/4] 构建 Wails 设置窗口 (wails build)..."

# 清理旧的 Wails 构建产物，避免残留旧的可执行文件
# （wails build 不会自动清理 MacOS 目录中的旧文件）
WAILS_MACOS_DIR="build/bin/luffot-settings.app/Contents/MacOS"
if [ -d "$WAILS_MACOS_DIR" ]; then
    echo "  清理旧的 Wails 构建产物..."
    rm -rf "$WAILS_MACOS_DIR"
fi

# 使用 -skipbindings 跳过绑定生成阶段
# 绑定生成会编译并运行整个应用（包括主进程代码），导致卡住
# 前端绑定文件已在 frontend/src/lib 中，无需重新生成
wails build --platform darwin/arm64 -skipbindings

# wails build 输出到 build/bin/luffot-settings.app（目录名取 wails.json 的 name 字段）
WAILS_APP_DIR="build/bin/luffot-settings.app"
echo "  Wails 设置窗口: $WAILS_APP_DIR"

# 重要：wails build 可能编译了根目录的 main.go 而非 cmd/luffot-wails/main.go，
# 导致 Wails 设置窗口实际运行的是主进程代码。
# 这里用 go build + production tag 重新编译正确的入口并替换。
echo "  重新编译 Wails 设置窗口入口 (cmd/luffot-wails)..."
CGO_LDFLAGS="-framework UniformTypeIdentifiers" CGO_ENABLED=1 go build \
    -tags production \
    -ldflags "-s -w" \
    -o "$WAILS_APP_DIR/Contents/MacOS/Luffot Settings" \
    ./cmd/luffot-wails/
echo "  Wails 设置窗口二进制已替换为正确入口"

# 5. 将 Wails 设置窗口嵌入主 App Bundle 的 Helpers 目录
# 这样设置窗口作为主 App 的 Helper，在系统设置中只需给 Luffot.app 一个应用授权
echo ""
echo "[5/4] 将设置窗口嵌入主 App Bundle..."
HELPERS_DIR="$APP_DIR/Contents/Helpers"
mkdir -p "$HELPERS_DIR"

# 将 Wails 设置窗口整个 .app 复制到 Helpers 目录
# 使用 "Luffot Settings.app" 作为统一名称
EMBEDDED_APP="$HELPERS_DIR/Luffot Settings.app"
rm -rf "$EMBEDDED_APP"
cp -R "$WAILS_APP_DIR" "$EMBEDDED_APP"
echo "  已嵌入: $EMBEDDED_APP"

# 同时保留 build/bin 下的独立副本，方便开发调试
echo "  独立副本保留: $WAILS_APP_DIR"

echo ""
echo "========================================"
echo "  构建完成!"
echo ""
echo "  主进程:       build/bin/Luffot.app"
echo "  设置窗口(嵌入): Luffot.app/Contents/Helpers/Luffot Settings.app"
echo "  设置窗口(独立): build/bin/luffot-settings.app (开发调试用)"
echo ""
echo "  使用方式:"
echo "    open build/bin/Luffot.app"
echo "  或直接运行:"
echo "    ./build/bin/luffot"
echo "========================================"