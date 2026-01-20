// Package doubaospeech 提供豆包语音 API 的 Go 实现
//
// 本包提供两个独立的客户端：
//
// 1. Client - 语音 API 客户端（使用 Bearer Token 或 API Key 认证）
//   - TTS (Text-to-Speech): 语音合成，支持同步、流式、异步多种模式
//   - ASR (Automatic Speech Recognition): 语音识别，支持一句话识别和流式识别
//   - Voice Clone: 声音复刻，支持创建自定义音色
//   - Realtime: 端到端实时语音对话
//   - Meeting: 会议转写和纪要生成
//   - Podcast: 多人播客音频合成
//   - Translation: 同声传译
//   - Media: 音视频字幕提取
//
// 2. Console - 控制台 API 客户端（使用 AK/SK 签名认证）
//   - ListTimbres: 列出可用音色
//   - ListSpeakers: 列出可用说话人
//   - ListVoiceCloneStatus: 查询声音复刻训练状态
//
// # 快速开始
//
// 创建语音 API 客户端：
//
//	client := doubaospeech.NewClient("your_app_id",
//	    doubaospeech.WithBearerToken("your_token"),
//	    doubaospeech.WithCluster("volcano_tts"),
//	)
//
// 创建控制台 API 客户端：
//
//	console := doubaospeech.NewConsole("your_access_key", "your_secret_key")
//
// 同步语音合成：
//
//	resp, err := client.TTS.Synthesize(ctx, &doubaospeech.TTSRequest{
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
// Client (语音 API) 支持三种认证方式：
//
// 1. API Key（推荐，最简单）:
//
//	client := doubaospeech.NewClient(appID, doubaospeech.WithAPIKey(apiKey))
//	// 获取地址: https://console.volcengine.com/speech/new/setting/apikeys
//
// 2. Bearer Token:
//
//	client := doubaospeech.NewClient(appID, doubaospeech.WithBearerToken(token))
//
// 3. Realtime V3 API Key（用于实时对话）:
//
//	client := doubaospeech.NewClient(appID, doubaospeech.WithRealtimeAPIKey(accessKey, appKey))
//
// Console (控制台 API) 需要 AK/SK 签名:
//
//	console := doubaospeech.NewConsole(accessKey, secretKey)
//	// 获取地址: https://console.volcengine.com/iam/keymanage/
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
