# MiniMax CLI

MiniMax API 命令行工具，支持多种 AI 服务的测试和调用。

## 特性

- 🔐 **Context 管理**：类似 kubectl 的 context 切换，支持多账户/环境
- 📝 **YAML/JSON 请求**：通过 `-f` 参数输入请求文件
- 📤 **JSON 输出**：支持 `--json` 输出，便于 pipe 多个命令
- 🖥️ **TUI 支持**：交互式界面探索 API
- 📁 **配置持久化**：配置存储在 `~/.giztoy/minimax/`

## 安装

```bash
# 使用 Bazel 构建
bazel build //go/cmd/minimax

# 或者使用 go install
cd go/cmd/minimax
go install
```

## 快速开始

### 1. 添加 Context

```bash
# 添加国内 context（默认使用 https://api.minimaxi.com）
minimax config add-context cn --api-key YOUR_API_KEY

# 添加海外 context（使用 https://api.minimaxi.chat）
minimax config add-context global --api-key YOUR_API_KEY --base-url https://api.minimaxi.chat

# 设置为默认 context
minimax config use-context cn

# 查看所有 context
minimax config list-contexts
```

**API 端点:**
| 区域 | Base URL |
|------|----------|
| 国内 | `https://api.minimaxi.com` (默认) |
| 海外 | `https://api.minimaxi.chat` |

### 2. 测试 API

示例请求文件位于 `examples/minimax/commands/` 目录下：

```bash
# 文本生成
minimax -c myctx text chat -f examples/minimax/commands/chat.yaml

# 语音合成
minimax -c myctx speech synthesize -f examples/minimax/commands/speech.yaml -o output.mp3

# 视频生成
minimax -c myctx video t2v -f examples/minimax/commands/video-t2v.yaml

# 图片生成
minimax -c myctx image generate -f examples/minimax/commands/image.yaml

# 音乐生成
minimax -c myctx music generate -f examples/minimax/commands/music.yaml -o song.mp3
```

### 3. Pipe 多个命令

```bash
# 获取 JSON 输出并用 jq 处理
minimax -c myctx text chat -f chat.yaml --json | jq '.choices[0].message.content'

# 链式调用（示例）
minimax -c myctx voice list --json | jq '.voices[0].voice_id'
```

## 命令结构

```
minimax [flags] <service> <command> [args]

Global Flags:
  -c, --context string   指定使用的 context
  -f, --file string      输入请求文件 (YAML/JSON)
  -o, --output string    输出文件路径
      --json             输出 JSON 格式（用于 pipe）
  -v, --verbose          详细输出

Services:
  config    配置管理
  text      文本生成
  speech    语音合成
  video     视频生成
  image     图片生成
  music     音乐生成
  voice     音色管理
  file      文件管理
  interactive  交互式 TUI
```

## 服务命令

### config - 配置管理

```bash
minimax config add-context <name> --api-key <key> [--base-url <url>]
minimax config delete-context <name>
minimax config use-context <name>
minimax config get-context
minimax config list-contexts
minimax config view
```

### text - 文本生成

```bash
minimax text chat -f request.yaml
minimax text chat-stream -f request.yaml
```

### speech - 语音合成

```bash
minimax speech synthesize -f request.yaml -o output.mp3
minimax speech stream -f request.yaml -o output.mp3
minimax speech async -f request.yaml
```

### video - 视频生成

```bash
minimax video t2v -f request.yaml              # 文生视频（创建任务）
minimax video t2v -f request.yaml --wait       # 文生视频（等待完成）
minimax video t2v -f request.yaml --wait -o video.mp4  # 等待并下载
minimax video i2v -f request.yaml              # 图生视频
minimax video frame -f request.yaml            # 首尾帧生成视频
minimax video status <task_id>                 # 查询任务状态
minimax video wait <task_id> -o video.mp4      # 等待任务并下载
```

### image - 图片生成

```bash
minimax image generate -f request.yaml
minimax image reference -f request.yaml
```

### music - 音乐生成

```bash
minimax music generate -f request.yaml -o song.mp3
```

### voice - 音色管理

```bash
minimax voice list [--type all|system|voice_cloning]
minimax voice clone -f request.yaml
minimax voice design -f request.yaml
minimax voice delete <voice_id>
```

### file - 文件管理

```bash
minimax file list [--purpose <purpose>]
minimax file upload <file> --purpose <purpose>
minimax file download <file_id> -o <output>
minimax file get <file_id>
minimax file delete <file_id>
```

### interactive - 交互式模式

```bash
minimax interactive
minimax i
minimax tui
```

## 配置文件

配置存储在 `~/.giztoy/minimax/config.yaml`：

```yaml
current_context: myctx
contexts:
  myctx:
    name: myctx
    api_key: your-api-key-here
    base_url: https://api.minimaxi.com  # 可选
    timeout: 30                          # 可选，秒
    default_model: MiniMax-M2.1         # 可选
    default_voice: female-shaonv        # 可选
  
  prod:
    name: prod
    api_key: production-api-key
    base_url: https://api.minimaxi.com
```

## 请求文件示例

> **注意**：目前推荐使用 JSON 格式的请求文件，因为 YAML 解析依赖 interface 包的 yaml 标签支持（待完善）。

请参考 `examples/minimax/commands/` 目录下的示例文件（Go 和 Rust CLI 共用）：

- `chat.yaml` - 文本聊天
- `speech.yaml` - 语音合成
- `async-speech.yaml` - 异步长文本语音合成
- `video-t2v.yaml` - 文生视频
- `video-i2v.yaml` - 图生视频
- `image.yaml` - 图片生成
- `music.yaml` - 音乐生成
- `voice-clone.yaml` - 音色复刻
- `voice-design.yaml` - 音色设计

## 示例脚本

提供了完整的示例脚本，同时支持 Go 和 Rust CLI 测试：

```bash
cd examples/minimax

# 先配置 context
minimax config add-context minimax_cn --api-key YOUR_API_KEY

# 使用 Go CLI 运行测试
./run.sh go all       # 全部测试
./run.sh go quick     # 快速测试（基础 + 声音管理）
./run.sh go 1         # 只运行阶段 1（TTS, Chat）

# 使用 Rust CLI 运行测试
./run.sh rust all

# 同时测试 Go 和 Rust
./run.sh both quick
```

### 测试阶段

| 阶段 | 内容 | 使用的示例文件 |
|------|------|---------------|
| 1 | 基础测试 (TTS, Chat) | `speech.yaml`, `chat.yaml` |
| 2 | 图片生成测试 | `image.yaml` |
| 3 | 流式测试 | `speech.yaml`, `chat.yaml` |
| 4 | 视频任务测试 | `video-t2v.yaml` |
| 5 | 声音管理测试 | `voice-design.yaml` |
| 6 | 音色克隆测试 | `voice-clone.yaml` |
| 7 | 文件管理测试 | - |
| 8 | 音乐生成测试 | `music.yaml` |

### 环境变量

```bash
# 通过环境变量设置 API Key（可选）
MINIMAX_API_KEY=xxx ./run.sh go all

# 指定 context 名称（默认 minimax_cn）
MINIMAX_CONTEXT=myctx ./run.sh rust all
```

## License

MIT
