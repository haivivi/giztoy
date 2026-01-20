#!/bin/bash

# DashScope API 示例测试脚本
# 
# 支持同时测试 Go 和 Rust CLI
#
# 使用方式:
#   直接运行: ./run.sh [runtime] [test_level]
#   Bazel:    bazel run //examples/cmd/dashscope:run -- [runtime] [test_level]
#
#   runtime: go | rust | both (默认: go)
#   test_level: 1, all, quick
#
# 前置条件: 需要先配置 context
#   dashscope config add-context dashscope_cn --api-key YOUR_API_KEY

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# 配置
CONTEXT_NAME="${DASHSCOPE_CONTEXT:-dashscope_cn}"
API_KEY="${DASHSCOPE_API_KEY:-}"

# 获取目录路径
if [ -n "$BUILD_WORKSPACE_DIRECTORY" ]; then
    # Bazel 环境
    PROJECT_ROOT="$BUILD_WORKSPACE_DIRECTORY"
else
    # 直接运行
    SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
    PROJECT_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"
fi

SCRIPT_DIR="$PROJECT_ROOT/examples/cmd/dashscope"
COMMANDS_DIR="$SCRIPT_DIR/commands"
OUTPUT_DIR="$SCRIPT_DIR/output"

# 构建 CLI（如果需要）
build_cli() {
    local target="$1"
    case "$target" in
        go)
            if [ ! -f "$PROJECT_ROOT/bazel-bin/go/cmd/dashscope/dashscope_/dashscope" ]; then
                log_info "构建 Go CLI (bazel build //go/cmd/dashscope)..."
                (cd "$PROJECT_ROOT" && bazel build //go/cmd/dashscope)
            fi
            ;;
        rust)
            # 优先使用 Bazel 构建，如果失败则回退到 Cargo
            if [ ! -f "$PROJECT_ROOT/bazel-bin/rust/cmd/dashscope/dashscope" ] && [ ! -f "$PROJECT_ROOT/rust/target/release/dashscope" ]; then
                log_info "构建 Rust CLI (bazel build //rust/cmd/dashscope)..."
                if ! (cd "$PROJECT_ROOT" && bazel build //rust/cmd/dashscope); then
                    log_warn "Bazel 构建失败，回退到 Cargo..."
                    if ! (cd "$PROJECT_ROOT/rust" && cargo build --release --bin dashscope); then
                        log_error "Cargo 构建也失败，请检查构建环境..."
                        exit 1
                    fi
                fi
            fi
            ;;
    esac
}

# CLI 命令路径
GO_CMD="$PROJECT_ROOT/bazel-bin/go/cmd/dashscope/dashscope_/dashscope"

# Rust CLI: 优先 Bazel，回退 Cargo
rust_cmd() {
    if [ -f "$PROJECT_ROOT/bazel-bin/rust/cmd/dashscope/dashscope" ]; then
        echo "$PROJECT_ROOT/bazel-bin/rust/cmd/dashscope/dashscope"
    else
        echo "$PROJECT_ROOT/rust/target/release/dashscope"
    fi
}

# 当前使用的命令
DASHSCOPE_CMD=""
RUNTIME=""

# 创建输出目录
mkdir -p "$OUTPUT_DIR"

# 辅助函数
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[PASS]${NC} $1"
}

log_error() {
    echo -e "${RED}[FAIL]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_runtime() {
    echo -e "${CYAN}[$RUNTIME]${NC} $1"
}

run_test_verbose() {
    local name="$1"
    local cmd="$2"
    
    log_info "测试: $name"
    
    if eval "$cmd"; then
        log_success "$name"
        return 0
    else
        log_error "$name"
        return 1
    fi
}

# 设置运行时环境
setup_runtime() {
    local runtime="$1"
    RUNTIME="$runtime"
    
    case "$runtime" in
        go)
            build_cli go
            DASHSCOPE_CMD="$GO_CMD"
            log_runtime "使用 Go CLI (Bazel build)"
            ;;
        rust)
            build_cli rust
            DASHSCOPE_CMD="$(rust_cmd)"
            log_runtime "使用 Rust CLI"
            ;;
        *)
            log_error "未知的运行时: $runtime"
            exit 1
            ;;
    esac
}

