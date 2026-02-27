# 任务：Rust GenX Realtime Transformers 对齐（Doubao/DashScope）

## Design Proposal
openteam/design_proposal.md

## 任务概要
在 Rust `genx` 中补齐 Doubao 与 DashScope 的 realtime transformer，实现与 Go 侧一致的会话生命周期、事件映射、错误传播与关闭语义。

## 任务目标
- 新增 `doubao_realtime` 与 `dashscope_realtime` 实现并接入 `TransformerMux`。
- 统一输出事件语义，避免上层按 provider 分叉处理。
- 提供可执行测试覆盖正常路径、边界条件与错误处理。

## 测试用例
### Bazel 测试 Target
- `bazel test //rust/genx/...`

### 功能测试
| Case | 输入 | 预期输出 | 验证方式 |
|------|------|----------|----------|
| 正常路径: 全链路收发 | 连续音频 chunk + EOS | 输出转写文本、模型文本、音频回包，最终正常结束 | mock websocket 序列断言输出顺序与字段 |
| 边界条件: 空输入流 | 直接 EOF | 输出流直接结束，无 panic | 单测断言 `next()` 返回 `Ok(None)` |
| 错误处理: 初始化失败 | 握手返回错误 | `transform()` 返回错误，不启动后台任务 | 单测断言初始化错误类型 |
| 错误处理: 运行时断连 | 会话中 websocket 中断 | 按策略重试或失败，错误可观测 | 单测断言重试次数与最终状态事件 |

### 集成测试
| Case | 场景 | 预期行为 |
|------|------|----------|
| Realtime 最小契约 | start -> input -> output -> end | 行为与 Go 契约一致，事件序列可复现 |

## 验收标准
- [ ] 所有测试通过 `bazel test` 运行
- [ ] Doubao/DashScope realtime transformer 均可被 mux 路由并正常工作
- [ ] EOF/EoS、错误传播、close 语义与设计文档一致

## 状态
- 预估时间: 8h
- 剩余时间: 8h
