# API Key 接口

## 原始文档

| 接口 | 链接 |
|------|------|
| ListAPIKeys | https://www.volcengine.com/docs/6561/1772905 |
| CreateAPIKey | https://www.volcengine.com/docs/6561/1772906 |
| DeleteAPIKey | https://www.volcengine.com/docs/6561/1772907 |
| UpdateAPIKey | https://www.volcengine.com/docs/6561/1772908 |

> 如果本文档信息不完整，请访问上述链接获取最新内容。

## ListAPIKeys

获取 API Key 列表。

### 请求

```
GET https://open.volcengineapi.com/?Action=ListAPIKeys&Version=2024-01-01
```

### 响应

```json
{
  "Result": {
    "APIKeys": [
      {
        "APIKeyId": "ak_xxx",
        "Name": "生产环境",
        "Status": "active",
        "CreatedAt": "2024-01-15T10:00:00Z",
        "ExpiredAt": "2025-01-15T10:00:00Z"
      }
    ]
  }
}
```

---

## CreateAPIKey

创建新的 API Key。

### 请求

```
POST https://open.volcengineapi.com/?Action=CreateAPIKey&Version=2024-01-01
```

### 请求参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| Name | string | 是 | API Key 名称 |
| ExpiredAt | string | 否 | 过期时间 |
| Description | string | 否 | 描述 |

### 响应

```json
{
  "Result": {
    "APIKeyId": "ak_xxx",
    "APIKeySecret": "sk_xxx",
    "Name": "生产环境"
  }
}
```

**注意**：APIKeySecret 仅在创建时返回一次，请妥善保存。

---

## DeleteAPIKey

删除 API Key。

### 请求

```
POST https://open.volcengineapi.com/?Action=DeleteAPIKey&Version=2024-01-01
```

### 请求参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| APIKeyId | string | 是 | API Key ID |

---

## UpdateAPIKey

更新 API Key 信息。

### 请求

```
POST https://open.volcengineapi.com/?Action=UpdateAPIKey&Version=2024-01-01
```

### 请求参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| APIKeyId | string | 是 | API Key ID |
| Name | string | 否 | 新名称 |
| Status | string | 否 | 状态：`active`/`inactive` |
| ExpiredAt | string | 否 | 新过期时间 |
