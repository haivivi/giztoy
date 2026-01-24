# SSML 标记语言

## 原始文档

- **SSML标记语言**: https://www.volcengine.com/docs/6561/1257543

> 如果本文档信息不完整，请访问上述链接获取最新内容。

## 概述

SSML（Speech Synthesis Markup Language）是用于控制语音合成的标记语言。

## 基本格式

```xml
<speak>
  你好，<break time="500ms"/>欢迎使用豆包语音。
</speak>
```

## 支持的标签

### speak

根标签，包裹所有内容。

```xml
<speak version="1.0">
  内容...
</speak>
```

### break

插入停顿。

| 属性 | 说明 |
|------|------|
| time | 停顿时长，如 `500ms`、`1s` |
| strength | 停顿强度：`none`/`x-weak`/`weak`/`medium`/`strong`/`x-strong` |

```xml
<speak>
  第一句话。<break time="1s"/>第二句话。
</speak>
```

### prosody

控制语音的韵律。

| 属性 | 说明 | 范围 |
|------|------|------|
| rate | 语速 | `x-slow`/`slow`/`medium`/`fast`/`x-fast` 或百分比 |
| pitch | 音调 | `x-low`/`low`/`medium`/`high`/`x-high` 或 Hz |
| volume | 音量 | `silent`/`x-soft`/`soft`/`medium`/`loud`/`x-loud` 或 dB |

```xml
<speak>
  <prosody rate="fast" pitch="high">快速高音</prosody>
  <prosody rate="80%" volume="+6dB">慢速响亮</prosody>
</speak>
```

### say-as

指定文本的发音方式。

| interpret-as | 说明 |
|--------------|------|
| cardinal | 数字 |
| ordinal | 序数词 |
| characters | 逐字符读 |
| date | 日期 |
| time | 时间 |
| telephone | 电话号码 |

```xml
<speak>
  电话号码是<say-as interpret-as="telephone">10086</say-as>。
  日期是<say-as interpret-as="date">2024-01-15</say-as>。
</speak>
```

### sub

替换文本的发音。

```xml
<speak>
  <sub alias="世界卫生组织">WHO</sub>发布了新报告。
</speak>
```

### phoneme

指定音标发音。

```xml
<speak>
  <phoneme alphabet="pinyin" ph="zhong1 guo2">中国</phoneme>
</speak>
```

### emphasis

强调。

| level | 说明 |
|-------|------|
| strong | 强调 |
| moderate | 中等强调 |
| reduced | 弱化 |

```xml
<speak>
  这是<emphasis level="strong">非常重要</emphasis>的内容。
</speak>
```

### audio

插入音频文件。

```xml
<speak>
  开始播放音效：
  <audio src="https://example.com/sound.mp3"/>
  音效结束。
</speak>
```

## 完整示例

```xml
<speak version="1.0">
  <prosody rate="medium" pitch="medium">
    欢迎使用豆包语音合成服务。
  </prosody>
  <break time="500ms"/>
  <prosody rate="fast">
    今天是<say-as interpret-as="date">2024-01-15</say-as>，
    星期一。
  </prosody>
  <break time="300ms"/>
  <emphasis level="strong">请注意</emphasis>，
  服务热线是<say-as interpret-as="telephone">400-123-4567</say-as>。
</speak>
```

## 使用方式

在请求中设置 `text_type` 为 `ssml`：

```json
{
  "request": {
    "text": "<speak>你好<break time=\"500ms\"/>世界</speak>",
    "text_type": "ssml"
  }
}
```
