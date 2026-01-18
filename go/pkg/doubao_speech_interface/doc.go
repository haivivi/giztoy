// Package doubao_speech_interface 定义了豆包语音 API 的 Go 客户端接口。
//
// 豆包语音 API 提供以下能力：
//   - TTS (Text-to-Speech): 语音合成，支持同步、流式、异步模式
//   - ASR (Automatic Speech Recognition): 语音识别，支持一句话识别、流式识别、文件识别
//   - Voice Clone: 声音复刻，训练自定义音色
//   - Realtime: 端到端实时语音对话
//   - Meeting: 会议场景转写
//   - Podcast: 播客场景合成
//   - Translation: 同声传译
//   - Media: 音视频处理（字幕提取等）
//
// 基本用法：
//
//	client := doubao_speech_interface.NewClient("your-app-id",
//	    doubao_speech_interface.WithBearerToken("your-token"),
//	    doubao_speech_interface.WithCluster("volcano_tts"),
//	)
//
//	// TTS 合成
//	resp, err := client.TTS.Synthesize(ctx, &doubao_speech_interface.TTSRequest{
//	    Text:      "你好，世界",
//	    VoiceType: "zh_female_shuangkuaisisi_moon_bigtts",
//	    Encoding:  doubao_speech_interface.EncodingMP3,
//	})
//
// 认证方式：
//   - Bearer Token: 通过 WithBearerToken 设置
//   - API Key: 通过 WithAPIKey 设置 accessKey 和 appKey
//
// 官方文档：https://www.volcengine.com/docs/6561/79817
package doubao_speech_interface
