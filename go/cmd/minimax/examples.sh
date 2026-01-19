#!/bin/bash

# MiniMax API 示例脚本
# 
# 使用方式: ./examples.sh [test_level]
# test_level: 1=基础测试, 2=图片生成, 3=流式测试, 4=视频任务, 5=声音管理, 6=音色克隆, 7=文件管理, 8=音乐生成, all=全部
#
# 前置条件: 需要先配置 context
#   go run ./main.go config add-context minimax_cn --api-key YOUR_API_KEY
#
# 或者通过环境变量设置 API_KEY（会自动创建 context）:
#   MINIMAX_API_KEY=xxx ./examples.sh all

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 配置
CONTEXT_NAME="${MINIMAX_CONTEXT:-minimax_cn}"
API_KEY="${MINIMAX_API_KEY:-}"

# 切换到脚本所在目录
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

EXAMPLES_DIR="$SCRIPT_DIR/examples"
OUTPUT_DIR="$SCRIPT_DIR/output"
MINIMAX_CMD="go run ./main.go"

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

# =====================================
# 阶段 0: 配置上下文
# =====================================
setup_context() {
    log_info "=== 阶段 0: 配置上下文 ==="
    
    if [ -n "$API_KEY" ]; then
        $MINIMAX_CMD config add-context "$CONTEXT_NAME" --api-key "$API_KEY" 2>/dev/null || true
    fi
    
    $MINIMAX_CMD config use-context "$CONTEXT_NAME" 2>/dev/null
    
    if ! $MINIMAX_CMD config get-context "$CONTEXT_NAME" >/dev/null 2>&1; then
        log_error "Context '$CONTEXT_NAME' 不存在！请先运行:"
        echo "  go run ./main.go config add-context $CONTEXT_NAME --api-key YOUR_API_KEY"
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
        "$MINIMAX_CMD -c $CONTEXT_NAME speech synthesize -f $EXAMPLES_DIR/speech.yaml -o $OUTPUT_DIR/speech.mp3"
    
    run_test_verbose "文本聊天" \
        "$MINIMAX_CMD -c $CONTEXT_NAME text chat -f $EXAMPLES_DIR/chat.yaml"
    
    log_success "阶段 1 完成"
    echo ""
}

# =====================================
# 阶段 2: 图片生成测试
# =====================================
test_level_2() {
    log_info "=== 阶段 2: 图片生成测试 ==="
    
    run_test_verbose "图片生成" \
        "$MINIMAX_CMD -c $CONTEXT_NAME image generate -f $EXAMPLES_DIR/image.yaml"
    
    log_success "阶段 2 完成"
    echo ""
}

# =====================================
# 阶段 3: 流式测试
# =====================================
test_level_3() {
    log_info "=== 阶段 3: 流式测试 ==="
    
    run_test_verbose "流式 TTS" \
        "$MINIMAX_CMD -c $CONTEXT_NAME speech stream -f $EXAMPLES_DIR/speech.yaml -o $OUTPUT_DIR/speech_stream.mp3"
    
    run_test_verbose "流式文本聊天" \
        "$MINIMAX_CMD -c $CONTEXT_NAME text chat-stream -f $EXAMPLES_DIR/chat.yaml"
    
    log_success "阶段 3 完成"
    echo ""
}

# =====================================
# 阶段 4: 视频生成测试
# =====================================
test_level_4() {
    log_info "=== 阶段 4: 视频生成测试 ==="
    
    run_test_verbose "视频任务创建 (T2V)" \
        "$MINIMAX_CMD -c $CONTEXT_NAME video t2v -f $EXAMPLES_DIR/video-t2v.yaml"
    
    log_success "阶段 4 完成"
    echo ""
}

# =====================================
# 阶段 5: 声音管理测试
# =====================================
test_level_5() {
    log_info "=== 阶段 5: 声音管理测试 ==="
    
    run_test_verbose "获取系统音色列表" \
        "$MINIMAX_CMD -c $CONTEXT_NAME voice list --type system --json | head -20"
    
    run_test_verbose "获取所有音色列表" \
        "$MINIMAX_CMD -c $CONTEXT_NAME voice list --type all --json | head -20"
    
    # 音色设计测试 - 使用 examples 中的模板，但生成唯一 ID
    DESIGNED_VOICE_ID="test_design_$(date +%s)"
    TEMP_DESIGN_FILE="$OUTPUT_DIR/voice_design_temp.yaml"
    
    # 复制并修改 voice_id
    sed "s/voice_id:.*/voice_id: $DESIGNED_VOICE_ID/" "$EXAMPLES_DIR/voice-design.yaml" > "$TEMP_DESIGN_FILE"
    
    log_info "测试: 音色设计（创建新音色: $DESIGNED_VOICE_ID）"
    DESIGN_RESULT=$($MINIMAX_CMD -c $CONTEXT_NAME voice design -f "$TEMP_DESIGN_FILE" --json 2>&1) || true
    
    if echo "$DESIGN_RESULT" | grep -q "voice_id\|demo_audio"; then
        log_success "音色设计成功"
        echo "$DESIGN_RESULT" | head -5
        
        log_info "测试: 删除设计的音色"
        if $MINIMAX_CMD -c $CONTEXT_NAME voice delete "$DESIGNED_VOICE_ID" --type voice_generation 2>&1; then
            log_success "删除设计音色成功"
        else
            log_warn "删除设计音色失败（可能音色尚未激活）"
        fi
    else
        log_warn "音色设计跳过（需要付费功能）"
    fi
    
    rm -f "$TEMP_DESIGN_FILE"
    
    log_success "阶段 5 完成"
    echo ""
}

