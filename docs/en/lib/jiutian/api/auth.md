# 身份验证 (Authentication)

API 使用 API 密钥进行身份验证，所有 API 请求 Header 都应包括以下格式的 Authorization HTTP 标题：

```
Authorization: Bearer {Token}
```

## 请求头参数

请求头包含必传的大模型 token 外，需要包含设备信息，用作设备调用统计：

| 名称 | 描述 | 是否必传 |
| --- | --- | --- |
| Authorization | Bearer {Token} | 是 |
| X-AI-IP | 设备 IP 地址 | 是 |
| X-AI-VID | 产品 ID（产品注册获取） | 是 |
| X-AI-UID | 设备 ID（设备注册下发） | 是 |

## 参数说明

- **X-AI-VID**: 产品 ID (product_id)，在平台创建产品信息时生成
- **X-AI-UID**: 设备 ID (device_id)，在 AI 设备接入纳管，调用获取设备信息接口获得（请查阅终端智能体服务管理平台设备接入协议文档）
