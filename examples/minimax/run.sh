#!/bin/bash

# MiniMax API 示例测试脚本
# 
# 支持同时测试 Go 和 Rust CLI
#
# 使用方式:
#   直接运行: ./run.sh [runtime] [test_level]
#   Bazel:    bazel run //examples/minimax:run -- [runtime] [test_level]
#
#   runtime: go | rust | both (默认: go)
#   test_level: 1-8, all, quick
#
# 前置条件: 需要先配置 context
#   minimax config add-context minimax_cn --api-key YOUR_API_KEY

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# 配置
CONTEXT_NAME="${MINIMAX_CONTEXT:-minimax_cn}"
API_KEY="${MINIMAX_API_KEY:-}"

# 获取目录路径
if [ -n "$BUILD_WORKSPACE_DIRECTORY" ]; then
    # Bazel 环境
    PROJECT_ROOT="$BUILD_WORKSPACE_DIRECTORY"
else
    # 直接运行
    SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
    PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
fi

SCRIPT_DIR="$PROJECT_ROOT/examples/minimax"
COMMANDS_DIR="$SCRIPT_DIR/commands"
OUTPUT_DIR="$SCRIPT_DIR/output"

# 构建 CLI（如果需要）
build_cli() {
    local target="$1"
    case "$target" in
        go)
            if [ ! -f "$PROJECT_ROOT/bazel-bin/go/cmd/minimax/minimax_/minimax" ]; then
                log_info "构建 Go CLI (bazel build //go/cmd/minimax)..."
                (cd "$PROJECT_ROOT" && bazel build //go/cmd/minimax)
            fi
            ;;
        rust)
            # 优先使用 Bazel 构建，如果失败则回退到 Cargo
            if [ ! -f "$PROJECT_ROOT/bazel-bin/rust/cmd/minimax/minimax" ] && [ ! -f "$PROJECT_ROOT/rust/target/release/minimax" ]; then
                log_info "构建 Rust CLI (bazel build //rust/cmd/minimax)..."
                if ! (cd "$PROJECT_ROOT" && bazel build //rust/cmd/minimax); then
                    log_warn "Bazel 构建失败，回退到 Cargo..."
                    if ! (cd "$PROJECT_ROOT/rust" && cargo build --release --bin minimax); then
                        log_error "Cargo 构建也失败，请检查构建环境..."
                        exit 1
                    fi
                fi
            fi
            ;;
    esac
}

# CLI 命令路径
GO_CMD="$PROJECT_ROOT/bazel-bin/go/cmd/minimax/minimax_/minimax"

# Rust CLI: 优先 Bazel，回退 Cargo
rust_cmd() {
    if [ -f "$PROJECT_ROOT/bazel-bin/rust/cmd/minimax/minimax" ]; then
        echo "$PROJECT_ROOT/bazel-bin/rust/cmd/minimax/minimax"
    else
        echo "$PROJECT_ROOT/rust/target/release/minimax"
    fi
}

# 当前使用的命令
MINIMAX_CMD=""
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
            MINIMAX_CMD="$GO_CMD"
            log_runtime "使用 Go CLI (Bazel build)"
            ;;
        rust)
            build_cli rust
            MINIMAX_CMD="$(rust_cmd)"
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
        $MINIMAX_CMD config add-context "$CONTEXT_NAME" --api-key "$API_KEY" 2>/dev/null || true
    fi
    
    $MINIMAX_CMD config use-context "$CONTEXT_NAME" 2>/dev/null || true
    
    # 检查 context 是否存在
    if ! $MINIMAX_CMD config list-contexts 2>/dev/null | grep -q "$CONTEXT_NAME"; then
        log_error "Context '$CONTEXT_NAME' 不存在！请先运行:"
        echo "  minimax config add-context $CONTEXT_NAME --api-key YOUR_API_KEY"
        exit 1
    fi
    
    log_success "上下文配置完成: $CONTEXT_NAME"
    echo ""
}

# =====================================
# 阶段 1: 基础测试 (TTS, Chat)
# =====================================
test_level_1() {
    log_info "=== 阶段 1: 基础测试 (TTS, Chat) ==="
    
    run_test_verbose "TTS 语音合成" \
        "$MINIMAX_CMD -c $CONTEXT_NAME speech synthesize -f $COMMANDS_DIR/speech.yaml -o $OUTPUT_DIR/speech_${RUNTIME}.mp3"
    
    run_test_verbose "文本聊天" \
        "$MINIMAX_CMD -c $CONTEXT_NAME text chat -f $COMMANDS_DIR/chat.yaml"
    
    log_success "阶段 1 完成"
    echo ""
}

# =====================================
# 阶段 2: 图片生成测试
# =====================================
test_level_2() {
    log_info "=== 阶段 2: 图片生成测试 ==="
    
    run_test_verbose "图片生成" \
        "$MINIMAX_CMD -c $CONTEXT_NAME image generate -f $COMMANDS_DIR/image.yaml"
    
    log_success "阶段 2 完成"
    echo ""
}

