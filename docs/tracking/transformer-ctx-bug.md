# Bug: Transformer transformLoop 使用创建时的 ctx 而非运行时 ctx

## 优先级

P1 — 影响所有 transformer 的生命周期控制

## 问题

所有 transformer 的 `Transform(ctx, pattern, input)` 方法将 `ctx` 传给后台 `transformLoop` goroutine：

```go
func (t *Voiceprint) Transform(ctx context.Context, _ string, input genx.Stream) (genx.Stream, error) {
    output := newBufferStream(100)
    go t.transformLoop(ctx, input, output)  // ← 这个 ctx 是创建 stream 时的
    return output, nil
}
```

问题在于：`ctx` 是 **创建 Stream 时的 context**，不是 **实际消费 Stream 时的 context**。

典型场景：
1. 初始化阶段用一个短超时 ctx 创建 pipeline
2. 运行阶段用一个长生命周期 ctx 消费 pipeline
3. 初始化 ctx 超时后，transformer goroutine 会被意外取消

## 影响范围

所有 transformer 都有这个问题：
- `go/pkg/genx/transformers/voiceprint.go`
- `go/pkg/genx/transformers/doubao_asr_sauc.go`
- `go/pkg/genx/transformers/doubao_tts_seed_v2.go`
- `go/pkg/genx/transformers/doubao_tts_icl_v2.go`
- `go/pkg/genx/transformers/doubao_realtime.go`
- `go/pkg/genx/transformers/dashscope_realtime.go`
- `go/pkg/genx/transformers/minimax_tts.go`
- `go/pkg/genx/transformers/codec_mp3_to_ogg.go`

## 修复方案

Transformer.Transform() 的 ctx 应该只用于**初始化**（如建立 WebSocket 连接）。
transformLoop 的生命周期应该由 **Stream 本身** 控制（input 关闭 → goroutine 退出）。

选项：
1. transformLoop 不用 ctx，完全靠 input.Next() 返回 EOF/error 退出
2. Transform 接口不变，但内部用 context.Background() 替代传入的 ctx
3. 让 caller 通过 input.CloseWithError() 控制取消

## 发现者

cursor[bot] review on PR #76, 独立验证后确认
