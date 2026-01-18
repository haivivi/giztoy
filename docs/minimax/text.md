# 文本生成 API

> **官方文档**:
> - Anthropic 兼容: https://platform.minimaxi.com/docs/api-reference/text-anthropic-api
> - OpenAI 兼容: https://platform.minimaxi.com/docs/api-reference/text-openai-api

## 概述

MiniMax 文本生成 API 支持对话内容生成和工具调用，兼容 Anthropic SDK 和 OpenAI SDK。

## 支持的模型

| 模型名称 | 输入输出总 token | 说明 |
|---------|-----------------|------|
| MiniMax-M2.1 | 204,800 | 强大多语言编程能力，输出速度约 60 tps |
| MiniMax-M2.1-lightning | 204,800 | M2.1 极速版，输出速度约 100 tps |
| MiniMax-M2 | 204,800 | 专为高效编码与 Agent 工作流而生 |

## 调用方式

### 方式一：Anthropic SDK（推荐）

#### 安装

```bash
# Python
pip install anthropic

# Node.js
npm install @anthropic-ai/sdk
```

#### 配置环境变量

```bash
export ANTHROPIC_API_KEY="your_api_key"
export ANTHROPIC_BASE_URL="https://api.minimaxi.com/v1"
```

#### Python 示例

```python
import anthropic

client = anthropic.Anthropic()

message = client.messages.create(
    model="MiniMax-M2.1",
    max_tokens=1024,
    messages=[
        {"role": "user", "content": "Hello, how are you?"}
    ]
)
print(message.content)
```

#### Node.js 示例

```javascript
import Anthropic from '@anthropic-ai/sdk';

const client = new Anthropic();

const message = await client.messages.create({
    model: "MiniMax-M2.1",
    max_tokens: 1024,
    messages: [
        { role: "user", content: "Hello, how are you?" }
    ]
});
console.log(message.content);
```

### 方式二：OpenAI SDK

#### 安装

```bash
# Python
pip install openai

# Node.js
npm install openai
```

#### 配置环境变量

```bash
export OPENAI_API_KEY="your_api_key"
export OPENAI_BASE_URL="https://api.minimaxi.com/v1"
```

#### Python 示例

```python
from openai import OpenAI

client = OpenAI()

response = client.chat.completions.create(
    model="MiniMax-M2.1",
    messages=[
        {"role": "user", "content": "Hello, how are you?"}
    ]
)
print(response.choices[0].message.content)
```

## 支持的参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| model | string | 是 | 模型名称 |
| messages | array | 是 | 消息列表 |
| max_tokens | integer | 否 | 最大输出 token 数 |
| temperature | float | 否 | 采样温度，范围 0-2 |
| top_p | float | 否 | 核采样参数 |
| stream | boolean | 否 | 是否流式输出 |
| tools | array | 否 | 工具定义列表 |
| tool_choice | string/object | 否 | 工具选择策略 |

## Message 字段支持

### 用户消息 (role: user)

```json
{
  "role": "user",
  "content": "文本内容或内容数组"
}
```

支持的内容类型：
- `text`: 文本内容
- `image`: 图片内容（base64 或 URL）

### 助手消息 (role: assistant)

```json
{
  "role": "assistant",
  "content": "助手回复内容"
}
```

### 系统消息 (role: system)

```json
{
  "role": "system",
  "content": "系统提示词"
}
```

## 流式响应

```python
with client.messages.stream(
    model="MiniMax-M2.1",
    max_tokens=1024,
    messages=[{"role": "user", "content": "Tell me a story"}]
) as stream:
    for text in stream.text_stream:
        print(text, end="", flush=True)
```

## 注意事项

1. 使用 Anthropic SDK 时，需要将 `ANTHROPIC_BASE_URL` 设置为 MiniMax 的 API 地址
2. 使用 OpenAI SDK 时，需要将 `OPENAI_BASE_URL` 设置为 MiniMax 的 API 地址
3. 模型名称使用 MiniMax 的模型名称，而非 Anthropic/OpenAI 的模型名称
