# 语音合成 1.0（经典版）

## 原始文档

| 文档 | 链接 |
|------|------|
| 产品简介 | https://www.volcengine.com/docs/6561/79817 |
| HTTP接口(一次性合成-非流式) | https://www.volcengine.com/docs/6561/79820 |
| WebSocket接口 | https://www.volcengine.com/docs/6561/79823 |
| 参数基本说明 | https://www.volcengine.com/docs/6561/79824 |
| 发音人参数列表 | https://www.volcengine.com/docs/6561/79825 |
| 鉴权方法 | https://www.volcengine.com/docs/6561/79826 |
| API接口文档 | https://www.volcengine.com/docs/6561/79827 |
| 音色列表 | https://www.volcengine.com/docs/6561/79828 |
| SSML标记语言 | https://www.volcengine.com/docs/6561/79829 |
| 接入FAQ | https://www.volcengine.com/docs/6561/79830 |

> 如果本文档信息不完整，请访问上述链接获取最新内容。

## 概述

语音合成 1.0 是经典版本的 TTS 服务，稳定可靠，适用于对延迟和稳定性有要求的场景。

## 与 2.0 版本对比

| 特性 | 1.0 经典版 | 2.0 大模型版 |
|------|-----------|-------------|
| 音质 | 标准 | 更自然 |
| 延迟 | 低 | 略高 |
| 稳定性 | 高 | 高 |
| 情感控制 | 有限 | 丰富 |
| 音色数量 | 多 | 持续增加 |

## 接入方式

| 方式 | 说明 | 文档 |
|------|------|------|
| HTTP | 一次性合成，非流式 | [http.md](./http.md) |
| WebSocket | 流式合成 | [websocket.md](./websocket.md) |

## 认证方式

经典版使用不同的认证方式，详见 [鉴权方法](https://www.volcengine.com/docs/6561/79826)。
