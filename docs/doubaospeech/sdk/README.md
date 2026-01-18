# SDK 接入文档

## 原始文档

| 文档 | 链接 |
|------|------|
| 离在线语音合成SDK - SDK概览 | https://www.volcengine.com/docs/6561/1221056 |
| 离在线语音合成SDK - Android集成指南 | https://www.volcengine.com/docs/6561/1221057 |
| 离在线语音合成SDK - iOS集成指南 | https://www.volcengine.com/docs/6561/1221058 |
| 流式语音识别SDK - SDK概览 | https://www.volcengine.com/docs/6561/1221060 |
| 流式语音识别SDK - Android集成指南 | https://www.volcengine.com/docs/6561/1221061 |
| 流式语音识别SDK - iOS集成指南 | https://www.volcengine.com/docs/6561/1221062 |
| 大模型流式识别SDK | https://www.volcengine.com/docs/6561/1354880 |
| 双向流式TTS - iOS SDK | https://www.volcengine.com/docs/6561/1329512 |
| 双向流式TTS - Android SDK | https://www.volcengine.com/docs/6561/1329513 |

> 如果本文档信息不完整，请访问上述链接获取最新内容。

## 概述

豆包语音提供多种移动端 SDK，支持离线和在线语音能力。

## SDK 列表

### 语音合成 SDK

| SDK | 平台 | 特点 |
|-----|------|------|
| 离在线语音合成 SDK | Android/iOS | 支持离线合成，无网络也可使用 |
| 双向流式 TTS SDK | Android/iOS | 实时交互，配合 LLM 使用 |

### 语音识别 SDK

| SDK | 平台 | 特点 |
|-----|------|------|
| 流式语音识别 SDK | Android/iOS | 实时语音识别 |
| 大模型流式识别 SDK | Android/iOS | 基于大模型，精度更高 |

## 离在线语音合成 SDK

### 功能特点

- ✅ 支持离线合成（需下载模型）
- ✅ 支持在线合成
- ✅ 自动切换离线/在线模式
- ✅ 低延迟播放

### 集成方式

#### Android

```gradle
implementation 'com.volcengine:tts-sdk:x.x.x'
```

#### iOS

```ruby
pod 'VolcengineTTS', '~> x.x.x'
```

详见：
- [Android 集成指南](./tts-android.md)
- [iOS 集成指南](./tts-ios.md)

## 流式语音识别 SDK

### 功能特点

- ✅ 实时语音识别
- ✅ VAD 端点检测
- ✅ 支持多语种
- ✅ 低功耗

### 集成方式

#### Android

```gradle
implementation 'com.volcengine:asr-sdk:x.x.x'
```

#### iOS

```ruby
pod 'VolcengineASR', '~> x.x.x'
```

详见：
- [Android 集成指南](./asr-android.md)
- [iOS 集成指南](./asr-ios.md)
