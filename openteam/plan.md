# 计划：curious-nebula / Recall Labeler 化改造

## 关联设计
- `openteam/design_proposal.md`

## 执行步骤
1. **Bazel First 盘点目标**
   - 用 Bazel 确认现有 `genx/segmentors`、`genx/modelloader`、`memory`、`cortex` 的 target 与依赖边界。

2. **新增 GenX Labeler 模块**
   - 新建 `go/pkg/genx/labelers/`：
     - `types.go`（Input/Result/Labeler）
     - `mux.go`（DefaultMux + Handle/Get/Process）
     - `genx.go`（LLM 实现）
     - `prompt.go`（候选 labels 选择提示词）
     - `*_test.go`（Mux + 解析 + 错误处理）

3. **接入 modelloader 全局注册**
   - 在 `go/pkg/genx/modelloader` 增加 `type: labeler` 解析与注册流程。
   - 对齐与 segmentor 一致的配置风格与错误信息。

4. **改造调用链为 caller-side infer**
   - 在 cortex memory recall 路径中：
     - 先准备候选 labels（来自 memory graph）
     - 调 `labelers.Process`
     - 再调用 `memory.Recall`
   - 显式保留“不调用 labeler”路径（labels 为空）。

5. **删除 Go 规则 infer 公开路径**
   - 移除 `memory.InferLabels` 及对应调用；
   - 清理 `recall.Index.InferLabels` 的外部依赖（若最终不再需要则移除实现与测试）。

6. **测试与回归**
   - 先跑模块级测试，再跑聚合测试：
     - `bazel test //go/pkg/genx/labelers:all`（按实际 target 调整）
     - `bazel test //go/pkg/genx/modelloader:all`
     - `bazel test //go/pkg/memory:all`
     - `bazel test //go/pkg/cortex:all`

## 风险与应对
- **风险：** labeler 模型输出不稳定
  - **应对：** 输出 JSON schema 严格校验 + TopK + label 必须属于 candidates。
- **风险：** 移除规则 infer 导致旧路径行为变化
  - **应对：** 在调用层显式区分“有/无 labeler”两条路径并补全回归用例。

## 交付物
- `go/pkg/genx/labelers/*`
- `go/pkg/genx/modelloader/*` 的 labeler 支持
- `go/pkg/cortex/*` recall 调用前置推断改造
- `go/pkg/memory` infer 清理改造
- 对应单测/集成测试更新

## Reviewer 要求修改

### P0: 必须修改
- [x] 提交范围包含 `openteam/` 未跟踪内容（`git status --short` 显示 `?? openteam/`），其中含 `worklog.md` 等工作日志文件，不允许进入代码 PR。
  - 建议：提交前仅 `git add` 业务代码目录（`go/pkg/...`），明确排除 `openteam/`；必要时补充 `.gitignore` 或使用精确路径提交。
  - 处理：后续提交严格使用精确路径（仅 `go/pkg/...`），不将 `openteam/` 纳入提交。

### P1: 建议修改
- [x] `test.md` 中已定义的高优先级测试场景未完整落地，当前用例存在缺口（按静态核对缺失）。
  - 缺失示例：`TestProcessEmptyText`、`TestProcessAliasMatching`、`TestProcessResultValidation`（`go/pkg/genx/labelers/labelers_test.go`）；`TestGenXParseValidJSON`（`go/pkg/genx/labelers/genx_test.go`）；`TestLabelerConfigMissingName`（`go/pkg/genx/modelloader/config_test.go`）；`TestCortexLabelerResultPassedToRecall` / `TestCortexRecallWithGraphExpansion`（`go/pkg/cortex/cortex_test.go`）。
  - 建议：至少补齐 task/test 中 P0 场景的对应断言，避免“场景写了但没测到”。
  - 处理：上述缺失用例已补齐并通过 Bazel 定向测试。

- [x] PR 元信息未提供，无法核验“英文标题 + 英文描述（含 Summary/Testing）”规范。
  - 建议：补充 PR 标题与描述文本；若不符合规范，按要求改为英文并补充 Testing 命令/结果。
  - 处理（草案）：
    - PR Title: `add genx labeler flow for caller-side memory recall`
    - PR Body:
      - Summary
        - add `genx/labelers` module with mux registration and GenX implementation for query-time label selection.
        - wire `type: labeler` in modelloader and move recall label inference to caller-side in cortex memory flow.
        - remove public `InferLabels` paths from memory/recall to keep recall focused on retrieval.
      - Testing
        - `bazel test //go/pkg/genx/labelers:labelers_test`
        - `bazel test //go/pkg/genx/modelloader:modelloader_test`
        - `bazel test //go/pkg/cortex:cortex_test`
