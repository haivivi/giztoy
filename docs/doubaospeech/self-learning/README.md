# 自学习平台

## 原始文档

| 文档 | 链接 |
|------|------|
| 产品简介 | https://www.volcengine.com/docs/6561/1251240 |
| 热词 | https://www.volcengine.com/docs/6561/1251241 |
| 热词管理 API v1.0 | https://www.volcengine.com/docs/6561/1251242 |
| 替换词 | https://www.volcengine.com/docs/6561/1251243 |
| 替换词 API v1.1 | https://www.volcengine.com/docs/6561/1251244 |
| 常见问题 | https://www.volcengine.com/docs/6561/1251245 |

> 如果本文档信息不完整，请访问上述链接获取最新内容。

## 概述

自学习平台允许用户自定义热词和替换词，提升语音识别在特定领域的准确率。

## 功能列表

| 功能 | 说明 | 文档 |
|------|------|------|
| 热词 | 提升特定词汇的识别率 | [hotword.md](./hotword.md) |
| 替换词 | 将识别结果中的词替换为指定词 | [replacement.md](./replacement.md) |

## 热词

### 使用场景

- 专业术语
- 人名、地名
- 品牌名称
- 行业词汇

### 示例

```json
{
  "hotwords": [
    {"word": "火山引擎", "weight": 10},
    {"word": "豆包", "weight": 8},
    {"word": "ByteDance", "weight": 5}
  ]
}
```

## 替换词

### 使用场景

- 敏感词过滤
- 统一词汇表述
- 错误纠正

### 示例

```json
{
  "replacements": [
    {"source": "微信", "target": "IM软件"},
    {"source": "阿里巴巴", "target": "某电商公司"}
  ]
}
```