# =====================================
# 阶段 3: 流式测试
# =====================================
test_level_3() {
    log_info "=== 阶段 3: 流式测试 ==="
    
    run_test_verbose "流式 TTS" \
        "$MINIMAX_CMD -c $CONTEXT_NAME speech stream -f $COMMANDS_DIR/speech.yaml -o $OUTPUT_DIR/speech_stream_${RUNTIME}.mp3"
    
    run_test_verbose "流式文本聊天" \
        "$MINIMAX_CMD -c $CONTEXT_NAME text stream -f $COMMANDS_DIR/chat.yaml"
    
    log_success "阶段 3 完成"
    echo ""
}

# =====================================
# 阶段 4: 视频生成测试
# =====================================
test_level_4() {
    log_info "=== 阶段 4: 视频生成测试 ==="
    
    run_test_verbose "视频任务创建 (T2V)" \
        "$MINIMAX_CMD -c $CONTEXT_NAME video t2v -f $COMMANDS_DIR/video-t2v.yaml"
    
    log_success "阶段 4 完成"
    echo ""
}

# =====================================
# 阶段 5: 声音管理测试
# =====================================
test_level_5() {
    log_info "=== 阶段 5: 声音管理测试 ==="
    
    run_test_verbose "获取音色列表" \
        "$MINIMAX_CMD -c $CONTEXT_NAME voice list --json | head -20"
    
    log_success "阶段 5 完成"
    echo ""
}

# =====================================
# 阶段 6: 音色克隆测试
# =====================================
test_level_6() {
    log_info "=== 阶段 6: 音色克隆测试 ==="
    
    # 先生成一段较长的音频用于克隆
    log_info "生成用于克隆的音频文件..."
    
    if ! $MINIMAX_CMD -c $CONTEXT_NAME speech synthesize -f "$COMMANDS_DIR/clone-source.yaml" -o "$OUTPUT_DIR/clone_source_${RUNTIME}.mp3" 2>&1; then
        log_error "生成克隆源音频失败"
        return 1
    fi
    log_success "生成克隆源音频成功"
    
    # 上传音频文件
    log_info "测试: 上传克隆音频"
    run_test_verbose "上传克隆音频" \
        "$MINIMAX_CMD -c $CONTEXT_NAME voice upload-clone-source --file $OUTPUT_DIR/clone_source_${RUNTIME}.mp3"
    
    log_success "阶段 6 完成"
    echo ""
}

# =====================================
# 阶段 7: 文件管理测试
# =====================================
test_level_7() {
    log_info "=== 阶段 7: 文件管理测试 ==="
    
    # 使用已生成的音频文件测试
    if [ -f "$OUTPUT_DIR/speech_${RUNTIME}.mp3" ]; then
        run_test_verbose "上传文件" \
            "$MINIMAX_CMD -c $CONTEXT_NAME file upload --file $OUTPUT_DIR/speech_${RUNTIME}.mp3 --purpose voice_clone"
        
        run_test_verbose "列出文件" \
            "$MINIMAX_CMD -c $CONTEXT_NAME file list --json | head -20"
    else
        log_warn "跳过文件管理测试：需要先运行阶段 1 生成 speech.mp3"
    fi
    
    log_success "阶段 7 完成"
    echo ""
}

# =====================================
# 阶段 8: 音乐生成测试
# =====================================
test_level_8() {
    log_info "=== 阶段 8: 音乐生成测试 ==="
    
    run_test_verbose "音乐生成" \
        "$MINIMAX_CMD -c $CONTEXT_NAME music generate -f $COMMANDS_DIR/music.yaml -o $OUTPUT_DIR/music_${RUNTIME}.mp3"
    
    log_success "阶段 8 完成"
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
        8) test_level_8 ;;
        all)
            test_level_1
            test_level_2
            test_level_3
            test_level_4
            test_level_5
            test_level_6
            test_level_7
            test_level_8
            ;;
        quick)
            test_level_1
            test_level_5
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
    echo "   MiniMax API 示例测试脚本"
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
            echo "      bazel run //examples/minimax:run -- [runtime] [test_level]"
            echo ""
            echo "runtime:"
            echo "  go    - 使用 Go CLI (默认)"
            echo "  rust  - 使用 Rust CLI"
            echo "  both  - 同时测试 Go 和 Rust"
            echo ""
            echo "test_level:"
            echo "  1     - 基础测试 (TTS, Chat)"
            echo "  2     - 图片生成测试"
            echo "  3     - 流式测试"
            echo "  4     - 视频任务测试"
            echo "  5     - 声音管理测试"
            echo "  6     - 音色克隆测试"
            echo "  7     - 文件管理测试"
            echo "  8     - 音乐生成测试"
            echo "  all   - 全部测试 (默认)"
            echo "  quick - 快速测试 (基础 + 声音管理)"
            echo ""
            echo "示例:"
            echo "  $0 go 1                                    # 直接运行"
            echo "  bazel run //examples/minimax:run -- go 1   # Bazel 运行"
            echo "  bazel run //examples/minimax:run -- both quick"
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
