# 九天大模型 API 文档

终端智能体服务管理平台 AI 大模型云云对接文档

## 文档索引

| 文档 | 说明 |
| --- | --- |
| [**tutorial.md**](tutorial.md) | **🚀 快速入门教程（推荐先看）** |
| [concepts.md](concepts.md) | 关键词说明（文本生成模型、助手、令牌） |
| [auth.md](auth.md) | 身份验证说明 |
| [chat.md](chat.md) | Chat Completions API |
| [device.md](device.md) | 设备接入协议（获取设备信息、心跳上报） |
| [faq.md](faq.md) | 常见问题 Q&A |

## 文档变更记录

| 日期 | 版本 | 操作内容 | 操作人 |
| --- | --- | --- | --- |
| 25.02.28 | V1.0 | 定义AI硬件厂商中控平台对接终端智能体服务管理平台大模型服务的接口协议 | 邹益强 |
| 25.04.30 | v1.0.1 | 调整文档格式 | 邹益强 |
| 25.11.14 | v1.0.2 | 增加申请邮件说明 | 邹益强 |
| 25.12.29 | v1.0.3 | 增加非生成式ai接入说明 | 邹益强 |

## AI 服务接入流程

1. 请提供厂商服务器IP开通访问白名单，以及向纳管平台申请的产品id（productId）来申请AI token
2. 邮件发送至：
   - zouyiqiang_fx@cmdc.chinamobile.com
   - zhucaiwen_fx@cmdc.chinamobile.com
   - 抄送：zhengzhongwei_fx@cmdc.chinamobile.com
3. 白名单开通后，厂商服务器就可以使用 AI TOKEN 调用九天大模型接口

## 环境配置

| 环境 | 地址 |
| --- | --- |
| 测试环境 | https://z5f3vhk2.cxzfdm.com:30101 |
| 生产环境 | https://ivs.chinamobiledevice.com:30100 |

**测试 Token**: `sk-Y73NAU0tArvGRlpUE9060529470b42Ac8bA34d40F48b0564`

**系统提示词**: 您好，我是中国移动的智能助理灵犀。如果您询问我的身份，我会回答："您好，我是中国移动智能助理灵犀"。

**模型上下文长度**: 8K
