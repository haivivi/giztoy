# 客户端事件

## 原始文档

- https://help.aliyun.com/zh/model-studio/client-events

> 如果本文档信息不完整，请访问上述链接获取最新内容。

---

## session.update

客户端建立 WebSocket 连接后，需首先发送该事件，用于更新会话的默认配置。

### 请求

```json
{
  "event_id": "event_ToPZqeobitzUJnt3QqtWg",
  "type": "session.update",
  "session": {
    "modalities": ["text", "audio"],
    "voice": "Chelsie",
    "input_audio_format": "pcm16",
    "output_audio_format": "pcm24",
    "instructions": "你是某五星级酒店的AI客服专员...",
    "turn_detection": {
      "type": "server_vad",
      "threshold": 0.5,
      "silence_duration_ms": 800
    },
    "seed": 1314,
    "max_tokens": 16384,
    "repetition_penalty": 1.05,
    "presence_penalty": 0.0,
    "top_k": 50,
    "top_p": 1.0,
    "temperature": 0.9
  }
}
```

### 参数说明

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| type | string | 是 | 固定为 `session.update` |
| event_id | string | 否 | 事件 ID |
| session | object | 否 | 会话配置 |

### session 对象

| 参数 | 类型 | 说明 |
|------|------|------|
| modalities | array | 输出模态：`["text"]` 或 `["text", "audio"]`（默认） |
| voice | string | 音色，详见音色列表。默认：Qwen3-Omni-Flash 为 Cherry，Qwen-Omni-Turbo 为 Chelsie |
| input_audio_format | string | 输入音频格式，仅支持 `pcm16` |
| output_audio_format | string | 输出音频格式：Flash 为 `pcm24`，Turbo 为 `pcm16` |
| smooth_output | boolean/null | 是否口语化回复（仅 Flash 模型，Turbo 模型会忽略此参数）。true=口语化，false=书面化，null=自动 |
| instructions | string | 系统消息，设定模型目标或角色 |
| turn_detection | object/null | VAD 配置，null 表示禁用 VAD |
| temperature | float | 采样温度 [0, 2)，Flash 默认 0.9，Turbo 默认 1.0（不可改） |
| top_p | float | 核采样参数 (0, 1.0]，Flash 默认 1.0，Turbo 默认 0.01（不可改） |
| top_k | int | Top-K 采样，默认 50 |
| max_tokens | int | 最大输出 token 数 |
| repetition_penalty | float | 重复惩罚 [-2.0, 2.0]，默认 0.0（Turbo 不可改） |
| presence_penalty | float | 存在惩罚 [-2.0, 2.0]，默认 0.0（Turbo 不可改） |
| seed | int | 随机种子 [0, 2^31-1]，默认 -1（Turbo 不可改） |

### turn_detection 对象

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| type | string | server_vad | 服务端 VAD 类型 |
| threshold | float | 0.5 | VAD 灵敏度 [-1.0, 1.0]，越低越敏感 |
| silence_duration_ms | int | 800 | 静音检测时长 [200, 6000] 毫秒 |

---

## response.create

指示服务端创建模型响应。在 VAD 模式下，服务端会自动创建响应，无需发送该事件。

### 请求

```json
{
  "type": "response.create",
  "event_id": "event_1718624400000"
}
```

### 参数说明

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| type | string | 是 | 固定为 `response.create` |
| event_id | string | 否 | 事件 ID |

---

## response.cancel

取消正在进行的响应。如果没有响应可取消，服务端会返回错误事件。

### 请求

```json
{
  "event_id": "event_B4o9RHSTWobB5OQdEHLTo",
  "type": "response.cancel"
}
```

---

## input_audio_buffer.append

将音频字节追加到输入音频缓冲区。

### 请求

```json
{
  "event_id": "event_B4o9RHSTWobB5OQdEHLTo",
  "type": "input_audio_buffer.append",
  "audio": "UklGR..."
}
```

### 参数说明

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| type | string | 是 | 固定为 `input_audio_buffer.append` |
| audio | string | 是 | Base64 编码的音频数据 |

---

## input_audio_buffer.commit

提交用户输入音频缓冲区，在对话中创建新的用户消息项。

- **VAD 模式**：客户端不需要发送此事件，服务端会自动提交
- **Manual 模式**：客户端必须提交才能创建用户消息项

> 如果客户端发送过 `input_image_buffer.append` 事件，此事件会将图像缓冲区一起提交。

### 请求

```json
{
  "event_id": "event_B4o9RHSTWobB5OQdEHLTo",
  "type": "input_audio_buffer.commit"
}
```

**注意**：提交输入音频缓冲区不会自动创建模型响应，需要单独发送 `response.create`（仅 Manual 模式）。

---

## input_audio_buffer.clear

清除缓冲区中的音频字节。

### 请求

```json
{
  "event_id": "event_xxx",
  "type": "input_audio_buffer.clear"
}
```

---

## input_image_buffer.append

将图像数据添加到图像缓冲区。图像可来自本地文件或视频流实时采集。

### 请求

```json
{
  "event_id": "event_xxx",
  "type": "input_image_buffer.append",
  "image": "xxx"
}
```

### 参数说明

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| type | string | 是 | 固定为 `input_image_buffer.append` |
| image | string | 是 | Base64 编码的图像数据 |

### 图像限制

| 限制项 | 要求 |
|--------|------|
| 格式 | JPG 或 JPEG |
| 分辨率 | 建议 480p-720p，最高 1080p |
| 大小 | < 500KB（Base64 编码前） |
| 发送频率 | 建议 1 张/秒 |
| 前置条件 | 发送前至少发送过一次 `input_audio_buffer.append` |

> 图像缓冲区与音频缓冲区一起通过 `input_audio_buffer.commit` 事件提交。
