# 单向流式 WebSocket 接口 (V3)

## 概述

单向流式 WebSocket 接口提供低延迟的流式语音合成，适用于对延迟敏感的场景。

## 接口信息

| 项目 | 值 |
|------|------|
| 请求地址 | `wss://openspeech.bytedance.com/api/v1/tts/ws_binary` |
| 协议 | WebSocket |

## 连接参数

WebSocket 连接时通过 URL 参数传递认证信息：

```
wss://openspeech.bytedance.com/api/v1/tts/ws_binary?appid=xxx&token=xxx&cluster=volcano_tts
```

## 消息格式

### 客户端发送

```json
{
  "app": {
    "appid": "your_appid",
    "cluster": "volcano_tts"
  },
  "user": {
    "uid": "user_001"
  },
  "audio": {
    "voice_type": "zh_female_cancan",
    "encoding": "mp3",
    "speed_ratio": 1.0
  },
  "request": {
    "reqid": "uuid",
    "text": "你好，世界！",
    "text_type": "plain",
    "operation": "submit"
  }
}
```

### 服务端响应

服务端以二进制帧返回音频数据，消息头包含元信息：

| 字节 | 说明 |
|------|------|
| 0-3 | 消息类型标识 |
| 4-7 | 序列号 |
| 8-11 | 数据长度 |
| 12+ | 音频数据 |

### 消息类型

| 类型 | 值 | 说明 |
|------|-----|------|
| Audio | 0x01 | 音频数据 |
| End | 0x02 | 结束标记 |
| Error | 0xFF | 错误信息 |

## 流程

1. 建立 WebSocket 连接
2. 发送合成请求（JSON）
3. 接收音频数据（二进制帧）
4. 收到结束标记后关闭连接

## Go 客户端示例

```go
import "github.com/gorilla/websocket"

// 建立连接
url := "wss://openspeech.bytedance.com/api/v1/tts/ws_binary?appid=xxx&token=xxx&cluster=volcano_tts"
conn, _, err := websocket.DefaultDialer.Dial(url, nil)
if err != nil {
    return err
}
defer conn.Close()

// 发送请求
request := TTSRequest{...}
if err := conn.WriteJSON(request); err != nil {
    return err
}

// 接收音频
for {
    msgType, data, err := conn.ReadMessage()
    if err != nil {
        break
    }
    
    if msgType == websocket.BinaryMessage {
        // 解析消息头
        seqType := data[0:4]
        if isEndMarker(seqType) {
            break
        }
        audioData := data[12:]
        // 处理音频数据...
    }
}
```
