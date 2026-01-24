# 认证与鉴权

## 原始文档

| 文档 | 链接 |
|------|------|
| 获取 API Key | https://help.aliyun.com/zh/model-studio/get-api-key |
| 开通服务 | https://help.aliyun.com/zh/dashscope/opening-service |
| 权限管理 | https://help.aliyun.com/zh/model-studio/permission-management-overview |
| 工作空间 | https://help.aliyun.com/zh/model-studio/use-workspace |

> 如果本文档信息不完整，请访问上述链接获取最新内容。

---

## 1. 开通服务

使用百炼（Model Studio / DashScope）前，需要先开通服务：

1. 以主账号登录 [百炼控制台](https://bailian.console.aliyun.com/)
2. 选择地域（北京 / 新加坡 / 弗吉尼亚）
3. 阅读并同意服务协议
4. 如提示"尚未实名认证"，需先完成实名认证

---

## 2. 创建 API Key

API Key 不会自动生成，需要手动创建：

### 创建流程

1. 进入"密钥管理 / API-Key"页面
2. 点击"新建 API Key"
3. 选择所属账户（主账号或 RAM 子账号）
4. 选择业务空间（Workspace）
5. 填写描述，确认创建
6. 复制并保存 API Key

### 权限说明

| 账户类型 | 可见范围 |
|---------|---------|
| 主账号 | 可查看所有 API Key |
| 子账号 | 仅可查看自己创建的 Key |

### API Key 有效期

- 默认**永久有效**，直到手动删除或用户被移除
- 可生成**临时 API Key**，有效期通常为 60 秒

### 数量限制

- 每个 Workspace 最多 **20 个 API Key**

---

## 3. 使用 API Key

### HTTP 请求

在请求头中添加：

```
Authorization: Bearer <API_KEY>
```

### 环境变量（推荐）

```bash
export DASHSCOPE_API_KEY="sk-xxxxxxxxxxxxxxxx"
```

代码中读取：

```go
apiKey := os.Getenv("DASHSCOPE_API_KEY")
```

### WebSocket 连接

```go
header := http.Header{}
header.Set("Authorization", "Bearer "+apiKey)

conn, _, err := websocket.DefaultDialer.Dial(endpoint, header)
```

---

## 4. 服务端点 (Endpoints)

### HTTP API

| 地域 | 端点 |
|------|------|
| 北京（中国大陆） | `https://dashscope.aliyuncs.com/compatible-mode/v1` |
| 新加坡（国际） | `https://dashscope-intl.aliyuncs.com/compatible-mode/v1` |
| 弗吉尼亚（美国） | `https://dashscope-us.aliyuncs.com/compatible-mode/v1` |

### WebSocket API

| 地域 | 端点 |
|------|------|
| 北京 | `wss://dashscope.aliyuncs.com/api-ws/v1/realtime` |
| 新加坡 | `wss://dashscope-intl.aliyuncs.com/api-ws/v1/realtime` |

### 应用 API

```
POST https://dashscope.aliyuncs.com/api/v1/apps/{APP_ID}/completion
```

---

## 5. 工作空间 (Workspace)

工作空间是资源隔离、权限控制、成本分摊的基本单位。

### 默认工作空间 vs 子工作空间

| 类型 | API Key 权限 |
|------|-------------|
| 默认工作空间 | 可访问所有模型和应用 |
| 子工作空间 | 仅可访问被授权的模型和应用 |

### 模型授权

子工作空间需要主账号显式授权才能调用模型：

1. 主账号登录控制台
2. 进入"模型授权"页面
3. 选择子工作空间
4. 勾选要授权的模型
5. 保存

---

## 6. 角色与权限

### 角色类型

| 角色 | 范围 | 权限 |
|------|------|------|
| 超级管理员 | 全局 | 管理用户、工作空间、模型、限流、所有 API Key |
| 工作空间管理员 | 特定空间 | 管理空间内用户、资源，不能管理全局资源 |
| 普通用户 | 特定空间 | 使用模型/应用，不能管理用户/Key |
| 访客 | 特定空间 | 只读访问 |

### 常用权限策略

| 策略 | 说明 |
|------|------|
| `AliyunBailianFullAccess` | 百炼服务完全访问权限 |
| `AliyunBailianDataFullAccess` | 数据/知识库完全访问权限 |
| `AliyunBailianReadOnlyAccess` | 只读访问权限 |

---

## 7. 错误处理

### 常见认证错误

| 错误 | 原因 | 解决方案 |
|------|------|---------|
| 401 Unauthorized | API Key 无效或过期 | 检查 API Key 是否正确 |
| 403 Forbidden | 无权限访问该模型/应用 | 检查工作空间授权 |
| "workspace 不存在" | 工作空间 ID 错误或无权限 | 确认 workspace_id |
| "model unauthorized" | 模型未授权给当前工作空间 | 联系主账号授权 |

---

## 8. 安全最佳实践

1. **不要硬编码** API Key，使用环境变量
2. **最小权限原则** - 为 API 调用创建专用 RAM 用户
3. **定期轮换** - 定期更换 API Key
4. **监控使用** - 关注异常调用
5. **使用临时 Key** - 对外暴露时使用临时 API Key
