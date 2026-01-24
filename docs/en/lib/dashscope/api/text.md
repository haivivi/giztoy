# 文本模型 API (Qwen)

## 原始文档

| 文档 | 链接 |
|------|------|
| Qwen API 参考 | https://help.aliyun.com/zh/model-studio/qwen-api-reference |
| OpenAI 兼容模式 | https://help.aliyun.com/zh/model-studio/compatibility-mode |
| 模型列表 | https://help.aliyun.com/zh/model-studio/model-list |

> 如果本文档信息不完整，请访问上述链接获取最新内容。

---

## 概述

百炼平台提供 OpenAI 兼容的文本生成 API，可直接使用 OpenAI SDK 调用通义千问（Qwen）系列模型。

## 支持的模型

### Qwen 系列

| 模型 | 上下文长度 | 特点 |
|------|-----------|------|
| qwen-turbo | 128K | 快速响应，性价比高 |
| qwen-plus | 128K | 平衡性能与成本 |
| qwen-max | 32K | 最强能力 |
| qwen-long | 1M | 超长上下文 |

### 多模态模型

| 模型 | 能力 |
|------|------|
| qwen-vl-plus | 视觉理解 |
| qwen-vl-max | 视觉理解（强化版） |
| qwen-audio-turbo | 音频理解 |

---

## API 端点

### OpenAI 兼容模式

```
POST https://dashscope.aliyuncs.com/compatible-mode/v1/chat/completions
```

### 原生 DashScope API

```
POST https://dashscope.aliyuncs.com/api/v1/services/aigc/text-generation/generation
```

---

## Chat Completions

### 请求示例

```bash
curl https://dashscope.aliyuncs.com/compatible-mode/v1/chat/completions \
  -H "Authorization: Bearer $DASHSCOPE_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "qwen-turbo",
    "messages": [
      {"role": "system", "content": "You are a helpful assistant."},
      {"role": "user", "content": "Hello!"}
    ]
  }'
```

### 请求参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| model | string | 是 | 模型名称 |
| messages | array | 是 | 消息列表 |
| max_tokens | int | 否 | 最大输出 token 数 |
| temperature | float | 否 | 采样温度 [0, 2] |
| top_p | float | 否 | 核采样参数 (0, 1] |
| stream | bool | 否 | 是否流式输出 |
| tools | array | 否 | 工具/函数定义 |
| tool_choice | string/object | 否 | 工具选择策略 |

### messages 格式

```json
{
  "messages": [
    {"role": "system", "content": "系统提示"},
    {"role": "user", "content": "用户消息"},
    {"role": "assistant", "content": "助手回复"},
    {"role": "user", "content": "后续问题"}
  ]
}
```

### 响应示例

```json
{
  "id": "chatcmpl-xxx",
  "object": "chat.completion",
  "created": 1234567890,
  "model": "qwen-turbo",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "Hello! How can I help you today?"
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 20,
    "completion_tokens": 10,
    "total_tokens": 30
  }
}
```

---

## 流式输出

### 请求

```json
{
  "model": "qwen-turbo",
  "messages": [...],
  "stream": true
}
```

### 响应（SSE）

```
data: {"id":"chatcmpl-xxx","choices":[{"delta":{"content":"Hello"}}]}

data: {"id":"chatcmpl-xxx","choices":[{"delta":{"content":"!"}}]}

data: {"id":"chatcmpl-xxx","choices":[{"delta":{},"finish_reason":"stop"}]}

data: [DONE]
```

---

## Function Calling

### 定义工具

```json
{
  "tools": [
    {
      "type": "function",
      "function": {
        "name": "get_weather",
        "description": "获取指定城市的天气",
        "parameters": {
          "type": "object",
          "properties": {
            "city": {
              "type": "string",
              "description": "城市名称"
            }
          },
          "required": ["city"]
        }
      }
    }
  ]
}
```

### 工具调用响应

```json
{
  "choices": [
    {
      "message": {
        "role": "assistant",
        "tool_calls": [
          {
            "id": "call_xxx",
            "type": "function",
            "function": {
              "name": "get_weather",
              "arguments": "{\"city\": \"北京\"}"
            }
          }
        ]
      },
      "finish_reason": "tool_calls"
    }
  ]
}
```

### 返回工具结果

```json
{
  "messages": [
    ...,
    {
      "role": "assistant",
      "tool_calls": [...]
    },
    {
      "role": "tool",
      "tool_call_id": "call_xxx",
      "content": "{\"temperature\": 25, \"weather\": \"sunny\"}"
    }
  ]
}
```

---

## 多模态输入

### 图像输入

```json
{
  "model": "qwen-vl-plus",
  "messages": [
    {
      "role": "user",
      "content": [
        {"type": "text", "text": "这张图片是什么？"},
        {"type": "image_url", "image_url": {"url": "https://example.com/image.jpg"}}
      ]
    }
  ]
}
```

### 音频输入

```json
{
  "model": "qwen-audio-turbo",
  "messages": [
    {
      "role": "user",
      "content": [
        {"type": "text", "text": "这段音频说了什么？"},
        {"type": "audio_url", "audio_url": {"url": "https://example.com/audio.mp3"}}
      ]
    }
  ]
}
```

---

## 使用 OpenAI SDK

### Python

```python
from openai import OpenAI

client = OpenAI(
    api_key=os.getenv("DASHSCOPE_API_KEY"),
    base_url="https://dashscope.aliyuncs.com/compatible-mode/v1"
)

response = client.chat.completions.create(
    model="qwen-turbo",
    messages=[
        {"role": "user", "content": "Hello!"}
    ]
)
```

### Go

```go
import "github.com/sashabaranov/go-openai"

config := openai.DefaultConfig(os.Getenv("DASHSCOPE_API_KEY"))
config.BaseURL = "https://dashscope.aliyuncs.com/compatible-mode/v1"

client := openai.NewClientWithConfig(config)
```

---

## 错误码

| HTTP 状态码 | 说明 |
|------------|------|
| 400 | 请求参数错误 |
| 401 | API Key 无效 |
| 403 | 无权限访问该模型 |
| 429 | 超出速率限制 |
| 500 | 服务器内部错误 |
