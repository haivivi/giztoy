#!/bin/bash

# Doubao Speech API 示例测试脚本
# 
# 支持同时测试 Go 和 Rust CLI
#
# 使用方式:
#   直接运行: ./run.sh [runtime] [test_level]
#   Bazel:    bazel run //examples/cmd/doubaospeech:run -- [runtime] [test_level]
#
#   runtime: go | rust | both (默认: go)
#   test_level: 1-6, all, quick
#
# 前置条件: 需要先配置 context
#   doubaospeech config add-context test --app-id YOUR_APP_ID --api-key YOUR_API_KEY

set -euo pipefail

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# 配置
CONTEXT_NAME="${DOUBAO_CONTEXT:-test}"
API_KEY="${DOUBAO_API_KEY:-}"
APP_ID="${DOUBAO_APP_ID:-}"

# 获取目录路径
if [ -n "${BUILD_WORKSPACE_DIRECTORY:-}" ]; then
    # Bazel 环境
    PROJECT_ROOT="$BUILD_WORKSPACE_DIRECTORY"
else
    # 直接运行
    SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
    PROJECT_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"
fi

SCRIPT_DIR="$PROJECT_ROOT/examples/cmd/doubaospeech"
COMMANDS_DIR="$SCRIPT_DIR/commands"
OUTPUT_DIR="$SCRIPT_DIR/output"

# 构建 CLI（如果需要）
build_cli() {
    local target="$1"
    case "$target" in
        go)
            if [ ! -f "$PROJECT_ROOT/bazel-bin/go/cmd/doubaospeech/doubaospeech_/doubaospeech" ]; then
                log_info "构建 Go CLI (bazel build //go/cmd/doubaospeech)..."
                (cd "$PROJECT_ROOT" && bazel build //go/cmd/doubaospeech)
            fi
            ;;
        rust)
            # 优先使用 Bazel 构建
            if [ ! -f "$PROJECT_ROOT/bazel-bin/rust/cmd/doubaospeech/doubaospeech" ]; then
                log_info "构建 Rust CLI (bazel build //rust/cmd/doubaospeech)..."
                if ! (cd "$PROJECT_ROOT" && bazel build //rust/cmd/doubaospeech); then
                    log_error "Bazel 构建失败，请检查构建环境..."
                    exit 1
                fi
            fi
            ;;
    esac
}

# CLI 命令路径
GO_CMD="$PROJECT_ROOT/bazel-bin/go/cmd/doubaospeech/doubaospeech_/doubaospeech"
RUST_CMD="$PROJECT_ROOT/bazel-bin/rust/cmd/doubaospeech/doubaospeech"

# 当前使用的命令
DOUBAO_CMD=""
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

# 显示帮助信息
show_help() {
    echo "用法: $0 [runtime] [test_level]"
    echo ""
    echo "runtime:"
    echo "  go         - 使用 Go CLI"
    echo "  rust       - 使用 Rust CLI"
    echo "  both       - 同时测试 Go 和 Rust (默认)"
    echo ""
    echo "test_level:"
    echo "  1          - TTS 基础测试 (同步合成)"
    echo "  2          - TTS 流式测试"
    echo "  3          - ASR 测试 (单句识别)"
    echo "  4          - 会议转写测试"
    echo "  5          - 播客合成测试"
    echo "  6          - 字幕提取测试"
    echo "  7          - Realtime 测试 (仅 Rust)"
    echo "  all        - 全部测试 (默认)"
    echo "  quick      - 快速测试 (TTS + ASR)"
    echo "  realtime   - Realtime 测试"
    echo ""
    echo "环境变量:"
    echo "  DOUBAO_CONTEXT   - 上下文名称 (默认: test)"
    echo "  DOUBAO_APP_ID    - App ID"
    echo "  DOUBAO_API_KEY   - API Key"
    echo ""
    echo "示例:"
    echo "  $0 go 1                                              # 直接运行"
    echo "  bazel run //examples/cmd/doubaospeech:run -- go 1    # Bazel 运行"
    echo "  bazel run //examples/cmd/doubaospeech:run -- rust quick"
    echo "  bazel run //examples/cmd/doubaospeech:run -- both all"
}

run_test_verbose() {
    local name="$1"
    local cmd="$2"
    
    log_info "测试: $name"
    
    # Use bash -c to safely execute the command string
    # This avoids word-splitting issues with paths containing spaces
    if bash -c "$cmd"; then
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
            DOUBAO_CMD="$GO_CMD"
            log_runtime "使用 Go CLI (Bazel build)"
            ;;
        rust)
            build_cli rust
            DOUBAO_CMD="$RUST_CMD"
            log_runtime "使用 Rust CLI (Bazel build)"
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
    
    if [ -n "$API_KEY" ] && [ -n "$APP_ID" ]; then
        $DOUBAO_CMD config add-context "$CONTEXT_NAME" --app-id "$APP_ID" --api-key "$API_KEY" 2>/dev/null || true
    fi
    
    $DOUBAO_CMD config use-context "$CONTEXT_NAME" 2>/dev/null || true
    
    # 检查 context 是否存在
    if ! $DOUBAO_CMD config list-contexts 2>/dev/null | grep -q "$CONTEXT_NAME"; then
        log_error "Context '$CONTEXT_NAME' 不存在！请先运行:"
        echo "  $DOUBAO_CMD config add-context $CONTEXT_NAME --app-id YOUR_APP_ID --api-key YOUR_API_KEY"
        exit 1
    fi
    
    log_success "上下文配置完成: $CONTEXT_NAME"
    echo ""
}

