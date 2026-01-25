# Doubao Speech CLI

豆包语音 API 命令行工具，支持多种语音服务的测试和调用。

## 特性

- 🔐 **Context 管理**：类似 kubectl 的 context 切换，支持多账户/环境
- 📝 **YAML/JSON 请求**：通过 `-f` 参数输入请求文件
- 📤 **JSON 输出**：支持 `--json` 输出，便于 pipe 多个命令
- 🖥️ **TUI 支持**：交互式界面探索 API
- 📁 **配置持久化**：配置存储在 `~/.giztoy/doubaospeech/`

## 安装

```bash
# 使用 Bazel 构建
bazel build //go/cmd/doubaospeech

# 或者使用 go install
cd go/cmd/doubaospeech
go install
```

## 快速开始

### 1. 添加 Context

```bash
# 添加一个新的 context
doubaospeech config add-context myctx --token YOUR_TOKEN --app-id YOUR_APP_ID

# 设置为默认 context
doubaospeech config use-context myctx

# 查看所有 context
doubaospeech config list-contexts
```

### 2. 测试 API

```bash
# TTS V2 HTTP 流式（推荐）
doubaospeech -c myctx tts v2 stream -f tts.yaml -o output.mp3

# TTS V2 WebSocket 双向
doubaospeech -c myctx tts v2 bidirectional -f tts.yaml -o output.mp3

# TTS V1（旧版）
doubaospeech -c myctx tts v1 synthesize -f tts.yaml -o output.mp3

# ASR V2 流式
doubaospeech -c myctx asr v2 stream -f asr.yaml --audio input.pcm

# SAMI Podcast
doubaospeech -c myctx podcast sami -f podcast.yaml -o output.mp3

# 实时对话
doubaospeech -c myctx realtime interactive -f realtime.yaml
```

### 3. Pipe 多个命令

```bash
# 获取 JSON 输出并用 jq 处理
doubaospeech -c myctx asr v2 stream -f asr.yaml --json | jq '.text'
```

---

## 命令结构（方案 A：按版本分组）

```
doubaospeech [flags] <service> [version] <command> [args]

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
  podcast      播客合成 (Podcast Synthesis)
  realtime     实时对话 (Real-time Conversation)
  voice        声音复刻 (Voice Cloning)
  meeting      会议转写 (Meeting Transcription)
  translation  同声传译 (Simultaneous Translation)
  media        媒体处理 (Subtitle Extraction)
  console      控制台管理 (API Keys, Quotas, etc.)
  interactive  交互式 TUI
```

---

## 服务命令

### tts - 语音合成

#### V1 API（经典版）

```bash
doubaospeech tts v1 synthesize -f request.yaml -o output.mp3  # 同步合成
doubaospeech tts v1 stream -f request.yaml -o output.mp3      # 流式合成
```

**V1 请求文件 (tts-v1.yaml)**:
```yaml
text: "你好，这是一段测试语音。"
voice_type: zh_female_cancan      # V1 音色（无后缀）
encoding: mp3
sample_rate: 24000
cluster: volcano_tts              # V1 需要指定 cluster
```

#### V2 API（大模型版，推荐）

```bash
doubaospeech tts v2 stream -f request.yaml -o output.mp3       # HTTP 流式
doubaospeech tts v2 ws -f request.yaml -o output.mp3           # WebSocket 单向
doubaospeech tts v2 bidirectional -f request.yaml -o output.mp3 # WebSocket 双向
doubaospeech tts v2 async -f request.yaml                      # 异步长文本
doubaospeech tts v2 status <task_id>                           # 查询异步任务
```

**V2 请求文件 (tts-v2.yaml)**:
```yaml
text: "你好，这是一段测试语音。"
speaker: zh_female_xiaohe_uranus_bigtts  # V2 音色（必须匹配 resource_id）
resource_id: seed-tts-2.0                 # 见下方音色规则
format: mp3
sample_rate: 24000
```

#### ⚠️ 音色与 Resource ID 对应规则

| Resource ID | 音色后缀要求 | 示例音色 |
|-------------|-------------|---------|
| `seed-tts-2.0` | `*_uranus_bigtts` | `zh_female_xiaohe_uranus_bigtts` ✅ |
| `seed-tts-1.0` | `*_moon_bigtts` | `zh_female_shuangkuaisisi_moon_bigtts` ✅ |
| `seed-icl-2.0` | `*_saturn_bigtts` | 复刻音色 |

