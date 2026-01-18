# 音乐生成 API

> **官方文档**: https://platform.minimaxi.com/docs/api-reference/music-generation

## 概述

音乐生成 API 支持根据歌曲描述（prompt）和歌词（lyrics）生成带人声的歌曲。

## 支持的模型

| 模型名称 | 说明 |
|---------|------|
| music-2.0 | 最新音乐生成模型，支持用户输入音乐灵感和歌词，生成 AI 音乐 |

## 端点

```
POST https://api.minimaxi.com/v1/music_generation
```

## 请求参数

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| prompt | string | 是 | - | 音乐创作灵感，描述风格、情绪、场景等 (10-300 字符) |
| lyrics | string | 是 | - | 歌词内容 (10-600 字符) |
| model | string | 否 | music-2.0 | 模型名称 |
| sample_rate | int | 否 | 32000 | 采样率：16000, 24000, 32000, 44100 |
| bitrate | int | 否 | 128000 | 比特率：32000, 64000, 128000, 256000 |
| format | string | 否 | mp3 | 音频格式：mp3, wav, pcm |

## 歌词格式

歌词使用换行符 `\n` 分隔每行，支持以下结构标签增强音乐性：

| 标签 | 说明 |
|------|------|
| [Intro] | 前奏 |
| [Verse] | 主歌 |
| [Chorus] | 副歌 |
| [Bridge] | 桥段 |
| [Outro] | 尾奏 |

### 歌词示例

```
[Intro]
轻轻的风吹过

[Verse]
走在熟悉的街道
回忆涌上心头
那些年的欢笑
如今都已远走

[Chorus]
时光匆匆流逝
青春不再回头
但那些美好瞬间
永远在心中停留

[Bridge]
也许有一天
我们会再相遇

[Outro]
轻轻的风吹过
带走了忧愁
```

## Prompt 编写指南

好的 prompt 应该描述：

- **音乐风格**: 流行、摇滚、民谣、电子、古典等
- **情绪氛围**: 欢快、忧伤、激昂、温柔、浪漫等
- **适用场景**: 适合雨夜、适合运动、适合放松等
- **乐器偏好**: 钢琴伴奏、吉他为主、电子合成器等

### Prompt 示例

```
流行音乐，温柔忧伤，适合雨夜独处，钢琴伴奏为主，节奏舒缓
```

```
电子舞曲，充满活力，适合健身运动，节奏感强，动感十足
```

```
民谣风格，清新自然，适合午后阳光，吉他弹唱，温暖治愈
```

## 请求示例

```bash
curl --request POST \
  --url https://api.minimaxi.com/v1/music_generation \
  --header 'Authorization: Bearer <your_api_key>' \
  --header 'Content-Type: application/json' \
  --data '{
    "model": "music-2.0",
    "prompt": "流行音乐，温柔忧伤，适合雨夜独处，钢琴伴奏为主",
    "lyrics": "[Verse]\n走在熟悉的街道\n回忆涌上心头\n那些年的欢笑\n如今都已远走\n\n[Chorus]\n时光匆匆流逝\n青春不再回头\n但那些美好瞬间\n永远在心中停留",
    "sample_rate": 32000,
    "bitrate": 128000,
    "format": "mp3"
  }'
```

## 响应格式

```json
{
  "data": {
    "audio": "<hex编码的音频数据>",
    "duration": 60000
  },
  "extra_info": {
    "audio_length": 60000,
    "audio_sample_rate": 32000,
    "audio_size": 960000,
    "bitrate": 128000,
    "audio_format": "mp3"
  },
  "base_resp": {
    "status_code": 0,
    "status_msg": "success"
  }
}
```

## 完整使用示例

```python
import requests

API_KEY = "your_api_key"
BASE_URL = "https://api.minimaxi.com/v1"
HEADERS = {
    "Authorization": f"Bearer {API_KEY}",
    "Content-Type": "application/json"
}

# 生成音乐
response = requests.post(
    f"{BASE_URL}/music_generation",
    headers=HEADERS,
    json={
        "model": "music-2.0",
        "prompt": "流行音乐，温柔忧伤，适合雨夜独处，钢琴伴奏为主",
        "lyrics": """[Verse]
走在熟悉的街道
回忆涌上心头
那些年的欢笑
如今都已远走

[Chorus]
时光匆匆流逝
青春不再回头
但那些美好瞬间
永远在心中停留""",
        "format": "mp3"
    }
)

result = response.json()
if result["base_resp"]["status_code"] == 0:
    # 解码并保存音频
    audio_hex = result["data"]["audio"]
    audio_bytes = bytes.fromhex(audio_hex)
    
    with open("output.mp3", "wb") as f:
        f.write(audio_bytes)
    print("Music saved!")
    print(f"Duration: {result['extra_info']['audio_length']}ms")
else:
    print(f"Error: {result['base_resp']['status_msg']}")
```

## 注意事项

1. **字符限制**: prompt 10-300 字符，lyrics 10-600 字符
2. **歌曲时长**: 目前支持生成最长约 1 分钟的音乐
3. **歌词格式**: 每个中文字符、标点符号、字母都算 1 个字符
4. **结构标签**: 使用结构标签可以增强音乐的层次感和节奏感
5. **音频格式**: 返回的是 hex 编码的音频数据，需要解码后保存