# =====================================
# 阶段 6: 音色克隆测试
# =====================================
test_level_6() {
    log_info "=== 阶段 6: 音色克隆测试 ==="
    
    # 先生成一段较长的音频用于克隆（使用 examples/clone-source.yaml）
    log_info "生成用于克隆的音频文件..."
    
    if ! $MINIMAX_CMD -c $CONTEXT_NAME speech synthesize -f "$EXAMPLES_DIR/clone-source.yaml" -o "$OUTPUT_DIR/clone_source.mp3" 2>&1; then
        log_error "生成克隆源音频失败"
        return 1
    fi
    log_success "生成克隆源音频成功"
    
    # 上传音频文件
    log_info "测试: 上传克隆音频"
    UPLOAD_RESULT=$($MINIMAX_CMD -c $CONTEXT_NAME voice upload "$OUTPUT_DIR/clone_source.mp3" --json 2>&1)
    
    if echo "$UPLOAD_RESULT" | grep -q "file_id"; then
        CLONE_FILE_ID=$(echo "$UPLOAD_RESULT" | grep -o '"file_id":[[:space:]]*[0-9]*' | grep -o '[0-9]*' | head -1)
        log_success "上传克隆音频成功，file_id: $CLONE_FILE_ID"
        
        if [ -n "$CLONE_FILE_ID" ]; then
            CLONE_VOICE_ID="test_clone_$(date +%s)"
            
            # 动态生成克隆配置（file_id 和 voice_id 是运行时确定的）
            TEMP_CLONE_FILE="$OUTPUT_DIR/voice_clone_temp.yaml"
            cat > "$TEMP_CLONE_FILE" << EOF
file_id: $CLONE_FILE_ID
voice_id: $CLONE_VOICE_ID
EOF
            
            log_info "测试: 执行音色克隆 (voice_id: $CLONE_VOICE_ID)"
            CLONE_RESULT=$($MINIMAX_CMD -c $CONTEXT_NAME voice clone -f "$TEMP_CLONE_FILE" --json 2>&1) || true
            
            if echo "$CLONE_RESULT" | grep -q "voice_id\|demo_audio"; then
                log_success "音色克隆成功"
                echo "$CLONE_RESULT" | head -5
                
                log_info "测试: 删除克隆的音色"
                if $MINIMAX_CMD -c $CONTEXT_NAME voice delete "$CLONE_VOICE_ID" --type voice_cloning 2>&1; then
                    log_success "删除克隆音色成功"
                else
                    log_warn "删除克隆音色失败"
                fi
            else
                log_warn "音色克隆跳过（需要付费功能）"
            fi
            
            rm -f "$TEMP_CLONE_FILE"
        fi
    else
        log_error "上传克隆音频失败"
        echo "$UPLOAD_RESULT"
    fi
    
    log_success "阶段 6 完成"
    echo ""
}

# =====================================
# 阶段 7: 文件管理测试
# =====================================
test_level_7() {
    log_info "=== 阶段 7: 文件管理测试 ==="
    
    # 使用已生成的音频文件测试
    if [ -f "$OUTPUT_DIR/speech.mp3" ]; then
        UPLOAD_RESULT=$($MINIMAX_CMD -c $CONTEXT_NAME file upload "$OUTPUT_DIR/speech.mp3" --purpose voice_clone --json 2>&1)
        
        if echo "$UPLOAD_RESULT" | grep -q "file_id"; then
            TEST_FILE_ID=$(echo "$UPLOAD_RESULT" | grep -o '"file_id":[[:space:]]*[0-9]*' | grep -o '[0-9]*' | head -1)
            log_success "上传文件成功，file_id: $TEST_FILE_ID"
            
            run_test_verbose "列出文件 (voice_clone)" \
                "$MINIMAX_CMD -c $CONTEXT_NAME file list --purpose voice_clone --json | head -20"
            
            if [ -n "$TEST_FILE_ID" ]; then
                run_test_verbose "获取文件信息" \
                    "$MINIMAX_CMD -c $CONTEXT_NAME file get $TEST_FILE_ID --json"
                
                run_test_verbose "删除文件" \
                    "$MINIMAX_CMD -c $CONTEXT_NAME file delete $TEST_FILE_ID --purpose voice_clone"
            fi
        else
            log_error "上传文件失败"
            echo "$UPLOAD_RESULT"
        fi
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
        "$MINIMAX_CMD -c $CONTEXT_NAME music generate -f $EXAMPLES_DIR/music.yaml -o $OUTPUT_DIR/music.mp3"
    
    log_success "阶段 8 完成"
    echo ""
}

# =====================================
# 主程序
# =====================================
main() {
    local test_level="${1:-all}"
    
    echo ""
    echo "======================================"
    echo "   MiniMax API 示例脚本"
    echo "======================================"
    echo ""
    echo "测试级别: $test_level"
    echo "示例目录: $EXAMPLES_DIR"
    echo "输出目录: $OUTPUT_DIR"
    echo "上下文名: $CONTEXT_NAME"
    echo ""
    
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
            echo "用法: $0 [1|2|3|4|5|6|7|8|all|quick]"
            echo ""
            echo "  1     - 基础测试 (TTS, Chat)"
            echo "  2     - 图片生成测试"
            echo "  3     - 流式测试"
            echo "  4     - 视频任务测试"
            echo "  5     - 声音管理测试 (列表、设计、删除)"
            echo "  6     - 音色克隆测试 (上传、克隆、删除)"
            echo "  7     - 文件管理测试 (上传、列表、获取、删除)"
            echo "  8     - 音乐生成测试"
            echo "  all   - 全部测试"
            echo "  quick - 快速测试 (基础 + 声音管理)"
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
