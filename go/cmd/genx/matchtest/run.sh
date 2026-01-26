#!/bin/bash

# matchtest - 规则匹配 LLM Benchmark 工具
#
# 使用方式:
#   直接运行: ./run.sh [args...]
#   Bazel:    bazel run //go/cmd/genx/matchtest:run -- [args...]
#
# Web UI 模式:
#   使用 -serve :8080 启动带 Web UI 的 benchmark
#   可以通过浏览器实时查看进度
#
# API Key 配置:
#   在 .bazelrc.user 中添加环境变量 (参考 .bazelrc.user.example)
#
# 示例:
#   ./run.sh -model zhipu/ -serve :8080  # 带 Web UI 运行
#   ./run.sh -model all -o results.json  # 保存结果
#   ./run.sh -list                       # 列出可用模型
#   ./run.sh -load results.json -serve :8080  # 查看已保存结果

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 获取目录路径
if [ -n "$BUILD_WORKSPACE_DIRECTORY" ]; then
    # Bazel 环境
    PROJECT_ROOT="$BUILD_WORKSPACE_DIRECTORY"
else
    # 直接运行
    SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
    PROJECT_ROOT="$(cd "$SCRIPT_DIR/../../../.." && pwd)"
fi

SCRIPT_DIR="$PROJECT_ROOT/go/cmd/genx/matchtest"
BINARY="$PROJECT_ROOT/bazel-bin/go/cmd/genx/matchtest/matchtest_/matchtest"
BAZEL_TARGET="//go/cmd/genx/matchtest"
DEFAULT_RULES_DIR="$PROJECT_ROOT/testdata/matchtest/rules"
DEFAULT_MODELS_DIR="$PROJECT_ROOT/testdata/matchtest/models"

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[OK]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

# 构建二进制文件（如果需要）
build_if_needed() {
    if [ ! -f "$BINARY" ]; then
        log_info "构建 matchtest (bazel build $BAZEL_TARGET)..."
        (cd "$PROJECT_ROOT" && bazel build "$BAZEL_TARGET")
        log_success "构建完成"
    fi
}

# 从 .bazelrc.user 加载环境变量
load_env_from_bazelrc() {
    local bazelrc_user="$PROJECT_ROOT/.bazelrc.user"
    if [ -f "$bazelrc_user" ]; then
        # 提取 action_env 并导出为环境变量
        while IFS= read -r line; do
            if [[ "$line" =~ ^build\ --action_env=([A-Z_]+)=(.+)$ ]]; then
                local key="${BASH_REMATCH[1]}"
                local value="${BASH_REMATCH[2]}"
                # 只导出 API key 相关的环境变量
                if [[ "$key" == *"API_KEY"* ]]; then
                    export "$key=$value"
                fi
            fi
        done < "$bazelrc_user"
    fi
}

# 显示帮助
show_help() {
    echo "matchtest - 规则匹配 LLM Benchmark 工具"
    echo ""
    echo "Usage:"
    echo "  $0 -model <pattern>            Test models matching pattern"
    echo "  $0 -list                       List available models"
    echo "  $0 -prompt                     Print generated system prompt"
    echo "  $0 -load <file.json>           Load and serve existing report"
    echo ""
    echo "Model patterns:"
    echo "  -model gemini/                 All Gemini models"
    echo "  -model sf/                     All SiliconFlow models"
    echo "  -model openai/                 All OpenAI models"
    echo "  -model sf/,gemini/             Multiple prefixes (comma-separated)"
    echo "  -model all                     All registered models"
    echo ""
    echo "Options:"
    echo "  -rules <dir>                   Rules directory (default: testdata/matchtest/rules)"
    echo "  -models <dir>                  Models config directory (default: testdata/matchtest/models)"
    echo "  -tpl <file.gotmpl>             Custom prompt template file"
    echo "  -o <file.json>                 Save results to JSON file"
    echo "  -serve :8080                   Start web server after test"
    echo "  -q                             Quiet mode"
    echo "  -no-tui                        Disable TUI progress display"
    echo ""
    echo "Examples:"
    echo "  $0 -model gemini/flash"
    echo "  $0 -model sf/ -o results.json -serve :8080"
    echo "  $0 -model all -o results.json"
    echo "  $0 -load results.json"
    echo ""
    echo "API Key 配置 (.bazelrc.user):"
    echo "  build --action_env=OPENAI_API_KEY=sk-xxx"
    echo "  build --action_env=GEMINI_API_KEY=xxx"
    echo "  build --action_env=SILICONFLOW_API_KEY=sk-xxx"
}

# 检查参数中是否已有指定的 flag
has_arg() {
    local target="$1"
    shift
    for arg in "$@"; do
        if [ "$arg" = "$target" ]; then
            return 0
        fi
    done
    return 1
}

# 检查是否是不需要自动添加 models 参数的命令
# -load 和 -prompt 不需要 models，但 -prompt 需要 rules
is_load_command() {
    for arg in "$@"; do
        if [ "$arg" = "-load" ]; then
            return 0
        fi
    done
    return 1
}

main() {
    # 显示帮助
    if [ "$1" = "-h" ] || [ "$1" = "--help" ] || [ -z "$1" ]; then
        show_help
        exit 0
    fi

    # 加载环境变量
    load_env_from_bazelrc

    # 构建
    build_if_needed

    # -load 命令直接执行，不需要额外参数
    if is_load_command "$@"; then
        exec "$BINARY" "$@"
    fi

    # 构建参数列表，自动添加缺失的 -rules 和 -models
    ARGS=()
    
    # 始终需要 -rules
    if ! has_arg "-rules" "$@"; then
        if [ -d "$DEFAULT_RULES_DIR" ]; then
            ARGS+=("-rules" "$DEFAULT_RULES_DIR")
        else
            log_error "默认 rules 目录不存在: $DEFAULT_RULES_DIR"
            exit 1
        fi
    fi
    
    # -prompt 命令不需要 -models
    if ! has_arg "-prompt" "$@" && ! has_arg "-models" "$@"; then
        if [ -d "$DEFAULT_MODELS_DIR" ]; then
            ARGS+=("-models" "$DEFAULT_MODELS_DIR")
        else
            log_error "默认 models 目录不存在: $DEFAULT_MODELS_DIR"
            exit 1
        fi
    fi
    
    # 执行二进制（直接运行以支持 TTY/TUI）
    exec "$BINARY" "${ARGS[@]}" "$@"
}

main "$@"
