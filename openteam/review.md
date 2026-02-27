# Review 标准：Rust GenX Realtime Transformers 对齐（Doubao/DashScope）

## 审查依据
- Design Proposal: `openteam/design_proposal.md`
- Task: `openteam/task.md`
- Plan: `openteam/plan.md`
- Test Spec: `openteam/test.md`

## 审查范围
- Rust GenX realtime transformers 相关改动（含 mux 接入、事件映射、错误与关闭语义、测试代码）
- 与本任务相关的文档与配置改动

## 检查清单

### 一、功能验收（与任务目标/验收标准逐条对齐）

- [ ] 验收标准 A：新增 Doubao/DashScope realtime transformer 并接入 `TransformerMux`（未通过）
  - 检查方法：
    - 静态检查是否新增：
      - `rust/genx/src/transformers/doubao_realtime.rs`
      - `rust/genx/src/transformers/dashscope_realtime.rs`
    - 检查 `transformers/mod.rs` 或等价注册点是否完成暴露与 mux 路由。
    - 检查 pattern/命名与计划文档要求（如 `testdata/cmd/apply/genx-realtime.yaml`）是否一致。
  - 通过标准：
    - 两个 provider 均可被 mux 识别并路由；不存在仅实现未注册、仅注册空实现、或 TODO 占位。

- [ ] 验收标准 B：统一输出事件语义，避免上层按 provider 分叉（未通过）
  - 检查方法：
    - 对比 Doubao 与 DashScope transformer 输出事件类型/字段/状态语义。
    - 检查是否通过统一模型（转写文本、模型文本、音频回包、状态事件）输出，而非 provider 私有泄漏。
    - 检查上层调用点是否无需 provider 特判分支。
  - 通过标准：
    - 事件语义一致、字段可对齐；上层无新增 provider-specific 分叉逻辑。

- [ ] 验收标准 C：会话生命周期、EOF/EoS、错误传播、close 语义与设计一致（部分通过，存在阻断问题）
  - 检查方法：
    - 初始化阶段：握手/session 创建失败是否直接返回错误且不启动后台任务。
    - 运行阶段：双向收发任务是否可有序收敛；上游 close 后是否按序停止。
    - EOF/空输入：是否安全结束并返回 `Ok(None)`（或语义等价行为），无 panic。
    - 断连/运行时错误：是否按约定重试或失败，且错误可观测（状态事件/错误返回）。
  - 通过标准：
    - 上述语义均在实现与测试中可追踪、可证明；无吞错、挂死、资源泄漏、未关闭通道。

- [ ] 验收标准 D：测试覆盖满足任务要求（未通过）
  - 检查方法：
    - 检查 `//rust/genx/...` 对应测试是否新增/更新，覆盖：
      1) 正常全链路收发；
      2) 空输入流；
      3) 初始化失败；
      4) 运行时断连（含重试次数与最终状态）；
      5) 最小契约 start->input->output->end。
    - 检查开发者是否在变更说明中给出 Bazel 测试执行证据（命令/结果）。
  - 通过标准：
    - 用例覆盖完整且断言关键行为（顺序、字段、状态）；无“只测 happy path”。

### 二、代码质量与架构约束

- [x] 代码风格与可维护性（通过）
  - 检查方法：命名、模块边界、重复代码、注释质量、可读性。
  - 通过标准：命名清晰、无明显复制粘贴异味、无难以维护的嵌套分支。

- [x] 错误处理完整性（基本通过）
  - 检查方法：检查 `Result` 传播、错误映射、上下文信息保留、日志/状态可观测性。
  - 通过标准：错误分类明确，不丢上下文；禁止 silently ignore。

- [ ] 并发与资源安全（未通过，需修复 stream_id 生命周期管理）
  - 检查方法：检查任务生命周期、通道关闭、取消信号、join/abort 策略。
  - 通过标准：不会出现悬挂任务、竞争条件导致的重复结束或双重释放语义问题。

- [ ] 性能与稳定性（未通过，DashScope 分片策略不满足 chunk 约束）
  - 检查方法：检查音频 chunk 处理、缓冲策略、重试退避与上限是否合理。
  - 通过标准：无明显无界缓冲/忙等循环；重试有上限且可配置或可说明。

- [x] 严禁占位实现（通过）
  - 检查方法：扫描 TODO、`unimplemented!`、返回假数据、空分支注册。
  - 通过标准：对外暴露能力必须端到端可用；未完成能力不得注册暴露。

- [ ] 项目约束合规（未通过，存在不应提交文件/本地环境污染）
  - 检查方法：确认不引入与任务无关改动；遵守 Bazel First 目标路径与结构约束。
  - 通过标准：改动聚焦 realtime transformer；无无关重构污染。

### 三、提交与 PR 规范检查

- [ ] 不应提交文件检查（基于 `git diff --name-only`）（未通过）
  - 检查方法：排查 `bin/`、编译产物、临时文件、日志、IDE 配置、敏感信息等。
  - 通过标准：不存在不应提交内容；如出现必须要求移除。

- [ ] PR 标题与描述质量（未通过）
  - 检查方法：核对 PR title/description 是否英文，且 description 含 `Summary` 与 `Testing`。
  - 通过标准：
    - 标题：英文、简洁、动词开头、说明目的；
    - 描述：英文，`Summary` 1-3 条价值点，`Testing` 明确命令/结果或未执行原因。

## 判定规则
- **Pass**：所有 P0 检查项通过，无阻断问题。
- **Needs Fixes**：存在任一阻断问题（功能缺失、语义不一致、关键测试缺失、违规提交、PR 规范不合格）。
- **Reject**：存在严重设计偏离、明显敷衍（占位实现/假实现）、或引入高风险缺陷且缺少修复计划。

## 审查结果
- 总体状态：Needs Fixes
- 发现问题数：2（P0=1，P1=1）
- 最后审查时间：2026-02-27

## 三轮结论（增量审查）
- 已确认修复：
  1. DashScope `stream_id` 同轮一致性问题已修正，并补充了对应断言测试。
  2. DashScope remainder flush 时机已修正（仅 EOS/close flush），并补充分片测试。
  3. 已新增 realtime 路由 helper 与 mux 路由测试。
  4. 已新增 Bazel `realtime_contract_test` 目标。
  5. Rust e2e 占位代码与错误 BUILD 目标已从本轮变更中移除，不再阻断。
  6. `MODULE.bazel.lock` 本地环境污染项已清理，本轮 lock 仅包含依赖增量。

- 仍阻断的问题：
  1. PR #93 描述仍与当前代码事实不符（仍写无代码改动）。

- 非阻断但需解释：
  - `.gitignore` 新增 `openteam/` 的仓库级策略变更需给出明确理由（否则建议回退并改用本地 exclude）。
