#!/bin/bash
# HarmonyOS 命令行构建脚本
# 使用 hvigorw 构建 HAP 包

set -e

# Check if running inside bazel
if [[ -z "$BUILD_WORKSPACE_DIRECTORY" ]]; then
    echo "ERROR: This script must be run via bazel." >&2
    echo >&2
    echo "Usage:" >&2
    echo "  bazel run //examples/bazel/harmonyos:build_native -- [command]" >&2
    echo >&2
    echo "Commands:" >&2
    echo "  clean        清理构建产物" >&2
    echo "  install-deps 安装依赖" >&2
    echo "  build        构建 HAP (默认)" >&2
    echo "  run          构建、安装并运行" >&2
    exit 1
fi

# 获取项目目录 (使用 workspace 路径)
PROJECT_DIR="$BUILD_WORKSPACE_DIRECTORY/examples/bazel/harmonyos/HelloWorld"

# DevEco Studio 路径（从环境变量或默认值）
DEVECO_HOME="${DEVECO_HOME:-/Applications/DevEco-Studio.app/Contents}"

# 设置 Node.js 路径
export NODE_HOME="$DEVECO_HOME/tools/node"
export PATH="$NODE_HOME/bin:$PATH"

# 设置其他工具路径
export PATH="$DEVECO_HOME/tools/hvigor/bin:$DEVECO_HOME/tools/ohpm/bin:$PATH"
export DEVECO_SDK_HOME="$DEVECO_HOME/sdk"
export JAVA_HOME="$DEVECO_HOME/jbr/Contents/Home"
export PATH="$JAVA_HOME/bin:$PATH"

# HDC 路径
HDC="$DEVECO_SDK_HOME/default/openharmony/toolchains/hdc"

echo "=== HarmonyOS 命令行构建 ==="
echo "项目目录: $PROJECT_DIR"
echo "DEVECO_HOME: $DEVECO_HOME"
echo ""

# 命令参数
COMMAND="${1:-build}"

# 函数定义
do_clean() {
    echo "=== 清理构建 ==="
    cd "$PROJECT_DIR"
    rm -rf entry/build .hvigor
    echo "清理完成"
}

do_install_deps() {
    echo "=== 安装依赖 ==="
    cd "$PROJECT_DIR"
    ohpm install
}

do_build() {
    do_install_deps
    
    echo ""
    echo "=== 构建 HAP ==="
    cd "$PROJECT_DIR"
    hvigorw assembleHap --no-daemon -p product=default
    
    echo ""
    echo "=== 构建完成 ==="
    HAP_FILE=$(find entry/build -name "*.hap" -type f | head -1)
    echo "HAP 文件: $HAP_FILE"
}

do_run() {
    do_build
    
    echo ""
    echo "=== 安装到设备 ==="
    cd "$PROJECT_DIR"
    HAP_FILE=$(find entry/build -name "*.hap" -type f | head -1)
    
    if [ -z "$HAP_FILE" ]; then
        echo "ERROR: HAP 文件未找到"
        exit 1
    fi
    
    "$HDC" install "$HAP_FILE"
    
    echo ""
    echo "=== 启动应用 ==="
    # 从 app.json5 获取 bundle name (使用 sed 兼容 macOS)
    BUNDLE_NAME=$(sed -n 's/.*"bundleName"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' AppScope/app.json5)
    echo "Bundle: $BUNDLE_NAME"
    "$HDC" shell aa start -a EntryAbility -b "$BUNDLE_NAME"
    
    echo ""
    echo "✅ 应用已启动！"
}

case "$COMMAND" in
    clean)
        do_clean
        ;;
    install-deps)
        do_install_deps
        ;;
    build)
        do_build
        ;;
    run)
        do_run
        ;;
    *)
        echo "用法: $0 [命令]"
        echo ""
        echo "命令:"
        echo "  clean        清理构建产物"
        echo "  install-deps 安装依赖"
        echo "  build        构建 HAP (默认)"
        echo "  run          构建、安装并运行"
        ;;
esac
