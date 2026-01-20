#!/bin/bash
#
# Doubao CLI Test Script
# Test doubao CLI 命令
#
# Usage:
#   ./run.sh [test_name]
#   ./run.sh tts          # Test TTS 命令
#   ./run.sh asr          # Test ASR 命令
#   ./run.sh all          # Run所有测试
#
# 也可以通过 Bazel 运行:
#   bazel run //examples/cmd/doubaospeech:run -- tts

set -euo pipefail

# ==================== Configuration ====================

# Determine script/config directory (handle both direct and Bazel execution)
if [[ -n "${BUILD_WORKSPACE_DIRECTORY:-}" ]]; then
    # Running via bazel run
    SCRIPT_DIR="$BUILD_WORKSPACE_DIRECTORY/examples/cmd/doubaospeech"
else
    SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
fi
OUTPUT_DIR="${DOUBAO_OUTPUT_DIR:-/tmp/doubao_output}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# ==================== Logging ====================

log_info() { echo -e "${BLUE}[INFO]${NC} $*"; }
log_success() { echo -e "${GREEN}[PASS]${NC} $*"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $*"; }
log_error() { echo -e "${RED}[FAIL]${NC} $*"; }
log_section() { echo -e "\n${BLUE}========================================${NC}"; echo -e "${BLUE}  $*${NC}"; echo -e "${BLUE}========================================${NC}\n"; }

# ==================== Setup ====================

# Find doubao CLI binary
find_doubao_cli() {
    # Check if running via Bazel
    if [[ -n "${BUILD_WORKSPACE_DIRECTORY:-}" ]]; then
        # Running via bazel run, use bazel-bin
        local cli="$BUILD_WORKSPACE_DIRECTORY/bazel-bin/go/cmd/doubao/doubao_/doubao"
        if [[ -x "$cli" ]]; then
            echo "$cli"
            return 0
        fi
    fi
    
    # Check PATH
    if command -v doubao &>/dev/null; then
        echo "doubao"
        return 0
    fi
    
    # Check bazel-bin relative to script (examples/cmd/doubaospeech -> project root)
    local workspace_root="$SCRIPT_DIR/../../.."
    local cli="$workspace_root/bazel-bin/go/cmd/doubao/doubao_/doubao"
    if [[ -x "$cli" ]]; then
        echo "$cli"
        return 0
    fi
    
    return 1
}

setup() {
    mkdir -p "$OUTPUT_DIR"
    
    # Find CLI
    DOUBAO_CLI=$(find_doubao_cli) || {
        log_error "doubao CLI not found. Build it first:"
        echo "  bazel build //go/cmd/doubao:doubao"
        exit 1
    }
    
    log_info "Using CLI: $DOUBAO_CLI"
    log_info "Output dir: $OUTPUT_DIR"
    log_info "Config dir: $SCRIPT_DIR"
    
    # Check if context is configured
    if ! "$DOUBAO_CLI" config get-context &>/dev/null; then
        log_warn "No context configured. Set up with:"
        echo "  $DOUBAO_CLI config add-context test --app-id YOUR_APP_ID --api-key YOUR_API_KEY"
        echo "  $DOUBAO_CLI config use-context test"
    fi
}

# ==================== Test Functions ====================

test_tts_synthesize() {
    log_section "TTS Synthesize (volcano_mega)"
    
    local config="$SCRIPT_DIR/commands/tts.yaml"
    local output="$OUTPUT_DIR/tts_output.mp3"
    
    log_info "Config: $config"
    log_info "Output: $output"
    
    local result
    if result=$("$DOUBAO_CLI" tts synthesize -f "$config" -o "$output" -v 2>&1); then
        if [[ -f "$output" ]]; then
            local size=$(wc -c < "$output" | tr -d ' ')
            log_success "TTS synthesize completed ($size bytes)"
            record_result "TTS 2.0 大模型" "PASS" "$output ($size bytes)"
        else
            log_warn "Output file not created"
            record_result "TTS 2.0 大模型" "FAIL" "" "No output file"
        fi
    else
        local error_type=$(parse_error "$result")
        log_error "TTS synthesize failed"
        echo "$result" | sed 's/^/  /' | head -3
        record_result "TTS 2.0 大模型" "$error_type" "" "$result"
    fi
}

test_tts_stream() {
    log_section "TTS Stream (HTTP)"
    
    local config="$SCRIPT_DIR/commands/tts.yaml"
    local output="$OUTPUT_DIR/tts_stream_output.mp3"
    
    log_info "Config: $config"
    log_info "Output: $output"
    
    local result
    if result=$("$DOUBAO_CLI" tts stream -f "$config" -o "$output" -v 2>&1); then
        if [[ -f "$output" ]]; then
            local size=$(wc -c < "$output" | tr -d ' ')
            log_success "TTS stream completed ($size bytes)"
            record_result "TTS Stream" "PASS" "$output ($size bytes)"
        else
            log_warn "Output file not created"
            record_result "TTS Stream" "FAIL" "" "No output file"
        fi
    else
        local error_type=$(parse_error "$result")
        log_error "TTS stream failed"
        record_result "TTS Stream" "$error_type" "" "$result"
    fi
}

test_tts_async() {
    log_section "TTS Async"
    
    local config="$SCRIPT_DIR/commands/tts-async.yaml"
    
    log_info "Config: $config"
    
    local result
    if result=$("$DOUBAO_CLI" tts async -f "$config" -v --json 2>&1) && echo "$result" > "$OUTPUT_DIR/tts_async_response.json"; then
        log_success "TTS async task submitted"
        echo "$result" | jq . 2>/dev/null || echo "$result"
        record_result "TTS Async" "PASS" "tts_async_response.json"
    else
        local error_type=$(parse_error "$result")
        log_error "TTS async failed"
        record_result "TTS Async" "$error_type" "" "$result"
    fi
}

test_asr_one_sentence() {
    log_section "ASR One Sentence"
    
    local config="$SCRIPT_DIR/commands/asr-one-sentence.yaml"
    
    log_info "Config: $config"
    
    local result
    if result=$("$DOUBAO_CLI" asr one-sentence -f "$config" -v --json 2>&1) && echo "$result" > "$OUTPUT_DIR/asr_one_sentence_response.json"; then
        log_success "ASR one-sentence completed"
        echo "$result" | jq . 2>/dev/null || echo "$result"
        record_result "ASR One-Sentence" "PASS" "asr_one_sentence_response.json"
    else
        local error_type=$(parse_error "$result")
        log_error "ASR one-sentence failed"
        record_result "ASR One-Sentence" "$error_type" "" "$result"
    fi
}

test_asr_stream() {
    log_section "ASR Stream"
    
    local config="$SCRIPT_DIR/commands/asr-stream.yaml"
    
    log_info "Config: $config"
    
    local result
    if result=$("$DOUBAO_CLI" asr stream -f "$config" -v --json 2>&1) && echo "$result" > "$OUTPUT_DIR/asr_stream_response.json"; then
        log_success "ASR stream completed"
        echo "$result" | jq . 2>/dev/null || echo "$result"
        record_result "ASR Stream" "PASS" "asr_stream_response.json"
    else
        local error_type=$(parse_error "$result")
        log_error "ASR stream failed"
        record_result "ASR Stream" "$error_type" "" "$result"
    fi
}

test_realtime() {
    log_section "Realtime Conversation"
    
    local config="$SCRIPT_DIR/commands/realtime.yaml"
    
    log_info "Config: $config"
    log_info "This is an interactive test, press Ctrl+C to exit"
    
    record_result "Realtime V3" "SKIPPED" "" "Interactive test"
    "$DOUBAO_CLI" realtime start -f "$config" -v || log_warn "Realtime test ended"
}

test_podcast() {
    log_section "Podcast Synthesis"
    
    local config="$SCRIPT_DIR/commands/podcast.yaml"
    
    log_info "Config: $config"
    
    local result
    if result=$("$DOUBAO_CLI" podcast create -f "$config" -v --json 2>&1) && echo "$result" > "$OUTPUT_DIR/podcast_response.json"; then
        log_success "Podcast task submitted"
        echo "$result" | jq . 2>/dev/null || echo "$result"
        record_result "Podcast" "PASS" "podcast_response.json"
    else
        local error_type=$(parse_error "$result")
        log_error "Podcast failed"
        record_result "Podcast" "$error_type" "" "$result"
    fi
}

test_meeting() {
    log_section "Meeting Transcription"
    
    local config="$SCRIPT_DIR/commands/meeting.yaml"
    
    log_info "Config: $config"
    
    local result
    if result=$("$DOUBAO_CLI" meeting create -f "$config" -v --json 2>&1) && echo "$result" > "$OUTPUT_DIR/meeting_response.json"; then
        log_success "Meeting task submitted"
        echo "$result" | jq . 2>/dev/null || echo "$result"
        record_result "Meeting" "PASS" "meeting_response.json"
    else
        local error_type=$(parse_error "$result")
        log_error "Meeting failed"
        record_result "Meeting" "$error_type" "" "$result"
    fi
}

test_translation() {
    log_section "Translation"
    
    local config="$SCRIPT_DIR/commands/translation.yaml"
    
    log_info "Config: $config"
    log_info "This is an interactive test, press Ctrl+C to exit"
    
    record_result "Translation" "SKIPPED" "" "Interactive test"
    "$DOUBAO_CLI" translation start -f "$config" -v || log_warn "Translation test ended"
}

test_subtitle() {
    log_section "Subtitle Extraction"
    
    local config="$SCRIPT_DIR/commands/subtitle.yaml"
    
    log_info "Config: $config"
    
    local result
    if result=$("$DOUBAO_CLI" media subtitle -f "$config" -v --json 2>&1) && echo "$result" > "$OUTPUT_DIR/subtitle_response.json"; then
        log_success "Subtitle task submitted"
        echo "$result" | jq . 2>/dev/null || echo "$result"
        record_result "Media Subtitle" "PASS" "subtitle_response.json"
    else
        local error_type=$(parse_error "$result")
        log_error "Subtitle failed"
        record_result "Media Subtitle" "$error_type" "" "$result"
    fi
}

test_voice_train() {
    log_section "Voice Clone Training"
    
    local config="$SCRIPT_DIR/commands/voice-train.yaml"
    
    log_info "Config: $config"
    
    local result
    if result=$("$DOUBAO_CLI" voice train -f "$config" -v --json 2>&1) && echo "$result" > "$OUTPUT_DIR/voice_train_response.json"; then
        log_success "Voice clone training submitted"
        echo "$result" | jq . 2>/dev/null || echo "$result"
        record_result "Voice Clone" "PASS" "voice_train_response.json"
    else
        local error_type=$(parse_error "$result")
        log_error "Voice clone training failed"
        record_result "Voice Clone" "$error_type" "" "$result"
    fi
}

# ==================== Result Tracking ====================

# Arrays to track test results
declare -a TEST_NAMES=()
declare -a TEST_STATUS=()  # PASS, FAIL, NOT_ENABLED, SKIPPED
declare -a TEST_OUTPUT=()
declare -a TEST_ERROR=()

# Record test result
record_result() {
    local name="$1"
    local status="$2"
    local output="${3:-}"
    local error="${4:-}"
    
    TEST_NAMES+=("$name")
    TEST_STATUS+=("$status")
    TEST_OUTPUT+=("$output")
    TEST_ERROR+=("$error")
}

# Parse error to determine if service not enabled
parse_error() {
    local error="$1"
    if [[ "$error" == *"resource not granted"* ]] || [[ "$error" == *"code=3001"* ]]; then
        echo "NOT_ENABLED"
    elif [[ "$error" == *"code=3050"* ]] || [[ "$error" == *"model not found"* ]]; then
        echo "INVALID_VOICE"
    else
        echo "FAIL"
    fi
}

# ==================== Summary ====================

print_summary() {
    log_section "Test Results"
    
    # Print table header
    echo ""
    printf "%-20s %-15s %-30s\n" "Service" "Status" "Output/Error"
    printf "%-20s %-15s %-30s\n" "-------" "------" "------------"
    
    local pass_count=0
    local fail_count=0
    local not_enabled_count=0
    
    for i in "${!TEST_NAMES[@]}"; do
        local name="${TEST_NAMES[$i]}"
        local status="${TEST_STATUS[$i]}"
        local output="${TEST_OUTPUT[$i]}"
        local error="${TEST_ERROR[$i]}"
        
        local status_str=""
        local detail=""
        
        case "$status" in
            PASS)
                status_str="${GREEN}✅ PASS${NC}"
                detail="$output"
                ((pass_count++))
                ;;
            FAIL)
                status_str="${RED}❌ FAIL${NC}"
                detail="$error"
                ((fail_count++))
                ;;
            NOT_ENABLED)
                status_str="${YELLOW}⚠️  Not enabled${NC}"
                detail="需要在控制台开通服务"
                ((not_enabled_count++))
                ;;
            INVALID_VOICE)
                status_str="${YELLOW}⚠️  音色错误${NC}"
                detail="$error"
                ((fail_count++))
                ;;
            SKIPPED)
                status_str="${BLUE}⏭️  跳过${NC}"
                detail="交互式测试"
                ;;
        esac
        
        # Truncate detail if too long
        if [[ ${#detail} -gt 40 ]]; then
            detail="${detail:0:37}..."
        fi
        
        printf "%-20s %-15b %-40s\n" "$name" "$status_str" "$detail"
    done
    
    echo ""
    printf "%-20s %-15s %-30s\n" "-------" "------" "------------"
    echo ""
    echo -e "Summary: ${GREEN}$pass_count passed${NC}, ${RED}$fail_count failed${NC}, ${YELLOW}$not_enabled_count not enabled${NC}"
    
    # Print output directory info
    echo ""
    log_section "Output Files"
    echo "Directory: $OUTPUT_DIR"
    echo ""
    
    if [[ -d "$OUTPUT_DIR" ]]; then
        shopt -s nullglob
        
        # Audio files
        local audio_files=("$OUTPUT_DIR"/*.mp3 "$OUTPUT_DIR"/*.wav "$OUTPUT_DIR"/*.ogg)
        if [[ ${#audio_files[@]} -gt 0 ]]; then
            echo "🎵 Audio files:"
            for f in "${audio_files[@]}"; do
                if [[ -f "$f" ]]; then
                    local size=$(wc -c < "$f" | tr -d ' ')
                    printf "   %-40s %10s bytes\n" "$(basename "$f")" "$size"
                fi
            done
            echo ""
            echo "   Play: ffplay '$OUTPUT_DIR/<file>'"
        fi
        
        # JSON files
        local json_files=("$OUTPUT_DIR"/*.json)
        if [[ ${#json_files[@]} -gt 0 ]]; then
            echo ""
            echo "📄 JSON responses:"
            for f in "${json_files[@]}"; do
                [[ -f "$f" ]] && printf "   %s\n" "$(basename "$f")"
            done
            echo ""
            echo "   View: cat '$OUTPUT_DIR/<file>' | jq"
        fi
        
        shopt -u nullglob
    fi
    
    # Service enablement hint
    if [[ $not_enabled_count -gt 0 ]]; then
        echo ""
        log_section "⚠️  服务Not enabled提示"
        echo "部分服务Not enabled，请在火山引擎控制台检查："
        echo "  https://console.volcengine.com/speech"
    echo ""
        echo "确保以下服务已开通并绑定到 App ID:"
        echo "  - TTS 1.0: volcano_tts (volc.tts.default)"
        echo "  - TTS 2.0 大模型: volcano_mega (volc.seedtts.default)"
        echo "  - ASR: volcengine_streaming_common"
        echo "  - Voice Clone: volcano_icl"
    fi
}

# ==================== Main ====================

print_help() {
    echo "Doubao CLI Test Script"
    echo ""
    echo "Usage: $0 [test_name]"
    echo ""
    echo "Available tests:"
    echo "  tts           TTS synthesize (sync)"
    echo "  tts-stream    TTS stream (HTTP)"
    echo "  tts-async     TTS async task"
    echo "  asr           ASR one-sentence"
    echo "  asr-stream    ASR streaming"
    echo "  realtime      Realtime conversation (interactive)"
    echo "  podcast       Podcast synthesis"
    echo "  meeting       Meeting transcription"
    echo "  translation   Simultaneous translation (interactive)"
    echo "  subtitle      Subtitle extraction"
    echo "  voice-train   Voice clone training"
    echo "  all           Run all non-interactive tests"
    echo "  help          Show this help"
    echo ""
    echo "Examples:"
    echo "  $0 tts                    # Test TTS synthesize"
    echo "  $0 all                    # Run all tests"
    echo "  bazel run //examples/cmd/doubaospeech:run -- tts"
}

main() {
    local test_name="${1:-help}"
    
    case "$test_name" in
        tts)
            setup
            test_tts_synthesize
            print_summary
            ;;
        tts-stream)
            setup
            test_tts_stream
            print_summary
            ;;
        tts-async)
            setup
            test_tts_async
            print_summary
            ;;
        asr)
            setup
            test_asr_one_sentence
            print_summary
            ;;
        asr-stream)
            setup
            test_asr_stream
            print_summary
            ;;
        realtime)
            setup
            test_realtime
            ;;
        podcast)
            setup
            test_podcast
            print_summary
            ;;
        meeting)
            setup
            test_meeting
            print_summary
            ;;
        translation)
            setup
            test_translation
            ;;
        subtitle)
            setup
            test_subtitle
            print_summary
            ;;
        voice-train)
            setup
            test_voice_train
            print_summary
            ;;
        all)
            setup
            echo ""
            echo "╔══════════════════════════════════════════════════════════╗"
            echo "║           Running All Non-Interactive Tests              ║"
            echo "╚══════════════════════════════════════════════════════════╝"
            echo ""
            test_tts_synthesize || true
            test_tts_stream || true
            test_tts_async || true
            test_asr_one_sentence || true
            test_podcast || true
            test_meeting || true
            test_subtitle || true
            print_summary
            ;;
        help|--help|-h)
            print_help
            ;;
        *)
            log_error "Unknown test: $test_name"
            print_help
            exit 1
            ;;
    esac
}

main "$@"
