# 视频生成 Agent API

> **官方文档**: https://platform.minimaxi.com/docs/api-reference/video-generation-agent

## 概述

视频生成 Agent API 支持基于预设模板和用户输入生成视频，采用异步任务模式。

## 接口说明

视频 Agent 接口包含 2 个 API：

1. **创建视频 Agent 任务** - 基于模板创建视频生成任务
2. **查询视频 Agent 任务状态** - 查询任务状态，获取下载地址

## 模板清单

| 模板 ID | 模板名称 | 说明 | media_inputs | text_inputs |
|---------|---------|------|--------------|-------------|
| 392753057216684038 | 跳水 | 上传图片，生成主体完成跳水动作的视频 | 需要 | / |
| 393881433990066176 | 吊环 | 上传宠物照片，生成完成吊环动作的视频 | 需要 | / |
| 393769180141805569 | 绝地求生 | 上传宠物图片并输入野兽种类，生成野外求生视频 | 需要 | 需要 |
| 394246956137422856 | 万物皆可 labubu | 上传人物/宠物照片，生成 labubu 换脸视频 | 需要 | / |
| 393879757702918151 | 麦当劳宠物外卖员 | 上传爱宠照片，生成麦当劳宠物外卖员视频 | 需要 | / |
| 393766210733957121 | 藏族风写真 | 上传面部参考图，生成藏族风视频写真 | 需要 | / |
| 394125185182695432 | 生无可恋 | 输入主角痛苦做某事，生成角色痛苦生活的小动画 | / | 需要 |
| 393857704283172864 | 情书写真 | 上传照片生成冬日雪景写真 | 需要 | / |
| 393866076583718914 | 女模特试穿广告 | 上传服装图片，生成女模特试穿广告 | 需要 | / |
| 398574688191234048 | 四季写真 | 上传人脸照片生成四季写真 | 需要 | / |
| 393876118804459526 | 男模特试穿广告 | 上传服装图片，生成男模特试穿广告 | 需要 | / |

## 创建视频 Agent 任务

### 端点

```
POST https://api.minimaxi.com/v1/video_agent
```

### 请求参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| template_id | string | 是 | 模板 ID |
| media_inputs | array | 视模板而定 | 媒体输入（图片/视频） |
| text_inputs | array | 视模板而定 | 文本输入 |

### media_inputs 对象

| 参数 | 类型 | 说明 |
|------|------|------|
| type | string | 媒体类型：`image` 或 `video` |
| url | string | 媒体文件 URL |
| file_id | string | 或使用 file_id |

### text_inputs 对象

| 参数 | 类型 | 说明 |
|------|------|------|
| key | string | 输入键名 |
| value | string | 输入值 |

### 请求示例

```bash
curl --request POST \
  --url https://api.minimaxi.com/v1/video_agent \
  --header 'Authorization: Bearer <your_api_key>' \
  --header 'Content-Type: application/json' \
  --data '{
    "template_id": "392753057216684038",
    "media_inputs": [
      {
        "type": "image",
        "url": "https://example.com/pet.jpg"
      }
    ]
  }'
```

### 带文本输入的请求示例

```bash
curl --request POST \
  --url https://api.minimaxi.com/v1/video_agent \
  --header 'Authorization: Bearer <your_api_key>' \
  --header 'Content-Type: application/json' \
  --data '{
    "template_id": "393769180141805569",
    "media_inputs": [
      {
        "type": "image",
        "url": "https://example.com/pet.jpg"
      }
    ],
    "text_inputs": [
      {
        "key": "beast_type",
        "value": "老虎"
      }
    ]
  }'
```

### 响应格式

```json
{
  "task_id": "task_agent_abc123",
  "base_resp": {
    "status_code": 0,
    "status_msg": "success"
  }
}
```

## 查询视频 Agent 任务状态

### 端点

```
GET https://api.minimaxi.com/v1/video_agent/{task_id}
```

### 路径参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| task_id | string | 是 | 任务 ID |

### 请求示例

```bash
curl --request GET \
  --url https://api.minimaxi.com/v1/video_agent/task_agent_abc123 \
  --header 'Authorization: Bearer <your_api_key>'
```

### 响应格式

```json
{
  "task_id": "task_agent_abc123",
  "status": "Success",
  "download_url": "https://cdn.minimaxi.com/video/xxx.mp4",
  "base_resp": {
    "status_code": 0,
    "status_msg": "success"
  }
}
```

### 任务状态

| 状态 | 说明 |
|------|------|
| Pending | 任务等待中 |
| Processing | 任务处理中 |
| Success | 任务完成 |
| Failed | 任务失败 |

## 完整使用流程

```python
import requests
import time

API_KEY = "your_api_key"
BASE_URL = "https://api.minimaxi.com/v1"
HEADERS = {
    "Authorization": f"Bearer {API_KEY}",
    "Content-Type": "application/json"
}

# 1. 创建视频 Agent 任务
response = requests.post(
    f"{BASE_URL}/video_agent",
    headers=HEADERS,
    json={
        "template_id": "392753057216684038",
        "media_inputs": [
            {
                "type": "image",
                "url": "https://example.com/pet.jpg"
            }
        ]
    }
)
task_id = response.json()["task_id"]
print(f"Task created: {task_id}")

# 2. 轮询任务状态
while True:
    response = requests.get(
        f"{BASE_URL}/video_agent/{task_id}",
        headers=HEADERS
    )
    result = response.json()
    status = result["status"]
    print(f"Status: {status}")
    
    if status == "Success":
        download_url = result["download_url"]
        break
    elif status == "Failed":
        raise Exception("Video generation failed")
    
    time.sleep(10)

# 3. 下载视频
response = requests.get(download_url)
with open("output.mp4", "wb") as f:
    f.write(response.content)
print("Video downloaded!")
```

## 注意事项

1. 不同模板需要不同的输入类型，请参考模板清单
2. 视频生成为异步任务，需要轮询查询状态
3. 生成的视频下载链接有有效期限制
4. 上传的图片应符合模板要求（如人脸照片、宠物照片等）
