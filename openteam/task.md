# 任务：GenX Labeler 驱动的 Recall 前置标签推断

## Design Proposal
openteam/design_proposal.md

## 任务概要
将 recall 的标签推断从 Go memory 内置规则方法迁移为“调用方前置”的 GenX Labeler 流程：调用方先调用 `labelers.Process` 选择匹配 labels，再调用 `memory.Recall`。同时移除 Go 侧 `InferLabels` 对外入口，保持 recall 核心职责聚焦在检索本身。

## 任务目标
- 在 GenX 中新增与 Segmentor 同风格的 `labelers` 模块（接口 + Mux + 全局注册）。
- 在 modelloader 中支持 `type: labeler` 配置与注册。
- 调整 recall 调用链：不再自动 infer，调用方显式决定是否先做 label 推断。
- 移除 Go `memory.InferLabels`/`recall.Index.InferLabels` 对外使用路径（按最终代码结构清理）。

## 测试用例
### Bazel 测试 Target
- `bazel test //go/pkg/genx/labelers:labelers_test`
- `bazel test //go/pkg/genx/modelloader:modelloader_test`
- `bazel test //go/pkg/memory:memory_test`
- `bazel test //go/pkg/cortex:cortex_test`

### 功能测试
| Case | 输入 | 预期输出 | 验证方式 |
|------|------|----------|----------|
| 正常路径: 调用前先 labeler 推断 | text="我昨天和小明聊恐龙" + candidates=[person:小明,person:小红,topic:恐龙] | 返回包含 person:小明/topic:恐龙 的 matches；Recall 使用这些 labels | 单测 mock labeler 输出并断言 RecallQuery.Labels |
| 正常路径: 不调用 labeler | RecallQuery{Text:"聊恐龙", Labels:[]} | recall 仅按 text 检索，不发生自动 infer | memory/cortex 集成测试断言结果稳定且无 infer 调用 |
| 边界条件: 候选集为空 | text 非空 + candidates=[] | labeler 返回空匹配；Recall labels 为空 | labelers 单测 + cortex 路径测试 |
| 错误处理: labeler pattern 未注册 | pattern="labeler/not-found" | 返回明确错误，不吞错 | labelers mux 单测 |
| 错误处理: labeler 调用失败 | labeler 返回 error | 调用方可选择失败退出（或按策略降级） | cortex 路径测试覆盖策略分支 |

### 集成测试
| Case | 场景 | 预期行为 |
|------|------|----------|
| modelloader 注册 labeler | YAML 含 `type: labeler` | 能通过 `labelers.Get(name)` 获取并执行 |
| memory/recall 新链路 | cortex 先调 labeler 再 recall | 有 labels 时图扩展生效；无 labels 时仅 text 检索 |

## 验收标准
- [ ] `genx/labelers` 模块具备 `Handle/Get/Process` 与默认 Mux
- [ ] modelloader 支持 `type: labeler` 注册
- [ ] 调用方可显式控制“是否先推断 labels”
- [ ] Go 内置规则 infer 公开路径已删除并完成引用清理
- [ ] 相关 Bazel 测试通过

## 状态
- 预估时间: 8h
- 剩余时间: 8h