# =====================================
# 阶段 0: 配置上下文
# =====================================
setup_context() {
    log_info "=== 阶段 0: 配置上下文 ==="
    
    if [ -n "$API_KEY" ]; then
        $DASHSCOPE_CMD config add-context "$CONTEXT_NAME" --api-key "$API_KEY" 2>/dev/null || true
    fi
    
    $DASHSCOPE_CMD config use-context "$CONTEXT_NAME" 2>/dev/null || true
    
    # 检查 context 是否存在
    if ! $DASHSCOPE_CMD config list-contexts 2>/dev/null | grep -q "$CONTEXT_NAME"; then
        log_error "Context '$CONTEXT_NAME' 不存在！请先运行:"
        echo "  dashscope config add-context $CONTEXT_NAME --api-key YOUR_API_KEY"
        exit 1
    fi
    
    log_success "上下文配置完成: $CONTEXT_NAME"
    echo ""
}

# =====================================
# 阶段 1: 实时语音测试 (Omni Chat)
# =====================================
test_level_1() {
    log_info "=== 阶段 1: 实时语音测试 (Omni Chat) ==="
    
    # 检查是否有音频文件
    if [ -f "$COMMANDS_DIR/omni-chat.yaml" ]; then
        run_test_verbose "Omni Chat (配置文件模式)" \
            "$DASHSCOPE_CMD -c $CONTEXT_NAME omni chat -f $COMMANDS_DIR/omni-chat.yaml -o $OUTPUT_DIR/omni_output_${RUNTIME}.pcm"
    else
        log_warn "跳过 Omni Chat 测试：需要音频输入文件"
        log_info "你可以创建 $COMMANDS_DIR/omni-chat.yaml 文件来测试"
    fi
    
    log_success "阶段 1 完成"
    echo ""
}

# 运行所有测试
run_tests() {
    local test_level="$1"
    
    setup_context
    
    case "$test_level" in
        1) test_level_1 ;;
        all)
            test_level_1
            ;;
        quick)
            log_info "快速测试：只检查 CLI 是否正常启动"
            $DASHSCOPE_CMD --help > /dev/null && log_success "CLI 正常" || log_error "CLI 异常"
            $DASHSCOPE_CMD config --help > /dev/null && log_success "config 命令正常" || log_error "config 命令异常"
            $DASHSCOPE_CMD omni --help > /dev/null && log_success "omni 命令正常" || log_error "omni 命令异常"
            ;;
        *)
            return 1
            ;;
    esac
}

# =====================================
# 主程序
# =====================================
main() {
    local runtime="${1:-go}"
    local test_level="${2:-quick}"
    
    echo ""
    echo "======================================"
    echo "   DashScope API 示例测试脚本"
    echo "======================================"
    echo ""
    echo "运行时:   $runtime"
    echo "测试级别: $test_level"
    echo "请求目录: $COMMANDS_DIR"
    echo "输出目录: $OUTPUT_DIR"
    echo "上下文名: $CONTEXT_NAME"
    echo ""
    
    case "$runtime" in
        go|rust)
            setup_runtime "$runtime"
            run_tests "$test_level"
            ;;
        both)
            echo "===== 使用 Go CLI 测试 ====="
            setup_runtime "go"
            run_tests "$test_level"
            
            echo ""
            echo "===== 使用 Rust CLI 测试 ====="
            setup_runtime "rust"
            run_tests "$test_level"
            ;;
        *)
            echo "用法: $0 [runtime] [test_level]"
            echo "      bazel run //examples/cmd/dashscope:run -- [runtime] [test_level]"
            echo ""
            echo "runtime:"
            echo "  go    - 使用 Go CLI (默认)"
            echo "  rust  - 使用 Rust CLI"
            echo "  both  - 同时测试 Go 和 Rust"
            echo ""
            echo "test_level:"
            echo "  1     - 实时语音测试 (Omni Chat)"
            echo "  all   - 全部测试"
            echo "  quick - 快速测试 (只检查 CLI 启动)"
            echo ""
            echo "示例:"
            echo "  $0 go quick                                        # 直接运行"
            echo "  bazel run //examples/cmd/dashscope:run -- go quick # Bazel 运行"
            echo "  bazel run //examples/cmd/dashscope:run -- both all"
            exit 1
            ;;
    esac
    
    echo ""
    echo "======================================"
    echo "   测试完成"
    echo "======================================"
    echo ""
    echo "输出文件保存在: $OUTPUT_DIR"
    ls -la "$OUTPUT_DIR" 2>/dev/null || true
}

main "$@"
