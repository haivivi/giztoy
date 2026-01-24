# 音色列表

## 通用音色

### 中文女声

| 音色 ID | 名称 | 特点 |
|---------|------|------|
| zh_female_cancan | 灿灿 | 甜美活泼 |
| zh_female_shuangshuan | 爽爽 | 知性温柔 |
| zh_female_qingxin | 清新 | 清新自然 |
| zh_female_tianmei | 甜美 | 温柔甜美 |

### 中文男声

| 音色 ID | 名称 | 特点 |
|---------|------|------|
| zh_male_yangguang | 阳光 | 阳光活力 |
| zh_male_wenzhong | 稳重 | 成熟稳重 |
| zh_male_qingsong | 轻松 | 轻松随和 |

### 英文音色

| 音色 ID | 名称 | 特点 |
|---------|------|------|
| en_female_sweet | Sweet | 甜美英音 |
| en_male_warm | Warm | 温暖男声 |

### 多语种音色

| 音色 ID | 语言 | 特点 |
|---------|------|------|
| ja_female_warm | 日语 | 温柔女声 |
| ko_female_sweet | 韩语 | 甜美女声 |

## 精品音色

精品音色需要单独开通，具体列表请在控制台查看。

## 复刻音色

复刻音色格式：`{voice_type}_{user_id}`

使用复刻音色时需要：
1. 先通过声音复刻 API 创建音色
2. 获取复刻音色 ID
3. 在 cluster 中使用 `volcano_icl`

## 音色参数

每个音色支持以下调节参数：

| 参数 | 范围 | 默认 | 说明 |
|------|------|------|------|
| speed_ratio | 0.2 - 3.0 | 1.0 | 语速 |
| volume_ratio | 0.1 - 3.0 | 1.0 | 音量 |
| pitch_ratio | 0.1 - 3.0 | 1.0 | 音调 |

## 情感控制

部分音色支持情感控制：

| emotion | 说明 |
|---------|------|
| happy | 开心 |
| sad | 悲伤 |
| angry | 愤怒 |
| fearful | 恐惧 |
| surprised | 惊讶 |
| neutral | 中性 |

示例：

```json
{
  "audio": {
    "voice_type": "zh_female_cancan",
    "emotion": "happy"
  }
}
```
