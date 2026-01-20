# Chat Completions API

接口兼容 OpenAI 请求格式。

## 接口地址

```
POST /llm/v1/chat/completions
```

## 请求参数

| 参数名称 | 类型 | 是否必填 | 默认值 | 参数说明 |
| --- | --- | --- | --- | --- |
| model | string | 是 | 无 | 模型名，可选值：`jiutian-lan`（九天基础语言大模型）、`jiutian-chat`（非生成式AI） |
| messages | array | 是 | 无 | 要生成句子的提示和上下文 |
| stream | boolean | 是 | true | `true`: 流式输出；`false`: 输出最终结果。注意：Python 访问传 `True` 或 `False` |
| temperature | number | 否 | 0.8 | 采样温度，控制输出的随机性，取值范围：(0.0, 1.0]，不能等于0。值越大输出更随机，值越小输出更稳定 |
| top_p | number | 否 | 0.95 | 核采样参数，取值范围：(0.0, 1.0]，不能等于0。例如 0.1 意味着只考虑前 10% 概率的 tokens |

> **注意**: 建议根据应用场景调整 `top_p` 或 `temperature` 参数，但不要同时调整两个参数。

## Messages 格式

```json
[
  {"role": "system", "content": "You are a helpful assistant."},
  {"role": "user", "content": "Who won the world series in 2020?"},
  {"role": "assistant", "content": "The Dodgers won the World Series in 2020."},
  {"role": "user", "content": "Where was it played?"}
]
```

## 非流式请求

设置 `stream` 为 `false`，`message.content` 为返回的完整内容。

### 请求示例

```bash
curl -X POST "https://ivs.chinamobiledevice.com:30100/llm/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -H "X-AI-VID: $AI_VID" \
  -H "X-AI-UID: $AI_UID" \
  -H "Authorization: Bearer $API_KEY" \
  -d '{
    "model": "jiutian-lan",
    "messages": [
      {
        "role": "system",
        "content": "您好，我是中国移动的智能助理灵犀。如果您询问我的身份，我会回答：您好，我是中国移动智能助理灵犀"
      },
      {
        "role": "user",
        "content": "test"
      }
    ],
    "stream": false
  }'
```

### 响应示例

```json
{
  "created": 1725501491,
  "usage": {
    "completion_tokens": 22,
    "prompt_tokens": 52,
    "total_tokens": 74
  },
  "model": "jiutian-lan",
  "id": "50040940-1b47-4455-af8b-004f35bc0da5",
  "choices": [
    {
      "finish_reason": "stop",
      "index": 0,
      "type": "text",
      "message": {
        "role": "assistant",
        "content": "The 2020 World Series was played at Globe Life Field in Arlington, Texas."
      },
      "status": "finish",
      "logprobs": null
    }
  ]
}
```

## 流式请求

设置 `stream` 为 `true`，返回 Server-Sent Events (SSE) 格式的流式响应。

### 请求示例

```bash
curl -X POST "https://ivs.chinamobiledevice.com:30100/llm/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -H "X-AI-VID: $AI_VID" \
  -H "X-AI-UID: $AI_UID" \
  -H "Authorization: Bearer $API_KEY" \
  -d '{
    "model": "jiutian-lan",
    "messages": [
      {
        "role": "system",
        "content": "您好，我是中国移动的智能助理灵犀"
      },
      {
        "role": "user",
        "content": "你好"
      }
    ],
    "stream": true
  }'
```

### 响应示例

```
data: {"id":"endpoint_common_5751","object":"chat.completion.chunk","created":1745985810,"model":"jiutian_75b","choices":[{"index":0,"delta":{"role":"assistant","content":"为您提供"},"finish_reason":null}]}

data: {"id":"endpoint_common_5751","object":"chat.completion.chunk","created":1745985810,"model":"jiutian_75b","choices":[{"index":0,"delta":{"role":"assistant","content":"帮助"},"finish_reason":null}]}

data: {"id":"endpoint_common_5751","object":"chat.completion.chunk","created":1745985810,"model":"jiutian_75b","usage":{"prompt_tokens":23,"completion_tokens":27,"total_tokens":50},"choices":[{"index":0,"delta":{"role":"assistant","content":""},"finish_reason":"stop"}]}

data: [DONE]
```

## 非生成式 AI

儿童手表等产品，需要接入非生成式 AI 时，申请 token 时在邮件里面注明需要非生成式 AI。

非生成式 AI 的 model 参数使用 `jiutian-chat`：

```bash
curl -X POST "https://ivs.chinamobiledevice.com:30100/llm/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -H "X-AI-VID: $AI_VID" \
  -H "X-AI-UID: $AI_UID" \
  -H "Authorization: Bearer $API_KEY" \
  -d '{
    "model": "jiutian-chat",
    "messages": [
      {
        "role": "system",
        "content": "今天天气怎么样"
      },
      {
        "role": "user",
        "content": "你是谁"
      }
    ],
    "stream": true
  }'
```
