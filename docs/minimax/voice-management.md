# 声音管理 API

> **官方文档**: https://platform.minimaxi.com/docs/api-reference/voice-management-get

## 概述

声音管理 API 用于查询和管理可用的音色，包括系统音色和用户自定义音色。

## 接口说明

声音管理包含 2 个接口：

1. **查询可用音色** - 获取所有可用音色列表
2. **删除音色** - 删除用户自定义音色

## 查询可用音色

### 端点

```
POST https://api.minimaxi.com/v1/get_voice
```

### 请求参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| voice_type | string | 否 | 音色类型：`all`（默认）、`system`、`voice_cloning`、`voice_generation` |

### 请求示例

```bash
curl --request POST \
  --url https://api.minimaxi.com/v1/get_voice \
  --header 'Authorization: Bearer <your_api_key>' \
  --header 'Content-Type: application/json' \
  --data '{
    "voice_type": "all"
  }'
```

### 响应格式

```json
{
  "system_voice": [
    {
      "voice_id": "male-qn-qingse",
      "voice_name": "青涩青年音",
      "description": ["青涩的年轻男性声音"]
    },
    {
      "voice_id": "female-shaonv",
      "voice_name": "少女音",
      "description": ["清脆的少女声音"]
    }
  ],
  "voice_cloning": [
    {
      "voice_id": "my_cloned_voice",
      "voice_name": "我的克隆音色",
      "created_time": "2024-01-15T10:30:00Z"
    }
  ],
  "voice_generation": [
    {
      "voice_id": "my_designed_voice",
      "voice_name": "我的设计音色",
      "created_time": "2024-01-16T14:20:00Z"
    }
  ],
  "base_resp": {
    "status_code": 0,
    "status_msg": "success"
  }
}
```

### 音色类型说明

| 类型 | 说明 |
|------|------|
| system | 系统预置音色，300+ 种可选 |
| voice_cloning | 用户通过音色复刻创建的音色 |
| voice_generation | 用户通过音色设计创建的音色 |

## 常用系统音色

| voice_id | 名称 | 说明 |
|----------|------|------|
| male-qn-qingse | 青涩青年音 | 青涩的年轻男性声音 |
| male-qn-jingying | 精英青年音 | 成熟稳重的男性声音 |
| male-qn-badao | 霸道青年音 | 霸气的男性声音 |
| female-shaonv | 少女音 | 清脆的少女声音 |
| female-yujie | 御姐音 | 成熟女性声音 |
| female-chengshu | 成熟女性音 | 稳重的成熟女性声音 |
| presenter_male | 男性播音员 | 专业播音风格 |
| presenter_female | 女性播音员 | 专业播音风格 |
| audiobook_male_1 | 有声书男声1 | 适合有声书朗读 |
| audiobook_female_1 | 有声书女声1 | 适合有声书朗读 |
| cute_boy | 可爱男孩 | 童声 |
| Charming_Lady | 魅力女性 | 有魅力的女性声音 |

## 删除音色

### 端点

```
POST https://api.minimaxi.com/v1/delete_voice
```

### 请求参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| voice_id | string | 是 | 要删除的音色 ID |
| voice_type | string | 是 | 音色类型：`voice_cloning` 或 `voice_generation` |

### 请求示例

```bash
curl --request POST \
  --url https://api.minimaxi.com/v1/delete_voice \
  --header 'Authorization: Bearer <your_api_key>' \
  --header 'Content-Type: application/json' \
  --data '{
    "voice_id": "my_custom_voice",
    "voice_type": "voice_cloning"
  }'
```

### 响应格式

```json
{
  "voice_id": "my_custom_voice",
  "description": null,
  "created_time": "",
  "base_resp": {
    "status_code": 0,
    "status_msg": "success"
  }
}
```

## 注意事项

1. **系统音色**: 系统音色不可删除，只能删除 `voice_cloning` 和 `voice_generation` 类型的音色
2. **voice_type 必填**: 删除时必须指定音色类型
3. **临时音色**: 未使用的自定义音色会在 168 小时（7 天）后自动删除
4. **音色数量**: 系统提供 300+ 种预置音色可供选择
