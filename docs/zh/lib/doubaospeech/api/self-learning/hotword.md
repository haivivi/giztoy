# 热词管理 API

## 原始文档

- **热词管理 API v1.0**: https://www.volcengine.com/docs/6561/1251242

> 如果本文档信息不完整，请访问上述链接获取最新内容。

## 概述

热词可以提升特定词汇在语音识别中的识别率。

## 接口列表

| 接口 | 方法 | 路径 | 说明 |
|------|------|------|------|
| 创建热词表 | POST | `/api/v1/hotword/create` | 创建热词表 |
| 更新热词表 | POST | `/api/v1/hotword/update` | 更新热词表 |
| 删除热词表 | POST | `/api/v1/hotword/delete` | 删除热词表 |
| 查询热词表 | GET | `/api/v1/hotword/query` | 查询热词表详情 |
| 列出热词表 | GET | `/api/v1/hotword/list` | 列出所有热词表 |

## 创建热词表

### 请求

```
POST https://openspeech.bytedance.com/api/v1/hotword/create
```

### Body

```json
{
  "appid": "your_appid",
  "name": "金融术语",
  "description": "金融行业专业术语",
  "hotwords": [
    {"word": "基金净值", "weight": 10},
    {"word": "年化收益率", "weight": 9},
    {"word": "资产配置", "weight": 8}
  ]
}
```

### 参数说明

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| name | string | 是 | 热词表名称 |
| description | string | 否 | 描述 |
| hotwords | array | 是 | 热词列表 |
| hotwords[].word | string | 是 | 热词 |
| hotwords[].weight | int | 否 | 权重 1-10，默认 5 |

### 响应

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "hotword_id": "hw_xxx",
    "name": "金融术语",
    "word_count": 3
  }
}
```

## 在识别中使用热词

在 ASR 请求中指定热词表：

```json
{
  "request": {
    "hotword_id": "hw_xxx"
  }
}
```

## 限制

- 单个热词表最多 1000 个词
- 单个热词最长 20 个字符
- 每个账号最多创建 100 个热词表
