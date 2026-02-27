# Rust GenX Realtime Transformers 对齐（Doubao/DashScope）

## 背景
Go 侧已提供 `doubao_realtime` 与 `dashscope_realtime` transformer，支持双向流式会话。Rust 侧目前缺少对应 realtime transformer，导致 Rust 无法承接低延迟语音会话链路。

## 目标
- 在 Rust `genx` 中补齐 Realtime transformer（Doubao + DashScope）。
- 对齐 Go 侧会话生命周期、音频/文本事件映射、异常与断线恢复策略。
- 产出可用于 e2e 的统一接口，避免上层按 provider 分叉处理。

## 设计
1. 新增模块：
   - `transformers/doubao_realtime.rs`
   - `transformers/dashscope_realtime.rs`
2. 统一事件模型：
   - 输入：音频 chunk / 控制 chunk。
   - 输出：转写文本、模型文本、音频回包、状态事件。
3. 连接与生命周期：
   - 初始化阶段完成握手和 session 创建。
   - 运行时通过内部任务驱动收发；上游 close 时有序收敛。
4. 测试：
   - 单测：模拟 websocket 事件序列，覆盖中断、重连、错误映射。
   - 集成：对齐 Go 的最小行为契约（开始、输入、输出、结束）。

## 其他（可选）
- 并行边界：仅 realtime transformers，不改 Agent 与 Tool 系统。
- 建议 owner：Rust Realtime/网络方向。
