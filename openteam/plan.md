# Plan：Rust GenX Realtime Transformers 对齐（Doubao/DashScope）

## 关联设计
- `openteam/design_proposal.md`

## 执行计划（Bazel First）
1. **基线梳理与契约冻结（0.5h）**
   - 对齐 Go 侧 realtime transformer 行为契约（输入/输出事件、EOF/EoS、错误传播、close 语义）。
   - 明确 Rust 侧 `Transformer` / `Stream` 映射点，补齐事件映射表。

2. **实现 Doubao Realtime Transformer（2h）**
   - 新增 `rust/genx/src/transformers/doubao_realtime.rs`。
   - 完成会话初始化、双向收发任务、上游关闭后的有序收敛。
   - 覆盖错误分类：初始化失败、运行时错误、连接中断。

3. **实现 DashScope Realtime Transformer（2h）**
   - 新增 `rust/genx/src/transformers/dashscope_realtime.rs`。
   - 与 Doubao 复用统一事件输出语义，保证上层无需按 provider 分叉。

4. **接入 mux 与配置注册（1h）**
   - 在 `transformers/mod.rs` 暴露实现并注册到 `TransformerMux`。
   - 校验 pattern 命名与 `testdata/cmd/apply/genx-realtime.yaml` 一致。

5. **测试与验证（2h）**
   - 单测：mock websocket 事件序列，覆盖正常路径/中断/重连失败/错误映射。
   - 集成：对齐 Go 最小契约（开始、输入、输出、结束）。
   - 运行 Bazel：`bazel test //rust/genx/...`（必要时补充 e2e target）。

## 风险与应对
- **风险**：provider 事件模型差异导致统一层语义漂移。
  - **应对**：先定义统一事件表，按表实现并在测试中逐项断言。
- **风险**：断线重连策略不一致。
  - **应对**：在实现前明确可重试错误集合、退避策略与最大重试次数。

## 交付物
- `doubao_realtime.rs` / `dashscope_realtime.rs`
- `transformers/mod.rs` 与相关注册改动
- 单测与集成测试用例
- 更新后的 `openteam/task.md` / `openteam/worklog.md`

## 预计耗时
- 总计：7.5h
