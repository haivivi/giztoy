# 文件管理 API

> **官方文档**: https://platform.minimaxi.com/docs/api-reference/file-management

## 概述

文件管理 API 用于配合其他接口使用，提供文件的上传、列出、检索、下载和删除功能。

## 支持的文件格式

| 类型 | 格式 |
|------|------|
| 文档 | pdf, docx, txt, jsonl |
| 音频 | mp3, m4a, wav |

## 容量限制

| 限制项 | 限制值 |
|--------|--------|
| 总容量 | 100 GB |
| 单个文件 | 512 MB |

## 接口说明

文件管理包含 5 个接口：

1. **上传** - 上传文件到平台
2. **列出** - 获取已上传的文件列表
3. **检索** - 获取单个文件的详细信息
4. **下载** - 下载文件内容
5. **删除** - 删除文件

## 上传文件

### 端点

```
POST https://api.minimaxi.com/v1/files
```

### 请求参数

使用 `multipart/form-data` 格式：

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| file | file | 是 | 要上传的文件 |
| purpose | string | 是 | 文件用途 |

### purpose 可选值

| 值 | 说明 |
|-----|------|
| voice_clone | 音色复刻音频 |
| voice_clone_demo | 音色复刻示例音频 |
| t2a_async | 异步语音合成文本文件 |
| fine-tune | 微调数据文件 |
| assistants | 助手文件 |

### 请求示例

```bash
curl --request POST \
  --url https://api.minimaxi.com/v1/files \
  --header 'Authorization: Bearer <your_api_key>' \
  --form 'file=@document.txt' \
  --form 'purpose=t2a_async'
```

### 响应格式

```json
{
  "file": {
    "file_id": "file_abc123",
    "filename": "document.txt",
    "bytes": 1024,
    "created_at": 1704067200,
    "purpose": "t2a_async"
  },
  "base_resp": {
    "status_code": 0,
    "status_msg": "success"
  }
}
```

## 列出文件

### 端点

```
GET https://api.minimaxi.com/v1/files
```

### 查询参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| purpose | string | 否 | 按用途筛选 |
| limit | int | 否 | 返回数量限制，默认 20 |
| after | string | 否 | 分页游标 |

### 请求示例

```bash
curl --request GET \
  --url 'https://api.minimaxi.com/v1/files?purpose=t2a_async&limit=10' \
  --header 'Authorization: Bearer <your_api_key>'
```

### 响应格式

```json
{
  "data": [
    {
      "file_id": "file_abc123",
      "filename": "document.txt",
      "bytes": 1024,
      "created_at": 1704067200,
      "purpose": "t2a_async"
    },
    {
      "file_id": "file_def456",
      "filename": "audio.mp3",
      "bytes": 2048000,
      "created_at": 1704067100,
      "purpose": "voice_clone"
    }
  ],
  "has_more": false,
  "base_resp": {
    "status_code": 0,
    "status_msg": "success"
  }
}
```

## 检索文件

### 端点

```
GET https://api.minimaxi.com/v1/files/{file_id}
```

### 路径参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| file_id | string | 是 | 文件 ID |

### 请求示例

```bash
curl --request GET \
  --url https://api.minimaxi.com/v1/files/file_abc123 \
  --header 'Authorization: Bearer <your_api_key>'
```

### 响应格式

```json
{
  "file": {
    "file_id": "file_abc123",
    "filename": "document.txt",
    "bytes": 1024,
    "created_at": 1704067200,
    "purpose": "t2a_async",
    "status": "processed"
  },
  "base_resp": {
    "status_code": 0,
    "status_msg": "success"
  }
}
```

## 下载文件

### 端点

```
GET https://api.minimaxi.com/v1/files/{file_id}/content
```

### 路径参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| file_id | string | 是 | 文件 ID |

### 请求示例

```bash
curl --request GET \
  --url https://api.minimaxi.com/v1/files/file_abc123/content \
  --header 'Authorization: Bearer <your_api_key>' \
  --output downloaded_file.txt
```

### 响应

返回文件的二进制内容。

## 删除文件

### 端点

```
POST https://api.minimaxi.com/v1/files/{file_id}/delete
```

### 路径参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| file_id | string | 是 | 文件 ID |

### 请求示例

```bash
curl --request POST \
  --url https://api.minimaxi.com/v1/files/file_abc123/delete \
  --header 'Authorization: Bearer <your_api_key>'
```

### 响应格式

```json
{
  "file_id": "file_abc123",
  "deleted": true,
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
    "Authorization": f"Bearer {API_KEY}"
}

# 1. 上传文件
with open("document.txt", "rb") as f:
    response = requests.post(
        f"{BASE_URL}/files",
        headers=HEADERS,
        files={"file": f},
        data={"purpose": "t2a_async"}
    )
file_id = response.json()["file"]["file_id"]
print(f"Uploaded: {file_id}")

# 2. 列出文件
response = requests.get(
    f"{BASE_URL}/files",
    headers=HEADERS,
    params={"limit": 10}
)
files = response.json()["data"]
print(f"Total files: {len(files)}")

# 3. 检索文件信息
response = requests.get(
    f"{BASE_URL}/files/{file_id}",
    headers=HEADERS
)
file_info = response.json()["file"]
print(f"File info: {file_info}")

# 4. 下载文件
response = requests.get(
    f"{BASE_URL}/files/{file_id}/content",
    headers=HEADERS
)
with open("downloaded.txt", "wb") as f:
    f.write(response.content)
print("File downloaded!")

# 5. 删除文件
response = requests.post(
    f"{BASE_URL}/files/{file_id}/delete",
    headers=HEADERS
)
print(f"Deleted: {response.json()['deleted']}")
```

## 注意事项

1. **容量限制**: 总容量 100 GB，单个文件最大 512 MB
2. **文件有效期**: 某些类型的文件（如视频生成结果）有下载有效期
3. **用途匹配**: 上传文件时需要指定正确的 `purpose`，以便与相应的 API 配合使用
4. **文件状态**: 上传后的文件可能需要处理，可通过检索接口查看状态
