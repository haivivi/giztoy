# 视频生成 API

> **官方文档**: https://platform.minimaxi.com/docs/api-reference/video-generation

## 概述

视频生成 API 支持基于文本描述或图片生成视频，采用异步任务模式。

## 支持的模型

| 模型名称 | 说明 |
|---------|------|
| MiniMax-Hailuo-2.3 | 最新模型，支持高分辨率 |
| MiniMax-Hailuo-2.3-Fast | 快速版本 |
| MiniMax-Hailuo-02 | 支持 1080P 分辨率，10 秒视频 |
| T2V-01 | 文生视频模型 |
| T2V-01-Director | 支持镜头运动控制 |
| I2V-01 | 图生视频模型 |
| I2V-01-Director | 图生视频，支持镜头控制 |
| I2V-01-live | 图生视频，支持多种画风 |

## 接口说明

视频生成采用异步方式，包含以下接口：

1. **创建文生视频任务** - 基于文本描述生成视频
2. **创建图生视频任务** - 基于图片生成视频
3. **创建首尾帧生成视频任务** - 基于首帧和尾帧图片生成视频
4. **创建主体参考生成视频任务** - 基于主体参考图生成视频
5. **查询视频生成任务状态** - 查询任务状态
6. **生成视频下载** - 下载生成的视频

## 创建文生视频任务

### 端点

```
POST https://api.minimaxi.com/v1/video_generation
```

### 请求参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| model | string | 是 | 模型名称 |
| prompt | string | 是 | 视频描述文本 |
| duration | int | 否 | 视频时长（秒），可选 6 或 10 |
| resolution | string | 否 | 分辨率：`768P` 或 `1080P` |

### 镜头运动控制（Director 模型）

当使用 Director 模型时，prompt 支持以下镜头运动指令：

| 指令类型 | 指令 |
|---------|------|
| 推拉 (Truck) | `[Truck left]`, `[Truck right]` |
| 摇镜 (Pan) | `[Pan left]`, `[Pan right]` |
| 推进 (Push) | `[Push in]`, `[Pull out]` |
| 升降 (Pedestal) | `[Pedestal up]`, `[Pedestal down]` |
| 俯仰 (Tilt) | `[Tilt up]`, `[Tilt down]` |
| 变焦 (Zoom) | `[Zoom in]`, `[Zoom out]` |
| 抖动 (Shake) | `[Shake]` |
| 跟随 (Follow) | `[Tracking shot]` |
| 静止 (Static) | `[Static shot]` |

### 请求示例

```bash
curl --request POST \
  --url https://api.minimaxi.com/v1/video_generation \
  --header 'Authorization: Bearer <your_api_key>' \
  --header 'Content-Type: application/json' \
  --data '{
    "model": "MiniMax-Hailuo-2.3",
    "prompt": "一只可爱的猫咪在草地上奔跑，阳光明媚",
    "duration": 6,
    "resolution": "1080P"
  }'
```

### 响应格式

```json
{
  "task_id": "task_abc123",
  "base_resp": {
    "status_code": 0,
    "status_msg": "success"
  }
}
```

## 创建图生视频任务

### 端点

```
POST https://api.minimaxi.com/v1/video_generation
```

### 请求参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| model | string | 是 | 模型名称（I2V 系列） |
| prompt | string | 否 | 视频描述文本 |
| first_frame_image | string | 是 | 首帧图片 URL 或 base64 |
| duration | int | 否 | 视频时长（秒） |
| resolution | string | 否 | 分辨率 |

### 请求示例

```bash
curl --request POST \
  --url https://api.minimaxi.com/v1/video_generation \
  --header 'Authorization: Bearer <your_api_key>' \
  --header 'Content-Type: application/json' \
  --data '{
    "model": "I2V-01",
    "prompt": "猫咪开始奔跑",
    "first_frame_image": "https://example.com/cat.jpg"
  }'
```

## 创建首尾帧生成视频任务

### 端点

```
POST https://api.minimaxi.com/v1/video_generation
```

### 请求参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| model | string | 是 | 模型名称 |
| prompt | string | 否 | 视频描述文本 |
| first_frame_image | string | 是 | 首帧图片 |
| last_frame_image | string | 是 | 尾帧图片 |

## 创建主体参考生成视频任务

### 端点

```
POST https://api.minimaxi.com/v1/video_generation
```

### 请求参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| model | string | 是 | 模型名称 |
| prompt | string | 是 | 视频描述文本 |
| subject_reference | string | 是 | 主体参考图片 |

## 查询视频生成任务状态

### 端点

```
GET https://api.minimaxi.com/v1/query/video_generation
```

### 查询参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| task_id | string | 是 | 任务 ID |

### 请求示例

```bash
curl --request GET \
  --url 'https://api.minimaxi.com/v1/query/video_generation?task_id=task_abc123' \
  --header 'Authorization: Bearer <your_api_key>'
```

### 响应格式

```json
{
  "task_id": "task_abc123",
  "status": "Success",
  "file_id": "357248197701747",
  "video_width": 1366,
  "video_height": 768,
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

## 生成视频下载

### 端点

```
GET https://api.minimaxi.com/v1/files/retrieve_content
```

### 查询参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| file_id | string | 是 | 文件 ID |

### 请求示例

```bash
curl --request GET \
  --url "https://api.minimaxi.com/v1/files/retrieve_content?file_id=357248197701747" \
  --header 'Authorization: Bearer <your_api_key>' \
  --output output.mp4
```

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

# 1. 创建视频生成任务
response = requests.post(
    f"{BASE_URL}/video_generation",
    headers=HEADERS,
    json={
        "model": "MiniMax-Hailuo-2.3",
        "prompt": "一只可爱的猫咪在草地上奔跑",
        "duration": 6
    }
)
task_id = response.json()["task_id"]
print(f"Task created: {task_id}")

# 2. 轮询任务状态
while True:
    response = requests.get(
        f"{BASE_URL}/query/video_generation",
        headers=HEADERS,
        params={"task_id": task_id}
    )
    result = response.json()
    status = result["status"]
    print(f"Status: {status}")
    
    if status == "Success":
        file_id = result["file_id"]
        break
    elif status == "Failed":
        raise Exception("Video generation failed")
    
    time.sleep(10)

# 3. 下载视频
response = requests.get(
    f"{BASE_URL}/files/retrieve_content",
    headers=HEADERS,
    params={"file_id": file_id}
)
with open("output.mp4", "wb") as f:
    f.write(response.content)
print("Video downloaded!")
```

## 注意事项

1. 视频生成为异步任务，需要轮询查询状态
2. 生成的视频文件有下载有效期限制
3. 不同模型支持的分辨率和时长可能不同
4. Director 模型支持镜头运动控制指令
