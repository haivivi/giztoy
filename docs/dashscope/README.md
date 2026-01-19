# DashScope (阿里云百炼) API 文档

## 原始文档

| 文档 | 链接 |
|------|------|
| 百炼平台首页 | https://help.aliyun.com/zh/model-studio/ |
| API 参考 | https://help.aliyun.com/zh/model-studio/qwen-api-reference |
| 开通服务 | https://help.aliyun.com/zh/dashscope/opening-service |
| 获取 API Key | https://help.aliyun.com/zh/model-studio/get-api-key |
| 模型列表 | https://help.aliyun.com/zh/model-studio/model-list |

> 如果本文档信息不完整，请访问上述链接获取最新内容。

---

## 概述

DashScope 是阿里云大模型服务平台百炼（Model Studio）提供的 API 服务。支持：

- **文本生成** - 通义千问（Qwen）系列大语言模型，兼容 OpenAI API
- **多模态** - 图像理解、音频理解
- **实时对话** - Qwen-Omni-Realtime 实时语音/图像对话
- **智能体应用** - 调用已配置的 Agent/工作流应用
- **知识库** - 文档上传、索引、检索增强生成（RAG）

---

## 目录结构

```
docs/dashscope/
├── README.md           # 本文档 - 概述
├── auth.md             # 认证与鉴权
├── text.md             # 文本模型 API (Qwen)
├── app.md              # 应用调用 API
└── realtime/           # 实时多模态 API
    ├── README.md       # 概述
    ├── client-events.md # 客户端事件
    └── server-events.md # 服务端事件
```

---

## 服务端点

### HTTP API

| 地域 | 端点 | 用途 |
|------|------|------|
| 北京（中国大陆） | `https://dashscope.aliyuncs.com/compatible-mode/v1` | OpenAI 兼容 |
| 新加坡（国际） | `https://dashscope-intl.aliyuncs.com/compatible-mode/v1` | OpenAI 兼容 |
| 弗吉尼亚（美国） | `https://dashscope-us.aliyuncs.com/compatible-mode/v1` | OpenAI 兼容 |

### WebSocket API

| 地域 | 端点 | 用途 |
|------|------|------|
| 北京 | `wss://dashscope.aliyuncs.com/api-ws/v1/realtime` | 实时对话 |
| 新加坡 | `wss://dashscope-intl.aliyuncs.com/api-ws/v1/realtime` | 实时对话 |

### 应用 API

```
POST https://dashscope.aliyuncs.com/api/v1/apps/{APP_ID}/completion
```

---

## 支持的模型

### 文本模型 (Qwen)

| 模型 | 上下文 | 特点 |
|------|--------|------|
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

### 实时多模态模型

| 模型 | 输出格式 | 默认音色 |
|------|---------|---------|
| Qwen3-Omni-Flash-Realtime | pcm24 | Cherry |
| Qwen-Omni-Turbo-Realtime | pcm16 | Chelsie |

---

## 快速开始

### 1. 获取 API Key

1. 登录 [百炼控制台](https://bailian.console.aliyun.com/)
2. 进入"密钥管理"
3. 创建 API Key

### 2. 设置环境变量

```bash
export DASHSCOPE_API_KEY="sk-xxxxxxxxxxxxxxxx"
```

### 3. 调用示例

```bash
curl https://dashscope.aliyuncs.com/compatible-mode/v1/chat/completions \
  -H "Authorization: Bearer $DASHSCOPE_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "qwen-turbo",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

---

## 详细文档

| 文档 | 说明 |
|------|------|
| [认证与鉴权](./auth.md) | API Key 管理、权限控制、工作空间 |
| [文本模型 API](./text.md) | Qwen 系列模型、OpenAI 兼容接口 |
| [应用调用 API](./app.md) | 智能体应用、工作流、知识库检索 |
| [实时多模态](./realtime/README.md) | Qwen-Omni-Realtime 实时语音对话 |

---

## SDK

### Python (OpenAI SDK)

```python
from openai import OpenAI

client = OpenAI(
    api_key=os.getenv("DASHSCOPE_API_KEY"),
    base_url="https://dashscope.aliyuncs.com/compatible-mode/v1"
)

response = client.chat.completions.create(
    model="qwen-turbo",
    messages=[{"role": "user", "content": "Hello!"}]
)
```

### Go (go-openai)

```go
import "github.com/sashabaranov/go-openai"

config := openai.DefaultConfig(os.Getenv("DASHSCOPE_API_KEY"))
config.BaseURL = "https://dashscope.aliyuncs.com/compatible-mode/v1"

client := openai.NewClientWithConfig(config)
```

### 官方 SDK

- Python: `pip install dashscope`
- Java: Maven 依赖 `com.alibaba:dashscope-sdk`
