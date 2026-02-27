# Review 标准：GenX Labeler 驱动的 Recall 前置标签推断

## 审查依据
- Design Proposal: `openteam/design_proposal.md`
- Task: `openteam/task.md`
- Plan: `openteam/plan.md`
- Test: `openteam/test.md`（已提供详细场景，按其中 P0/P1 与 task 验收项做静态核验）

## 检查清单

### 一、功能验收（对应 task.md 验收标准）

- [x] 验收标准 1：`genx/labelers` 模块具备 `Handle/Get/Process` 与默认 Mux
  - 检查方法：
    - 检查 `go/pkg/genx/labelers/` 是否存在独立模块与核心类型（Input/Result/Labeler）。
    - 检查默认 Mux 实现与全局函数是否齐备：`DefaultMux`、`Handle`、`Get`、`Process`。
    - 检查错误分支：未注册 pattern 时是否返回明确错误（不可吞错、不可 panic）。
  - 通过标准：
    - API 形态与设计文档一致；
    - 能从代码静态确认注册、路由、调用路径完整；
    - 错误信息可定位（包含 pattern 或明确原因）。

- [x] 验收标准 2：modelloader 支持 `type: labeler` 注册
  - 检查方法：
    - 检查 `go/pkg/genx/modelloader` 是否新增 `labeler` 类型解析与注册分支。
    - 检查注册后可通过 `labelers.Get(name)` 获取。
    - 比对 segmentor 的风格一致性（配置字段、错误文案、注册时机）。
  - 通过标准：
    - 配置中 `type: labeler` 能走到 labelers 注册逻辑；
    - 未知类型/缺失字段会报错而不是静默忽略；
    - 与现有 modelloader 架构兼容，不破坏其他类型加载。

- [x] 验收标准 3：调用方可显式控制“是否先推断 labels”
  - 检查方法：
    - 检查 cortex recall 路径是否实现“caller-side infer”：先准备 candidates -> 调 `labelers.Process` -> 调 `memory.Recall`。
    - 检查保留“不调用 labeler”路径（Labels 为空时直接 recall）。
    - 检查 labeler 失败分支策略是否显式（失败退出或降级）且与代码注释/测试一致。
  - 通过标准：
    - 代码中不存在 recall 内部自动 infer 行为；
    - 两条路径（有 labeler / 无 labeler）都可从调用逻辑静态证明成立；
    - 失败分支不含 TODO/假实现。

- [x] 验收标准 4：Go 内置规则 infer 公开路径已删除并完成引用清理
  - 检查方法：
    - 检查 `memory.InferLabels` 是否已删除或不再对外可用。
    - 全局搜索 `InferLabels`（含 `recall.Index.InferLabels`）确认无外部调用残留。
    - 检查相关测试/文档引用是否同步更新。
  - 通过标准：
    - 对外公开 API 不再暴露规则 infer；
    - 无死代码、无悬空调用、无编译期残留接口引用；
    - 行为边界清晰：recall 仅消费传入 labels。

- [x] 验收标准 5：相关 Bazel 测试覆盖到位（静态核验）
  - 检查方法：
    - 审查是否新增/更新以下目标对应测试文件与断言逻辑：
      - `//go/pkg/genx/labelers:labelers_test`
      - `//go/pkg/genx/modelloader:modelloader_test`
      - `//go/pkg/memory:memory_test`
      - `//go/pkg/cortex:cortex_test`
    - 检查测试内容是否覆盖 task.md 中 5 个功能场景（正常、边界、未注册、调用失败）。
  - 通过标准：
    - 每个目标都有实质性断言，不是空测试；
    - 场景覆盖与 task.md 对齐；
    - 不依赖人工口头说明“测过了”。

### 二、设计一致性与架构边界
- [x] Labeler 与 Segmentor 职责分离明确（读路径 vs 写路径）
- [x] Recall 核心职责保持单一（图扩展 + 段检索），无自动推断内嵌
- [x] 不引入双路径长期并存（规则 infer 与 LLM infer 并行）
- [x] Go/Rust 语义对齐不被破坏（至少接口语义一致）

检查方法：对照 design_proposal 中“职责边界”和“Rust 对齐”章节，确认改动位置与行为一致。  
通过标准：无跨层耦合、无“万能模块化”回退。

### 三、代码质量
- [x] 错误处理完整：所有外部依赖调用（labeler、loader、recall）都有明确 error 分支
- [x] 边界处理完整：空 candidates、空 text、TopK 非法值有合理行为
- [x] 代码风格一致：命名、包结构、导出级别与既有模块一致
- [x] 禁止占位实现：不得出现 `TODO: Implement`、假数据返回、只注册不落地
- [x] 禁止引入 `Atomic(i128/u128)`（若涉及 Zig 代码，必须卡死）

### 四、变更洁净度与合规
- [ ] 检查不应提交文件：`bin/`、构建产物、缓存、临时文件、敏感信息、非必要 IDE 配置
  - 检查方法：基于 `git diff --name-only` 的文件列表静态审查
  - 通过标准：无违规文件进入提交范围
- [ ] PR 标题与描述质量（英文）
  - 标题：英文、动词开头、描述变更目的
  - 描述：包含 `Summary`（1-3 条价值要点）与 `Testing`（命令/结果或未执行原因）
  - 不通过处理：在 `plan.md` 增加 Reviewer 要求修改

## 审查结果
- 总体状态：Needs Fixes
- 发现问题数：3
- 最后审查时间：2026-02-27（二轮）

## 本轮审查结论（静态审查，二轮）

### 通过项
- `go/pkg/genx/labelers` 已按设计落地：接口、Mux、默认注册入口与 GenX 实现齐全。
- `modelloader` 已支持 `type: labeler` 分支注册，并可通过 `labelers.Get` 获取。
- `cortex` 已改为 caller-side infer（可选 `labeler`），`memory`/`recall` 不再内置自动 infer。
- `memory.InferLabels` 及 `go/pkg/recall/infer.go` 路径已移除，`go/pkg` 范围未发现残留调用。
- 上一轮指出的测试缺口已补齐（`TestProcessEmptyText`、`TestProcessAliasMatching`、`TestProcessResultValidation`、`TestGenXParseValidJSON`、`TestLabelerConfigMissingName`、`TestCortexLabelerResultPassedToRecall`、`TestCortexRecallWithGraphExpansion`）。

### 未通过项（需修复）
1. **P0 - 无关高风险改动混入**：`MODULE.bazel` 将 `crate.from_cargo.manifests` 从多清单回退为仅 `//rust:Cargo.toml`，并导致 `MODULE.bazel.lock` 大规模漂移（500+ 行）。这不是本任务范围，且风险极高。
2. **P0 - 提交洁净度违规**：最新提交包含 `openteam/*`（`worklog.md`/`review.md`/`plan.md` 等工作文档），按规范不得进入业务 PR。
3. **P1 - PR 规范待核验**：虽在 plan 中给了草案，但未见实际 PR 页面元信息，无法最终确认英文标题与 `Summary/Testing` 是否达标。
