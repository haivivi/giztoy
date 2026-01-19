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
POST https://api.minimaxi.com/v1/files/upload
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
| voice_clone | 音色复刻原始音频 |
| prompt_audio | 音色复刻示例音频 |
| t2a_async_input | 异步语音合成文本文件 |

### 请求示例

```bash
curl --request POST \
  --url https://api.minimaxi.com/v1/files/upload \
  --header 'Authorization: Bearer <your_api_key>' \
  --form 'file=@document.txt' \
  --form 'purpose=t2a_async_input'
```

### 响应格式

```json
{
  "file": {
    "file_id": "file_abc123",
    "filename": "document.txt",
    "bytes": 1024,
    "created_at": 1704067200,
    "purpose": "t2a_async_input"
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
GET https://api.minimaxi.com/v1/files/list
```

### 查询参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| purpose | string | 是 | 按用途筛选，可选值：`voice_clone`, `prompt_audio`, `t2a_async_input` |

### 请求示例

```bash
curl --request GET \
  --url 'https://api.minimaxi.com/v1/files/list?purpose=voice_clone' \
  --header 'Authorization: Bearer <your_api_key>'
```

### 响应格式

```json
{
  "files": [
    {
      "file_id": "297990555456011",
      "bytes": 5896337,
      "created_at": 1699964873,
      "filename": "audio.mp3",
      "purpose": "voice_clone"
    },
    {
      "file_id": "297990555456911",
      "bytes": 5896337,
      "created_at": 1700469398,
      "filename": "audio2.mp3",
      "purpose": "voice_clone"
    }
  ],
  "base_resp": {
    "status_code": 0,
    "status_msg": "success"
  }
}
```

## 检索文件

### 端点

```
GET https://api.minimaxi.com/v1/files/retrieve
```

### 查询参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| file_id | string | 是 | 文件 ID |

### 请求示例

```bash
curl --request GET \
  --url 'https://api.minimaxi.com/v1/files/retrieve?file_id=file_abc123' \
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
    "purpose": "t2a_async_input",
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
GET https://api.minimaxi.com/v1/files/retrieve_content
```

### 查询参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| file_id | integer | 是 | 需要下载的文件 ID |

### 请求示例

```bash
curl --request GET \
  --url 'https://api.minimaxi.com/v1/files/retrieve_content?file_id=file_abc123' \
  --header 'Authorization: Bearer <your_api_key>' \
  --output downloaded_file.txt
```

### 响应

返回文件的二进制内容。

## 删除文件

### 端点

```
POST https://api.minimaxi.com/v1/files/delete
```

### 请求参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| file_id | int64 | 是 | 文件 ID（数字类型） |
| purpose | string | 是 | 文件用途，必须与上传时指定的用途一致。可选值：`voice_clone`、`prompt_audio`、`t2a_async_input` |

### 请求示例

```bash
curl --request POST \
  --url https://api.minimaxi.com/v1/files/delete \
  --header 'Authorization: Bearer <your_api_key>' \
  --header 'Content-Type: application/json' \
  --data '{"file_id": 123456789, "purpose": "voice_clone"}'
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
        f"{BASE_URL}/files/upload",
        headers=HEADERS,
        files={"file": f},
        data={"purpose": "t2a_async_input"}
    )
file_id = response.json()["file"]["file_id"]
print(f"Uploaded: {file_id}")

# 2. 列出文件
response = requests.get(
    f"{BASE_URL}/files/list",
    headers=HEADERS,
    params={"purpose": "t2a_async_input"}
)
files = response.json()["files"]
print(f"Total files: {len(files)}")

# 3. 检索文件信息
response = requests.get(
    f"{BASE_URL}/files/retrieve",
    headers=HEADERS,
    params={"file_id": file_id}
)
file_info = response.json()["file"]
print(f"File info: {file_info}")

# 4. 下载文件
response = requests.get(
    f"{BASE_URL}/files/retrieve_content",
    headers=HEADERS,
    params={"file_id": file_id}
)
with open("downloaded.txt", "wb") as f:
    f.write(response.content)
print("File downloaded!")

# 5. 删除文件
response = requests.post(
    f"{BASE_URL}/files/delete",
    headers={**HEADERS, "Content-Type": "application/json"},
    json={"file_id": file_id, "purpose": "voice_clone"}
)
print(f"Deleted: {response.json()['deleted']}")
```

## 注意事项

1. **容量限制**: 总容量 100 GB，单个文件最大 512 MB
2. **文件有效期**: 某些类型的文件（如视频生成结果）有下载有效期
3. **用途匹配**: 上传文件时需要指定正确的 `purpose`，以便与相应的 API 配合使用
4. **列出文件必填参数**: `purpose` 参数是必填的，不能列出所有文件