**常见错误**:
```
resource ID is mismatched with speaker related resource
```
**含义**：音色后缀与 Resource ID 不匹配，不是"服务未开通"！

---

### asr - 语音识别

#### V1 API（经典版）

```bash
doubaospeech asr v1 recognize -f request.yaml                  # 一句话识别
doubaospeech asr v1 stream -f config.yaml --audio input.pcm    # 流式识别
```

#### V2 API（大模型版）

```bash
doubaospeech asr v2 stream -f config.yaml --audio input.pcm    # 流式识别
doubaospeech asr v2 file -f request.yaml                       # 文件识别
doubaospeech asr v2 status <task_id>                           # 查询任务
```

**V2 请求文件 (asr-v2.yaml)**:
```yaml
resource_id: volc.bigasr.sauc.duration
format: pcm
sample_rate: 16000
```

---

### podcast - 播客合成

```bash
doubaospeech podcast http -f request.yaml     # HTTP 提交（轮询结果）
doubaospeech podcast sami -f request.yaml -o output.mp3  # SAMI WebSocket 流式
doubaospeech podcast status <task_id>         # 查询 HTTP 任务状态
```

**SAMI Podcast 请求文件 (podcast-sami.yaml)**:
```yaml
action: 0  # 0=概要生成
input_text: "分析当前大语言模型的发展..."
audio_config:
  format: mp3
  sample_rate: 24000
speaker_info:
  speakers:
    - zh_male_dayixiansheng_v2_saturn_bigtts   # SAMI 专用音色
    - zh_female_mizaitongxue_v2_saturn_bigtts
```

---

### realtime - 实时对话

```bash
doubaospeech realtime interactive -f config.yaml  # 交互式对话
doubaospeech realtime connect -f config.yaml      # 连接实时服务
```

---

### voice - 声音复刻

```bash
doubaospeech voice list                 # 列出已训练音色
doubaospeech voice clone -f request.yaml  # 声音复刻
doubaospeech voice status <speaker_id>  # 查询训练状态
doubaospeech voice delete <speaker_id>  # 删除音色
```

---

### meeting - 会议转写

```bash
doubaospeech meeting create -f request.yaml  # 创建会议转写任务
doubaospeech meeting status <task_id>        # 查询任务状态
```

---

### translation - 同声传译

```bash
doubaospeech translation stream -f config.yaml --audio input.pcm -o output.pcm
doubaospeech translation interactive -f config.yaml
```

---

### media - 媒体处理

```bash
doubaospeech media subtitle -f request.yaml  # 提取字幕
doubaospeech media status <task_id>          # 查询任务状态
```

---

### console - 控制台管理

> ⚠️ Console API 需要火山引擎 AK/SK 认证，与语音 API 的 Token 认证不同。

```bash
# 音色管理
doubaospeech console timbre list [--page <n>] [--size <n>]
doubaospeech console timbre speaker --language <lang>

# API Key 管理
doubaospeech console apikey list
doubaospeech console apikey create --name <name>

# 服务管理
doubaospeech console service status

# 监控
doubaospeech console quota [--service <service_id>]
doubaospeech console usage --start <date> --end <date>
```

---

### config - 配置管理

```bash
doubaospeech config add-context <name> --token <token> --app-id <appid>
doubaospeech config delete-context <name>
doubaospeech config use-context <name>
doubaospeech config get-context
doubaospeech config list-contexts
doubaospeech config view
```

---

### interactive - 交互式模式

```bash
doubaospeech interactive
doubaospeech i
doubaospeech tui
```

---

## 配置文件

配置存储在 `~/.giztoy/doubaospeech/config.yaml`：

```yaml
current_context: myctx
contexts:
  myctx:
    name: myctx
    client:
      app_id: "your-app-id"
      api_key: "your-bearer-token"
    extra:
      default_voice: zh_female_xiaohe_uranus_bigtts
      default_resource_id: seed-tts-2.0
```

---

## 认证方式对照

| API 版本 | 认证 Header |
|---------|------------|
| V1 | `Authorization: Bearer {token}` |
| V2/V3 | `X-Api-App-Id` + `X-Api-Access-Key` + `X-Api-Resource-Id` |
| Console | AK/SK 签名 |

---

## 相关文档

- SDK 文档：`docs/zh/lib/doubaospeech/doc.md`
- API 文档：`docs/zh/lib/doubaospeech/api/`
- AI 开发指南：`go/pkg/doubaospeech/AGENTS.md`
- 示例代码：`examples/go/doubaospeech/`

---

## License

MIT
