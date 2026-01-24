# 双向流式 WebSocket 接口 (V3)

## 概述

双向流式 WebSocket 接口支持实时交互，可以在合成过程中动态追加文本。

## 接口信息

| 项目 | 值 |
|------|------|
| 请求地址 | `wss://openspeech.bytedance.com/api/v1/tts/ws_binary` |
| 协议 | WebSocket |

## 与单向流式的区别

| 特性 | 单向流式 | 双向流式 |
|------|----------|----------|
| 文本输入 | 一次性发送 | 可分段追加 |
| 适用场景 | 固定文本 | 实时生成文本（如 LLM 输出） |

## 消息格式

### 开始合成

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
    "encoding": "mp3"
  },
  "request": {
    "reqid": "uuid",
    "text": "第一段文本",
    "text_type": "plain",
    "operation": "submit"
  }
}
```

### 追加文本

```json
{
  "request": {
    "reqid": "uuid",
    "text": "追加的文本",
    "operation": "append"
  }
}
```

### 结束输入

```json
{
  "request": {
    "reqid": "uuid",
    "operation": "finish"
  }
}
```

## Operation 类型

| 值 | 说明 |
|------|------|
| submit | 开始合成 |
| append | 追加文本 |
| finish | 结束输入 |
| cancel | 取消任务 |

## 典型流程

```
客户端                              服务端
   |                                   |
   |--- submit (首段文本) ------------>|
   |<--- 音频数据 ---------------------|
   |--- append (追加文本) ------------>|
   |<--- 音频数据 ---------------------|
   |--- append (追加文本) ------------>|
   |<--- 音频数据 ---------------------|
   |--- finish ----------------------->|
   |<--- 剩余音频 ---------------------|
   |<--- 结束标记 ---------------------|
```

## Go 客户端示例

```go
// 发送首段文本
request := TTSRequest{
    Request: RequestParams{
        ReqID:     uuid.New().String(),
        Text:      "你好，",
        Operation: "submit",
    },
}
conn.WriteJSON(request)

// 模拟 LLM 流式输出
texts := []string{"今天", "天气", "真不错！"}
for _, text := range texts {
    conn.WriteJSON(map[string]interface{}{
        "request": map[string]interface{}{
            "reqid":     request.Request.ReqID,
            "text":      text,
            "operation": "append",
        },
    })
    time.Sleep(100 * time.Millisecond)
}

// 结束输入
conn.WriteJSON(map[string]interface{}{
    "request": map[string]interface{}{
        "reqid":     request.Request.ReqID,
        "operation": "finish",
    },
})

// 持续接收音频直到结束
```
