# 图片生成 API

> **官方文档**: https://platform.minimaxi.com/docs/api-reference/image-generation

## 概述

图片生成 API 支持基于文本描述或参考图片生成图像。

## 支持的模型

| 模型名称 | 说明 |
|---------|------|
| image-01 | 图像生成模型，画面表现细腻，支持文生图、图生图（人物主体参考） |
| image-01-live | 在 image-01 基础上额外支持多种画风设置 |

## 接口说明

图片生成包含 2 个接口：

1. **文生图** - 基于文本描述生成图像
2. **图生图** - 基于参考图片生成图像

## 文生图

### 端点

```
POST https://api.minimaxi.com/v1/image_generation
```

### 请求参数

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| model | string | 是 | - | 模型名称 |
| prompt | string | 是 | - | 图片描述文本 |
| aspect_ratio | string | 否 | `1:1` | 图片比例 |
| n | int | 否 | 1 | 生成数量 (1-9) |
| prompt_optimizer | boolean | 否 | true | 是否优化 prompt |

### aspect_ratio 可选值

| 比例 | 说明 |
|------|------|
| 1:1 | 正方形 |
| 16:9 | 横向宽屏 |
| 9:16 | 纵向竖屏 |
| 4:3 | 横向标准 |
| 3:4 | 纵向标准 |
| 3:2 | 横向 |
| 2:3 | 纵向 |
| 21:9 | 超宽屏 |
| 9:21 | 超竖屏 |

### 请求示例

```bash
curl --request POST \
  --url https://api.minimaxi.com/v1/image_generation \
  --header 'Authorization: Bearer <your_api_key>' \
  --header 'Content-Type: application/json' \
  --data '{
    "model": "image-01",
    "prompt": "一只可爱的橘猫，坐在窗台上，阳光洒在身上，高清摄影风格",
    "aspect_ratio": "1:1",
    "n": 1
  }'
```

### 响应格式

```json
{
  "data": {
    "images": [
      {
        "url": "https://cdn.minimaxi.com/image/xxx.png"
      }
    ]
  },
  "base_resp": {
    "status_code": 0,
    "status_msg": "success"
  }
}
```

## 图生图

### 端点

```
POST https://api.minimaxi.com/v1/image_generation
```

### 请求参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| model | string | 是 | 模型名称 |
| prompt | string | 是 | 图片描述文本 |
| image_prompt | string | 是 | 参考图片 URL |
| image_prompt_strength | float | 否 | 参考图片影响强度 (0-1)，默认 0.5 |
| aspect_ratio | string | 否 | 图片比例 |
| n | int | 否 | 生成数量 |

### 请求示例

```bash
curl --request POST \
  --url https://api.minimaxi.com/v1/image_generation \
  --header 'Authorization: Bearer <your_api_key>' \
  --header 'Content-Type: application/json' \
  --data '{
    "model": "image-01",
    "prompt": "将这只猫变成卡通风格",
    "image_prompt": "https://example.com/cat.jpg",
    "image_prompt_strength": 0.7,
    "aspect_ratio": "1:1"
  }'
```

## Prompt 编写指南

好的 prompt 应该包含以下元素：

### 主体描述
- 描述图片的主要对象
- 例如：`一只橘色的猫咪`、`一位穿着红色连衣裙的女性`

### 场景描述
- 描述背景和环境
- 例如：`在樱花树下`、`在现代化的办公室里`

### 风格描述
- 描述艺术风格
- 例如：`油画风格`、`赛博朋克风格`、`水彩画风格`、`高清摄影`

### 细节描述
- 描述光线、色调等细节
- 例如：`柔和的自然光`、`暖色调`、`高对比度`

### 示例 Prompt

```
一只可爱的橘猫，坐在窗台上看着窗外的雨，窗外是城市夜景，
霓虹灯的光芒透过雨滴折射，营造出温馨而略带忧郁的氛围，
高清摄影风格，柔和的室内光线，暖色调
```

## 完整使用示例

```python
import requests

API_KEY = "your_api_key"
BASE_URL = "https://api.minimaxi.com/v1"
HEADERS = {
    "Authorization": f"Bearer {API_KEY}",
    "Content-Type": "application/json"
}

# 文生图
response = requests.post(
    f"{BASE_URL}/image_generation",
    headers=HEADERS,
    json={
        "model": "image-01",
        "prompt": "一只可爱的橘猫，坐在窗台上，高清摄影风格",
        "aspect_ratio": "1:1",
        "n": 1
    }
)

result = response.json()
if result["base_resp"]["status_code"] == 0:
    image_url = result["data"]["images"][0]["url"]
    print(f"Image URL: {image_url}")
    
    # 下载图片
    img_response = requests.get(image_url)
    with open("output.png", "wb") as f:
        f.write(img_response.content)
    print("Image saved!")
else:
    print(f"Error: {result['base_resp']['status_msg']}")
```

## 注意事项

1. 生成的图片 URL 有有效期限制，请及时下载
2. `prompt_optimizer` 开启时会自动优化 prompt 以获得更好的效果
3. `image_prompt_strength` 值越高，生成的图片越接近参考图片
4. 不同比例适合不同的使用场景（如社交媒体、壁纸等）
5. 生成多张图片时，每张图片可能略有不同
