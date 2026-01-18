# Doubao Speech CLI

豆包语音 API 命令行工具，支持多种语音服务的测试和调用。

## 特性

- 🔐 **Context 管理**：类似 kubectl 的 context 切换，支持多账户/环境
- 📝 **YAML/JSON 请求**：通过 `-f` 参数输入请求文件
- 📤 **JSON 输出**：支持 `--json` 输出，便于 pipe 多个命令
- 🖥️ **TUI 支持**：交互式界面探索 API
- 📁 **配置持久化**：配置存储在 `~/.giztoy/doubao/`

## 安装

```bash
# 使用 Bazel 构建
bazel build //go/cmd/doubao

# 或者使用 go install
cd go/cmd/doubao
go install
```

## 快速开始

### 1. 添加 Context

```bash
# 添加一个新的 context
doubao config add-context myctx --token YOUR_TOKEN --app-id YOUR_APP_ID --cluster volcano_tts

# 设置为默认 context
doubao config use-context myctx

# 查看所有 context
doubao config list-contexts
```

### 2. 测试 API

```bash
# 语音合成
doubao -c myctx tts synthesize -f examples/tts.yaml -o output.mp3

# 流式语音合成
doubao -c myctx tts stream -f examples/tts.yaml -o output.mp3

# 语音识别
doubao -c myctx asr one-sentence -f examples/asr-one-sentence.yaml

# 流式语音识别
doubao -c myctx asr stream -f examples/asr-stream.yaml --audio input.pcm

# 声音复刻
doubao -c myctx voice train -f examples/voice-train.yaml

# 实时对话
doubao -c myctx realtime interactive -f examples/realtime.yaml

# 会议转写
doubao -c myctx meeting create -f examples/meeting.yaml

# 播客合成
doubao -c myctx podcast create -f examples/podcast.yaml

# 同声传译
doubao -c myctx translation stream -f examples/translation.yaml --audio input.pcm

# 字幕提取
doubao -c myctx media subtitle -f examples/subtitle.yaml
```

### 3. Pipe 多个命令

```bash
# 获取 JSON 输出并用 jq 处理
doubao -c myctx asr one-sentence -f asr.yaml --json | jq '.text'

# 查询任务状态
doubao -c myctx tts status task_12345 --json | jq '.status'
```

## 命令结构

```
doubao [flags] <service> <command> [args]

Global Flags:
  -c, --context string   指定使用的 context
  -f, --file string      输入请求文件 (YAML/JSON)
  -o, --output string    输出文件路径
      --json             输出 JSON 格式（用于 pipe）
  -v, --verbose          详细输出

Services:
  config       配置管理
  tts          语音合成 (Text-to-Speech)
  asr          语音识别 (Automatic Speech Recognition)
  voice        声音复刻 (Voice Cloning)
  realtime     实时对话 (Real-time Conversation)
  meeting      会议转写 (Meeting Transcription)
  podcast      播客合成 (Podcast Synthesis)
  translation  同声传译 (Simultaneous Translation)
  media        媒体处理 (Subtitle Extraction)
  console      控制台管理 (API Keys, Quotas, etc.)
  interactive  交互式 TUI
```

## 服务命令

### config - 配置管理

```bash
doubao config add-context <name> --token <token> --app-id <appid> [--cluster <cluster>]
doubao config delete-context <name>
doubao config use-context <name>
doubao config get-context
doubao config list-contexts
doubao config view
```

### tts - 语音合成

```bash
doubao tts synthesize -f request.yaml -o output.mp3  # 同步合成
doubao tts stream -f request.yaml -o output.mp3      # HTTP 流式
doubao tts stream-ws -f request.yaml -o output.mp3   # WebSocket 流式
doubao tts duplex -f request.yaml -o output.mp3      # 双工流式
doubao tts async -f request.yaml                     # 异步合成（长文本）
doubao tts status <task_id>                          # 查询任务状态
```

### asr - 语音识别

```bash
doubao asr one-sentence -f request.yaml              # 一句话识别 (< 60s)
doubao asr stream -f config.yaml --audio input.pcm   # 流式识别
doubao asr file -f request.yaml                      # 文件识别（异步）
doubao asr status <task_id>                          # 查询任务状态
```

### voice - 声音复刻

```bash
doubao voice train -f request.yaml     # 训练音色
doubao voice status <speaker_id>       # 查询训练状态
doubao voice list                      # 列出已训练音色
doubao voice delete <speaker_id>       # 删除音色
```

### realtime - 实时对话

```bash
doubao realtime connect -f config.yaml      # 连接实时服务
doubao realtime interactive -f config.yaml  # 交互式对话
```

### meeting - 会议转写

```bash
doubao meeting create -f request.yaml  # 创建会议转写任务
doubao meeting status <task_id>        # 查询任务状态
```

### podcast - 播客合成

```bash
doubao podcast create -f request.yaml  # 创建播客合成任务
doubao podcast status <task_id>        # 查询任务状态
```

### translation - 同声传译

```bash
doubao translation stream -f config.yaml --audio input.pcm -o output.pcm
doubao translation interactive -f config.yaml
```

### media - 媒体处理

```bash
doubao media subtitle -f request.yaml  # 提取字幕
doubao media status <task_id>          # 查询任务状态
```

### console - 控制台管理

```bash
# 音色管理
doubao console timbre list [--page <n>] [--size <n>]
doubao console timbre speaker --language <lang>

# API Key 管理
doubao console apikey list
doubao console apikey create --name <name>
doubao console apikey delete <apikey_id>
doubao console apikey update <apikey_id> [--name <name>] [--status active|inactive]

# 服务管理
doubao console service status
doubao console service activate <service_id>
doubao console service pause <service_id>
doubao console service resume <service_id>

# 监控
doubao console quota [--service <service_id>]
doubao console usage --start <date> --end <date> [--granularity hour|day|month]
doubao console qps
```

### interactive - 交互式模式

```bash
doubao interactive
doubao i
doubao tui
```

## 配置文件

配置存储在 `~/.giztoy/doubao/config.yaml`：

```yaml
current_context: myctx
contexts:
  myctx:
    name: myctx
    api_key: your-bearer-token-here
    extra:
      app_id: your-app-id
      cluster: volcano_tts
      default_voice: zh_female_cancan
      user_id: optional-user-id
  
  prod:
    name: prod
    api_key: production-token
    extra:
      app_id: prod-app-id
      cluster: volcano_tts
```

## 请求文件示例

请参考 `examples/` 目录下的示例文件：

- `tts.yaml` - 语音合成
- `tts-async.yaml` - 异步语音合成（长文本）
- `asr-one-sentence.yaml` - 一句话识别
- `asr-stream.yaml` - 流式语音识别
- `voice-train.yaml` - 声音复刻训练
- `realtime.yaml` - 实时对话配置
- `meeting.yaml` - 会议转写
- `podcast.yaml` - 播客合成
- `translation.yaml` - 同声传译
- `subtitle.yaml` - 字幕提取

## 开发状态

⚠️ **注意**：当前版本 CLI 框架已完成，但实际 API 调用尚未实现。运行命令会显示请求内容预览。

待实现：
- [ ] 实际 API 调用（使用 doubaospeech 包）
- [ ] 流式输出支持
- [ ] WebSocket 连接
- [ ] 异步任务轮询
- [ ] 更丰富的 TUI 界面

## License

MIT
