# 音色设计 API (Voice Design)

> **官方文档**: https://platform.minimaxi.com/docs/api-reference/speech-voice-design

## 概述

音色设计 API 允许用户通过文本描述（prompt）生成个性化音色，生成的音色可用于语音合成接口。

## 支持的模型

| 模型 | 说明 |
|------|------|
| speech-02-hd | 推荐使用，效果最佳 |
| speech-02-turbo | 性能出色 |
| speech-2.6-hd | 最新 HD 模型 |
| speech-2.6-turbo | 最新 Turbo 模型 |

## 端点

```
POST https://api.minimaxi.com/v1/voice_design
```

## 请求参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| prompt | string | 是 | 音色描述，描述想要的声音特征 |
| preview_text | string | 是 | 试听文本 |
| voice_id | string | 否 | 自定义的音色 ID |
| model | string | 否 | 模型版本，默认 speech-02-hd |
| output_directory | string | 否 | 输出目录 |

## Prompt 编写指南

好的 prompt 应该包含以下特征描述：

- **性别**: 男性、女性
- **年龄**: 年轻、中年、老年
- **音色特点**: 温柔、磁性、清脆、浑厚、沙哑等
- **语气风格**: 活泼、沉稳、专业、亲切等
- **适用场景**: 新闻播报、有声书、客服、广告等

### Prompt 示例

```
一个温柔的年轻女性声音，音色清脆甜美，语气亲切自然，适合有声书朗读
```

```
一个成熟稳重的男性声音，音色浑厚有磁性，语气专业沉稳，适合新闻播报
```

```
一个活泼可爱的少女声音，音色清亮，语气俏皮，适合动画配音
```

## 请求示例

```bash
curl --request POST \
  --url https://api.minimaxi.com/v1/voice_design \
  --header 'Authorization: Bearer <your_api_key>' \
  --header 'Content-Type: application/json' \
  --data '{
    "prompt": "一个温柔的年轻女性声音，音色清脆甜美，语气亲切自然",
    "preview_text": "你好，欢迎使用 MiniMax 语音合成服务",
    "voice_id": "my_designed_voice",
    "model": "speech-02-hd"
  }'
```

## 响应格式

```json
{
  "voice_id": "my_designed_voice",
  "demo_audio": "<hex编码的试听音频>",
  "base_resp": {
    "status_code": 0,
    "status_msg": "success"
  }
}
```

## 使用设计的音色

设计成功后，可在语音合成接口中使用：

```json
{
  "model": "speech-2.6-hd",
  "text": "使用设计的音色合成语音",
  "voice_setting": {
    "voice_id": "my_designed_voice"
  }
}
```

## 注意事项

1. **临时音色**: 设计的音色为临时音色，若 **7 天内** 未被用于语音合成（试听不算），将被自动删除
2. **费用计算**: 设计费用在首次使用语音合成时收取
3. **推荐模型**: 建议使用 `speech-02-hd` 模型以获得最佳效果
4. **无状态设计**: 接口不存储用户数据
5. **Prompt 质量**: prompt 的描述越详细、越具体，生成的音色效果越好
