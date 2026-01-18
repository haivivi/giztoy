// Package doubaospeech 提供豆包语音 API 的 Go 实现
//
// 本包实现了 doubao_speech_interface 中定义的接口，支持以下功能：
//
//   - TTS (Text-to-Speech): 语音合成，支持同步、流式、异步多种模式
//   - ASR (Automatic Speech Recognition): 语音识别，支持一句话识别和流式识别
//   - Voice Clone: 声音复刻，支持创建自定义音色
//   - Realtime: 端到端实时语音对话
//   - Meeting: 会议转写和纪要生成
//   - Podcast: 多人播客音频合成
//   - Translation: 同声传译
//   - Media: 音视频字幕提取
//   - Console: 控制台管理（音色、API Key、服务状态等）
//
// # 快速开始
//
// 创建客户端：
//
//	client := doubaospeech.NewClient("your_app_id",
//	    doubaospeech.WithBearerToken("your_token"),
//	    doubaospeech.WithCluster("volcano_tts"),
//	)
//
// 同步语音合成：
//
//	resp, err := client.TTS.Synthesize(ctx, &doubao_speech_interface.TTSRequest{
//	    Text:      "你好，世界！",
//	    VoiceType: "zh_female_cancan",
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//	// resp.Audio 包含音频数据
//
// 流式语音合成：
//
//	for chunk, err := range client.TTS.SynthesizeStream(ctx, req) {
//	    if err != nil {
//	        log.Fatal(err)
//	    }
//	    // 处理 chunk.Audio
//	}
//
// 流式语音识别：
//
//	session, err := client.ASR.OpenStreamSession(ctx, config)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer session.Close()
//
//	// 发送音频
//	session.SendAudio(ctx, audioData, false)
//	session.SendAudio(ctx, lastData, true)
//
//	// 接收结果
//	for chunk, err := range session.Recv() {
//	    if err != nil {
//	        break
//	    }
//	    fmt.Println(chunk.Text)
//	}
//
// # 认证方式
//
// 支持两种认证方式：
//
// 1. Bearer Token（推荐）:
//
//	client := doubaospeech.NewClient(appID, doubaospeech.WithBearerToken(token))
//
// 2. API Key:
//
//	client := doubaospeech.NewClient(appID, doubaospeech.WithAPIKey(accessKey, appKey))
//
// # 集群选择
//
// 不同服务需要使用不同的集群：
//
//   - volcano_tts: TTS 标准版
//   - volcano_mega: TTS 大模型版
//   - volcano_icl: 声音复刻版
//   - volcengine_streaming_common: ASR 流式识别
//
// # 错误处理
//
// 所有方法返回的错误都可以转换为 *Error 类型：
//
//	if err != nil {
//	    if e, ok := doubaospeech.AsError(err); ok {
//	        if e.IsRateLimit() {
//	            // 处理限流
//	        }
//	    }
//	}
package doubaospeech
