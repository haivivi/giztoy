# 豆包语音合成2.0

## 原始文档

| 文档 | 链接 |
|------|------|
| 产品简介 | https://www.volcengine.com/docs/6561/1234523 |
| 大模型语音合成API | https://www.volcengine.com/docs/6561/1257584 |
| 单向流式HTTP-V3 | https://www.volcengine.com/docs/6561/1329505 |
| 单向流式WebSocket-V3 | https://www.volcengine.com/docs/6561/1329506 |
| 双向流式WebSocket-V3 | https://www.volcengine.com/docs/6561/1329507 |
| 异步长文本接口 | https://www.volcengine.com/docs/6561/1096680 |
| 音色列表 | https://www.volcengine.com/docs/6561/1257544 |
| SSML标记语言 | https://www.volcengine.com/docs/6561/1257543 |

> 如果本文档信息不完整，请访问上述链接获取最新内容。

## 概述

豆包语音合成2.0 是基于大模型的语音合成服务，支持多种接入方式。

## 接口列表

| 接口 | 协议 | 特点 | 文档 |
|------|------|------|------|
| 大模型语音合成 | HTTP/WebSocket | 推荐，统一入口 | [api.md](./api.md) |
| 单向流式 HTTP | HTTP | 流式输出音频 | [stream-http.md](./stream-http.md) |
| 单向流式 WebSocket | WebSocket | 低延迟流式 | [stream-ws.md](./stream-ws.md) |
| 双向流式 WebSocket | WebSocket | 实时交互 | [duplex-ws.md](./duplex-ws.md) |
| 异步长文本 | HTTP | 大文本离线合成 | [async.md](./async.md) |

## 功能特性

- ✅ 大模型音色，效果更自然
- ✅ 支持声音复刻（ICL）
- ✅ 支持混音（Mix）
- ✅ 支持 SSML 标记
- ✅ 支持情感控制
- ✅ 支持多语种

## 音色分类

| 类型 | 说明 |
|------|------|
| 通用音色 | 预置的标准音色 |
| 精品音色 | 高品质定制音色 |
| 复刻音色 | 用户自定义复刻音色 |

详见 [音色列表](./voices.md)