# =====================================
# 阶段 1: TTS 基础测试
# =====================================
test_level_1() {
    log_info "=== 阶段 1: TTS 基础测试 ==="
    
    run_test_verbose "TTS 语音合成 (同步)" \
        "$DOUBAO_CMD -c $CONTEXT_NAME tts synthesize -f $COMMANDS_DIR/tts.yaml -o $OUTPUT_DIR/tts_${RUNTIME}.mp3"
    
    log_success "阶段 1 完成"
    echo ""
}

# =====================================
# 阶段 2: TTS 流式测试
# =====================================
test_level_2() {
    log_info "=== 阶段 2: TTS 流式测试 ==="
    
    run_test_verbose "TTS 流式合成" \
        "$DOUBAO_CMD -c $CONTEXT_NAME tts stream -f $COMMANDS_DIR/tts.yaml -o $OUTPUT_DIR/tts_stream_${RUNTIME}.mp3"
    
    log_success "阶段 2 完成"
    echo ""
}

# =====================================
# 阶段 3: ASR 测试
# =====================================
test_level_3() {
    log_info "=== 阶段 3: ASR 测试 ==="
    
    run_test_verbose "ASR 单句识别" \
        "$DOUBAO_CMD -c $CONTEXT_NAME asr one-sentence -f $COMMANDS_DIR/asr-one-sentence.yaml --json"
    
    log_success "阶段 3 完成"
    echo ""
}

# =====================================
# 阶段 4: 会议转写测试
# =====================================
test_level_4() {
    log_info "=== 阶段 4: 会议转写测试 ==="
    
    run_test_verbose "会议转写任务创建" \
        "$DOUBAO_CMD -c $CONTEXT_NAME meeting create -f $COMMANDS_DIR/meeting.yaml --json"
    
    log_success "阶段 4 完成"
    echo ""
}

# =====================================
# 阶段 5: 播客合成测试
# =====================================
test_level_5() {
    log_info "=== 阶段 5: 播客合成测试 ==="
    
    run_test_verbose "播客合成任务创建" \
        "$DOUBAO_CMD -c $CONTEXT_NAME podcast create -f $COMMANDS_DIR/podcast.yaml --json"
    
    log_success "阶段 5 完成"
    echo ""
}

# =====================================
# 阶段 6: 字幕提取测试
# =====================================
test_level_6() {
    log_info "=== 阶段 6: 字幕提取测试 ==="
    
    run_test_verbose "字幕提取任务创建" \
        "$DOUBAO_CMD -c $CONTEXT_NAME media subtitle -f $COMMANDS_DIR/subtitle.yaml --json"
    
    log_success "阶段 6 完成"
    echo ""
}

# =====================================
# 阶段 7: Realtime 测试 (仅 Rust)
# =====================================
test_level_7() {
    log_info "=== 阶段 7: Realtime 测试 ==="
    
    # Realtime 目前只在 Rust CLI 实现
    if [ "$RUNTIME" = "rust" ]; then
        run_test_verbose "Realtime 测试 (发送问候)" \
            "$DOUBAO_CMD -c $CONTEXT_NAME realtime test -f $COMMANDS_DIR/realtime.yaml -g '你好，今天天气怎么样？' --json"
    else
        log_warn "Realtime 测试仅支持 Rust CLI (Go CLI 暂未实现)"
    fi
    
    log_success "阶段 7 完成"
    echo ""
}

# 运行所有测试
run_tests() {
    local test_level="$1"
    
    setup_context
    
    case "$test_level" in
        1) test_level_1 ;;
        2) test_level_2 ;;
        3) test_level_3 ;;
        4) test_level_4 ;;
        5) test_level_5 ;;
        6) test_level_6 ;;
        7) test_level_7 ;;
        all)
            test_level_1
            test_level_2
            test_level_3
            test_level_4
            test_level_5
            test_level_6
            test_level_7
            ;;
        quick)
            test_level_1
            test_level_3
            ;;
        realtime)
            test_level_7
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
    local test_level="${2:-all}"
    
    echo ""
    echo "======================================"
    echo "   Doubao Speech API 示例测试脚本"
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
            if ! run_tests "$test_level"; then
                echo "Error: Unknown test level: $test_level"
                show_help
                exit 1
            fi
            ;;
        both)
            echo "===== 使用 Go CLI 测试 ====="
            setup_runtime "go"
            if ! run_tests "$test_level"; then
                echo "Error: Unknown test level: $test_level"
                show_help
                exit 1
            fi
            
            echo ""
            echo "===== 使用 Rust CLI 测试 ====="
            setup_runtime "rust"
            if ! run_tests "$test_level"; then
                echo "Error: Unknown test level: $test_level"
                show_help
                exit 1
            fi
            ;;
        *)
            echo "Error: Unknown runtime: $runtime"
            show_help
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
