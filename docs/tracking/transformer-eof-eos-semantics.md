# Bug: Transformer EOF vs EoS 语义不统一

## 优先级

P2 — 影响 pipeline 边界处理的正确性

## 问题

genx.Stream 有两种"结束"信号，但各 transformer 处理不一致：

### 两种结束信号

1. **io.EOF** — input.Next() 返回 EOF，表示 Stream 物理结束
2. **EoS marker** — MessageChunk.Ctrl.EndOfStream=true，表示逻辑子流边界

### 当前行为（不统一）

| Transformer | EOF 时行为 | EoS 时行为 |
|-------------|-----------|-----------|
| mp3_to_ogg | flush 残余 → return | 转换残余 → emit OGG EoS |
| doubao_asr | finishSession → return | finishSession → emit Text EoS |
| voiceprint | processSegment → return（label 丢失）| processSegment → emit PCM EoS |
| minimax_tts | return | emit Audio EoS |

### 核心问题

1. EOF 时有些 transformer flush 了数据但没 emit，最后一段数据的结果丢失
2. 没有明确文档说明 EOF vs EoS 的语义差异
3. EOF 时是否应该自动 emit 一个 EoS marker？没有统一约定

## 建议

1. 在 genx/doc.go 或 transformer.go 中明确文档化 EOF vs EoS 语义
2. 统一约定：EOF 时 transformer 是否应该 emit 最终 EoS marker
3. 考虑在 genx 层面提供 helper：`handleEOFAndEoS(input, output, processFunc)`

## 发现者

cursor[bot] review on PR #76, 分析后确认是系统性问题
