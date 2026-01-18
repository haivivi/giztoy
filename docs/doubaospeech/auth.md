# 鉴权方式

## 原始文档

- **鉴权方法**: https://www.volcengine.com/docs/6561/1221057

> 如果本文档信息不完整，请访问上述链接获取最新内容。

## 概述

豆包语音 API 使用 Token 认证方式，需要在请求中携带以下信息：

## 必需参数

| 参数 | 位置 | 说明 |
|------|------|------|
| appid | Body/Query | 应用 ID，控制台创建应用后获取 |
| token | Header | 访问令牌，控制台生成 |
| cluster | Body | 集群标识，不同产品使用不同集群 |

## 请求头

```
Authorization: Bearer;{token}
```

注意：Bearer 和 token 之间用分号 `;` 分隔，不是空格。

## 获取凭证

1. 登录 [火山引擎控制台](https://console.volcengine.com/)
2. 进入「语音技术」
3. 创建应用，获取 `appid`
4. 生成访问令牌，获取 `token`
5. 根据使用的产品，查看对应的 `cluster` 值

## Cluster 列表

### 语音合成2.0

| Cluster | 说明 |
|---------|------|
| `volcano_tts` | 标准版 |
| `volcano_mega` | 大模型版 |
| `volcano_icl` | 声音复刻版 |

### 语音识别

| Cluster | 说明 |
|---------|------|
| `volcano_asr` | 流式识别 |
| `volcengine_streaming_common` | 通用识别 |

## 示例

```bash
curl -X POST "https://openspeech.bytedance.com/api/v1/tts" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer;your_token_here" \
  -d '{
    "app": {
      "appid": "your_appid",
      "cluster": "volcano_tts"
    },
    "user": {
      "uid": "user_001"
    },
    "audio": {
      "voice_type": "zh_female_cancan"
    },
    "request": {
      "text": "你好世界"
    }
  }'
```
