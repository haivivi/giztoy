# 替换词 API

## 原始文档

- **替换词 API v1.1**: https://www.volcengine.com/docs/6561/1251244

> 如果本文档信息不完整，请访问上述链接获取最新内容。

## 概述

替换词可以将识别结果中的特定词汇替换为指定内容。

## 接口列表

| 接口 | 方法 | 路径 | 说明 |
|------|------|------|------|
| 创建替换词表 | POST | `/api/v1/replacement/create` | 创建替换词表 |
| 更新替换词表 | POST | `/api/v1/replacement/update` | 更新替换词表 |
| 删除替换词表 | POST | `/api/v1/replacement/delete` | 删除替换词表 |
| 查询替换词表 | GET | `/api/v1/replacement/query` | 查询详情 |

## 创建替换词表

### 请求

```
POST https://openspeech.bytedance.com/api/v1/replacement/create
```

### Body

```json
{
  "appid": "your_appid",
  "name": "敏感词过滤",
  "description": "过滤竞品名称",
  "replacements": [
    {"source": "竞品A", "target": "某产品"},
    {"source": "竞品B", "target": "某服务"}
  ]
}
```

### 参数说明

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| name | string | 是 | 替换词表名称 |
| description | string | 否 | 描述 |
| replacements | array | 是 | 替换规则列表 |
| replacements[].source | string | 是 | 源词 |
| replacements[].target | string | 是 | 目标词 |

### 响应

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "replacement_id": "rp_xxx",
    "name": "敏感词过滤",
    "rule_count": 2
  }
}
```

## 在识别中使用替换词

在 ASR 请求中指定替换词表：

```json
{
  "request": {
    "replacement_id": "rp_xxx"
  }
}
```

## 替换规则

- 支持精确匹配
- 支持正则表达式（部分场景）
- 替换发生在识别结果后处理阶段

## 限制

- 单个替换词表最多 500 条规则
- 源词最长 50 个字符
- 目标词最长 100 个字符
