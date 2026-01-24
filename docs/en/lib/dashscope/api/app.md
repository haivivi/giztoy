# 应用调用 API

## 原始文档

| 文档 | 链接 |
|------|------|
| 通过 API 调用应用 | https://help.aliyun.com/zh/model-studio/user-guide/application-calling |
| 应用 API 参考 | https://help.aliyun.com/zh/model-studio/dashscope-api-reference |
| 智能体编排应用 | https://help.aliyun.com/zh/model-studio/invoke-agent-orchestration-application |

> 如果本文档信息不完整，请访问上述链接获取最新内容。

---

## 概述

百炼平台支持创建和调用智能体应用（Agent）、工作流应用（Workflow）等。通过 API 可以：

- 调用已发布的应用
- 支持多轮对话
- 支持流式输出
- 支持知识库检索（RAG）
- 支持文件上传

---

## 前置准备

1. 在百炼控制台创建应用
2. 配置应用（模型、知识库、插件等）
3. 发布应用
4. 获取应用 ID（App ID）

---

## API 端点

```
POST https://dashscope.aliyuncs.com/api/v1/apps/{APP_ID}/completion
```

---

## 基本调用

### 请求示例

```bash
curl https://dashscope.aliyuncs.com/api/v1/apps/{APP_ID}/completion \
  -H "Authorization: Bearer $DASHSCOPE_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "input": {
      "prompt": "你好，请介绍一下你自己"
    },
    "parameters": {}
  }'
```

### 请求参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| input | object | 是 | 输入内容 |
| input.prompt | string | 是 | 用户输入 |
| input.session_id | string | 否 | 会话 ID（多轮对话） |
| input.memory_id | string | 否 | 长期记忆 ID |
| parameters | object | 否 | 调用参数 |

### 响应示例

```json
{
  "output": {
    "text": "你好！我是一个智能助手...",
    "finish_reason": "stop",
    "session_id": "session_xxx"
  },
  "usage": {
    "models": [
      {
        "model_id": "qwen-turbo",
        "input_tokens": 20,
        "output_tokens": 50
      }
    ]
  },
  "request_id": "xxx"
}
```

---

## 多轮对话

### 方式一：使用 session_id

首次调用后，服务端返回 `session_id`，后续调用传入该 ID：

```json
{
  "input": {
    "prompt": "继续刚才的话题",
    "session_id": "session_xxx"
  }
}
```

### 方式二：传入 messages

直接传入历史消息：

```json
{
  "input": {
    "messages": [
      {"role": "user", "content": "你好"},
      {"role": "assistant", "content": "你好！有什么可以帮你的？"},
      {"role": "user", "content": "今天天气怎么样？"}
    ]
  }
}
```

---

## 流式输出

### 请求

```json
{
  "input": {
    "prompt": "写一首诗"
  },
  "parameters": {
    "incremental_output": true
  }
}
```

### 响应（SSE）

```
data: {"output":{"text":"春"},"usage":{}}

data: {"output":{"text":"风"},"usage":{}}

data: {"output":{"text":"又"},"usage":{}}

...
```

---

## 知识库检索 (RAG)

### 基本 RAG 调用

应用配置了知识库后，自动进行检索：

```json
{
  "input": {
    "prompt": "公司的退款政策是什么？"
  }
}
```

### 指定知识库

```json
{
  "input": {
    "prompt": "查询产品手册"
  },
  "parameters": {
    "rag_options": {
      "pipeline_ids": ["pipeline_xxx"],
      "file_ids": ["file_xxx"]
    }
  }
}
```

### rag_options 参数

| 参数 | 类型 | 说明 |
|------|------|------|
| pipeline_ids | array | 知识库 ID 列表（最多 5 个） |
| file_ids | array | 文件 ID 列表 |
| tags | array | 标签筛选 |
| metadata_filter | object | 元数据筛选 |

---

## 文件上传

### 1. 获取上传凭证

```bash
POST https://dashscope.aliyuncs.com/api/v1/apps/{APP_ID}/files/upload_lease
```

```json
{
  "file_name": "document.pdf",
  "file_size": 1024000,
  "content_type": "application/pdf"
}
```

### 2. 上传文件

使用返回的 `upload_url` 和 `headers` 上传文件。

### 3. 调用时引用文件

```json
{
  "input": {
    "prompt": "总结这个文档的内容",
    "session_file_ids": ["file_session_xxx"]
  }
}
```

### 文件限制

| 限制项 | 要求 |
|--------|------|
| 大小 | 根据文件类型不同 |
| 数量 | 单次最多 10 个 |
| 状态 | 必须为 `FILE_IS_READY` |

---

## 思考过程 (Thoughts)

显示模型的思考过程（插件调用、知识检索等）：

### 启用

1. 在控制台开启"显示思考过程"
2. 发布应用
3. 调用时设置参数

```json
{
  "input": {
    "prompt": "查询最新的销售数据"
  },
  "parameters": {
    "has_thoughts": true
  }
}
```

### 响应

```json
{
  "output": {
    "text": "根据最新数据...",
    "thoughts": [
      {
        "thought": "用户需要查询销售数据",
        "action_type": "knowledge_retrieval",
        "action_input": "销售数据"
      },
      {
        "thought": "找到相关文档",
        "observation": "2024年Q1销售额..."
      }
    ]
  }
}
```

---

## 长期记忆

对于开启了长期记忆功能的应用，可以传入 `memory_id`：

```json
{
  "input": {
    "prompt": "还记得我之前说的吗？",
    "memory_id": "memory_xxx"
  }
}
```

---

## 错误处理

### 常见错误

| 错误码 | 说明 | 解决方案 |
|--------|------|---------|
| InvalidAppId | 应用 ID 无效 | 检查 App ID 是否正确 |
| AppNotPublished | 应用未发布 | 在控制台发布应用 |
| AccessDenied | 无权限访问 | 检查 API Key 和工作空间 |
| FileNotReady | 文件未就绪 | 等待文件处理完成 |
