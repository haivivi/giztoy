# MiniMax 开放平台 API 文档

> **官方文档**: [MiniMax 开放平台文档中心](https://platform.minimaxi.com/docs/api-reference/api-overview)
>
> **最后更新**: 2026-01-19
>
> **注意**: 本文档基于官方文档整理，如有更新请参考官方文档

## 官方文档导航

如果本文档信息不完整或需要最新信息，请访问以下官方链接：

| 功能模块 | 官方文档链接 |
|---------|-------------|
| 接口概览 | https://platform.minimaxi.com/docs/api-reference/api-overview |
| 文本生成 (Anthropic) | https://platform.minimaxi.com/docs/api-reference/text-anthropic-api |
| 文本生成 (OpenAI) | https://platform.minimaxi.com/docs/api-reference/text-openai-api |
| 同步语音合成 HTTP | https://platform.minimaxi.com/docs/api-reference/speech-t2a-http |
| 同步语音合成 WebSocket | https://platform.minimaxi.com/docs/api-reference/speech-t2a-ws |
| 异步长文本语音合成 | https://platform.minimaxi.com/docs/api-reference/speech-t2a-async |
| 音色快速复刻 | https://platform.minimaxi.com/docs/api-reference/speech-voice-cloning |
| 音色设计 | https://platform.minimaxi.com/docs/api-reference/speech-voice-design |
| 声音管理 | https://platform.minimaxi.com/docs/api-reference/speech-voice-management |
| 视频生成 | https://platform.minimaxi.com/docs/api-reference/video-generation |
| 视频生成 Agent | https://platform.minimaxi.com/docs/api-reference/video-generation-agent |
| 图片生成 | https://platform.minimaxi.com/docs/api-reference/image-generation |
| 音乐生成 | https://platform.minimaxi.com/docs/api-reference/music-generation |
| 文件管理 | https://platform.minimaxi.com/docs/api-reference/file-management |
| 错误码查询 | https://platform.minimaxi.com/docs/api-reference/error-code |

## 如何获取最新文档

### 方法一：直接访问官网

访问 [MiniMax 开放平台文档中心](https://platform.minimaxi.com/docs/api-reference/api-overview)，左侧导航栏包含所有 API 接口的详细文档。

### 方法二：使用 AI 工具读取

如果使用支持浏览器功能的 AI 工具（如 Cursor），可以：

1. 使用 `browser_navigate` 工具访问官方文档页面
2. 使用 `browser_snapshot` 获取页面内容
3. 解析页面中的 API 参数、请求/响应格式等信息

示例：
```
访问: https://platform.minimaxi.com/docs/api-reference/speech-t2a-http
```

### 方法三：查看官方 MCP 服务器

MiniMax 提供了官方的 MCP（Model Context Protocol）服务器实现，包含完整的 API 调用示例：

- **Python 版本**: https://github.com/MiniMax-AI/MiniMax-MCP
- **JavaScript 版本**: https://github.com/MiniMax-AI/MiniMax-MCP-JS

### 关于 OpenAPI/Swagger

MiniMax 目前**没有公开提供** OpenAPI/Swagger 规范文件。如需获取，可以：
- 联系官方技术支持: api-support@minimaxi.com
- 基于官方文档手动整理

## 概述

MiniMax 开放平台提供多模态 AI 能力，包括文本生成、语音合成、视频生成、图像生成、音乐生成等。

## API 能力概览

| 能力模块 | 说明 | 文档链接 |
|---------|------|---------|
| 文本生成 | 对话内容生成、工具调用 | [text.md](./text.md) |
| 同步语音合成 (T2A) | 短文本语音合成，支持 HTTP/WebSocket | [speech-t2a.md](./speech-t2a.md) |
| 异步长文本语音合成 | 长文本语音合成，异步任务模式 | [speech-t2a-async.md](./speech-t2a-async.md) |
| 音色快速复刻 | 上传音频复刻音色 | [voice-cloning.md](./voice-cloning.md) |
| 音色设计 | 基于描述生成个性化音色 | [voice-design.md](./voice-design.md) |
| 声音管理 | 查询和管理可用音色 | [voice-management.md](./voice-management.md) |
| 视频生成 | 文生视频、图生视频 | [video.md](./video.md) |
| 视频生成 Agent | 基于模板的视频生成 | [video-agent.md](./video-agent.md) |
| 图片生成 | 文生图、图生图 | [image.md](./image.md) |
| 音乐生成 | 基于描述和歌词生成音乐 | [music.md](./music.md) |
| 文件管理 | 文件上传、下载、管理 | [file.md](./file.md) |

## 认证方式

所有 API 使用 Bearer Token 认证：

```
Authorization: Bearer <your_api_key>
```

### 获取 API Key

1. **按量付费**: 在「账户管理 > 接口密钥」中创建 API Key，支持所有模态模型
2. **Coding Plan**: 创建 Coding Plan Key，仅支持文本模型

## 基础 URL

| 地址类型 | URL |
|---------|-----|
| 主要地址 | `https://api.minimaxi.com` |
| 备用地址 | `https://api-bj.minimaxi.com` |

## 请求头

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| Authorization | string | 是 | `Bearer <api_key>` |
| Content-Type | string | 是 | `application/json` |

## 错误处理

API 返回的错误响应格式：

```json
{
  "base_resp": {
    "status_code": 1000,
    "status_msg": "error message"
  }
}
```

## 官方资源

- [MiniMax 开放平台](https://platform.minimaxi.com)
- [官方 MCP 服务器 (Python)](https://github.com/MiniMax-AI/MiniMax-MCP)
- [官方 MCP 服务器 (JavaScript)](https://github.com/MiniMax-AI/MiniMax-MCP-JS)
- 技术支持邮箱: api-support@minimaxi.com
