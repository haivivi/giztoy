package cortex

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"time"

	ds "github.com/haivivi/giztoy/go/pkg/doubaospeech"
)

func init() {
	RegisterRunHandler("doubaospeech/tts/v1/synthesize", runDoubaoTTSV1Synthesize)
	RegisterRunHandler("doubaospeech/tts/v1/stream", runDoubaoTTSV1Stream)
	RegisterRunHandler("doubaospeech/tts/v2/stream", runDoubaoTTSV2Stream)
	RegisterRunHandler("doubaospeech/tts/v2/bidirectional", runDoubaoTTSV2Bidirectional)
	RegisterRunHandler("doubaospeech/tts/v2/async", runDoubaoTTSV2Async)
	RegisterRunHandler("doubaospeech/asr/v1/recognize", runDoubaoASRV1Recognize)
	RegisterRunHandler("doubaospeech/asr/v1/stream", runDoubaoASRV1Stream)
	RegisterRunHandler("doubaospeech/asr/v2/stream", runDoubaoASRV2Stream)
	RegisterRunHandler("doubaospeech/asr/v2/file", runDoubaoASRV2File)
	RegisterRunHandler("doubaospeech/voice/train", runDoubaoVoiceTrain)
	RegisterRunHandler("doubaospeech/realtime/connect", runDoubaoRealtimeConnect)
	RegisterRunHandler("doubaospeech/translation/stream", runDoubaoTranslationStream)
	RegisterRunHandler("doubaospeech/podcast/http/submit", runDoubaoPodcastHTTPSubmit)
	RegisterRunHandler("doubaospeech/podcast/sami", runDoubaoPodcastSAMI)
	RegisterRunHandler("doubaospeech/meeting/create", runDoubaoMeetingCreate)
	RegisterRunHandler("doubaospeech/media/subtitle", runDoubaoMediaSubtitle)
}

func newDoubaoClient(cred map[string]any) (*ds.Client, error) {
	appID, _ := cred["app_id"].(string)
	if appID == "" {
		return nil, fmt.Errorf("doubaospeech cred missing app_id")
	}
	var opts []ds.Option
	token, _ := cred["token"].(string)
	if token != "" {
		opts = append(opts, ds.WithBearerToken(token))
	}
	appKey, _ := cred["app_key"].(string)
	if appKey == "" {
		appKey = appID
	}
	if token != "" {
		opts = append(opts, ds.WithV2APIKey(token, appKey))
	}
	if baseURL, _ := cred["base_url"].(string); baseURL != "" {
		opts = append(opts, ds.WithBaseURL(baseURL))
	}
	return ds.NewClient(appID, opts...), nil
}

func runDoubaoTTSV1Synthesize(ctx context.Context, c *Cortex, task Document) (*RunResult, error) {
	cred, err := c.ResolveCred(ctx, task.GetString("cred"))
	if err != nil {
		return nil, err
	}
	client, err := newDoubaoClient(cred)
	if err != nil {
		return nil, err
	}

	req := &ds.TTSRequest{
		Text:      task.GetString("text"),
		VoiceType: task.GetString("voice_type"),
		Encoding:  ds.AudioEncoding(task.GetString("encoding")),
		Cluster:   task.GetString("cluster"),
	}

	reqCtx, cancel := context.WithTimeout(ctx, 300*time.Second)
	defer cancel()

	resp, err := client.TTS.Synthesize(reqCtx, req)
	if err != nil {
		return nil, fmt.Errorf("doubao tts v1: %w", err)
	}

	result := &RunResult{Kind: task.Kind, Status: "ok", AudioSize: len(resp.Audio)}
	if output := task.GetString("output"); output != "" && len(resp.Audio) > 0 {
		if err := os.WriteFile(output, resp.Audio, 0644); err != nil {
			return nil, fmt.Errorf("write audio: %w", err)
		}
		result.AudioFile = output
	}
	return result, nil
}

func runDoubaoTTSV1Stream(ctx context.Context, c *Cortex, task Document) (*RunResult, error) {
	cred, err := c.ResolveCred(ctx, task.GetString("cred"))
	if err != nil {
		return nil, err
	}
	client, err := newDoubaoClient(cred)
	if err != nil {
		return nil, err
	}

	req := &ds.TTSRequest{
		Text:      task.GetString("text"),
		VoiceType: task.GetString("voice_type"),
		Encoding:  ds.AudioEncoding(task.GetString("encoding")),
		Cluster:   task.GetString("cluster"),
	}

	reqCtx, cancel := context.WithTimeout(ctx, 300*time.Second)
	defer cancel()

	var audioBuf bytes.Buffer
	for chunk, err := range client.TTS.SynthesizeStream(reqCtx, req) {
		if err != nil {
			return nil, fmt.Errorf("doubao tts v1 stream: %w", err)
		}
		if chunk.Audio != nil {
			audioBuf.Write(chunk.Audio)
		}
	}

	result := &RunResult{Kind: task.Kind, Status: "ok", AudioSize: audioBuf.Len()}
	if output := task.GetString("output"); output != "" && audioBuf.Len() > 0 {
		if err := os.WriteFile(output, audioBuf.Bytes(), 0644); err != nil {
			return nil, fmt.Errorf("write audio: %w", err)
		}
		result.AudioFile = output
	}
	return result, nil
}

func runDoubaoTTSV2Stream(ctx context.Context, c *Cortex, task Document) (*RunResult, error) {
	cred, err := c.ResolveCred(ctx, task.GetString("cred"))
	if err != nil {
		return nil, err
	}
	client, err := newDoubaoClient(cred)
	if err != nil {
		return nil, err
	}

	req := &ds.TTSV2Request{
		Text:       task.GetString("text"),
		Speaker:    task.GetString("speaker"),
		ResourceID: task.GetString("resource_id"),
		Format:     task.GetString("format"),
		SampleRate: task.GetInt("sample_rate"),
	}

	reqCtx, cancel := context.WithTimeout(ctx, 300*time.Second)
	defer cancel()

	var audioBuf bytes.Buffer
	for chunk, err := range client.TTSV2.Stream(reqCtx, req) {
		if err != nil {
			return nil, fmt.Errorf("doubao tts v2 stream: %w", err)
		}
		if len(chunk.Audio) > 0 {
			audioBuf.Write(chunk.Audio)
		}
		if chunk.IsLast {
			break
		}
	}

	result := &RunResult{Kind: task.Kind, Status: "ok", AudioSize: audioBuf.Len()}
	if output := task.GetString("output"); output != "" && audioBuf.Len() > 0 {
		if err := os.WriteFile(output, audioBuf.Bytes(), 0644); err != nil {
			return nil, fmt.Errorf("write audio: %w", err)
		}
		result.AudioFile = output
	}
	return result, nil
}

func runDoubaoTTSV2Bidirectional(ctx context.Context, c *Cortex, task Document) (*RunResult, error) {
	cred, err := c.ResolveCred(ctx, task.GetString("cred"))
	if err != nil {
		return nil, err
	}
	client, err := newDoubaoClient(cred)
	if err != nil {
		return nil, err
	}

	req := &ds.TTSV2Request{
		Text:       task.GetString("text"),
		Speaker:    task.GetString("speaker"),
		ResourceID: task.GetString("resource_id"),
		Format:     task.GetString("format"),
	}
	resourceID := req.ResourceID
	if resourceID == "" {
		resourceID = ds.ResourceTTSV2
	}

	reqCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	session, err := client.TTSV2.OpenSession(reqCtx, &ds.TTSV2SessionConfig{
		Speaker:    req.Speaker,
		ResourceID: resourceID,
		Format:     req.Format,
		SampleRate: req.SampleRate,
	})
	if err != nil {
		return nil, fmt.Errorf("doubao tts v2 bidir: open session: %w", err)
	}
	defer session.Close()

	if err := session.SendText(reqCtx, req.Text, true); err != nil {
		return nil, fmt.Errorf("doubao tts v2 bidir: send text: %w", err)
	}

	var audioBuf bytes.Buffer
	for chunk, err := range session.Recv() {
		if err != nil {
			return nil, fmt.Errorf("doubao tts v2 bidir: recv: %w", err)
		}
		if len(chunk.Audio) > 0 {
			audioBuf.Write(chunk.Audio)
		}
		if chunk.IsLast {
			break
		}
	}

	result := &RunResult{Kind: task.Kind, Status: "ok", AudioSize: audioBuf.Len()}
	if output := task.GetString("output"); output != "" && audioBuf.Len() > 0 {
		if err := os.WriteFile(output, audioBuf.Bytes(), 0644); err != nil {
			return nil, fmt.Errorf("write audio: %w", err)
		}
		result.AudioFile = output
	}
	return result, nil
}

func runDoubaoTTSV2Async(ctx context.Context, c *Cortex, task Document) (*RunResult, error) {
	cred, err := c.ResolveCred(ctx, task.GetString("cred"))
	if err != nil {
		return nil, err
	}
	client, err := newDoubaoClient(cred)
	if err != nil {
		return nil, err
	}

	req := &ds.AsyncTTSRequest{
		Text:      task.GetString("text"),
		VoiceType: task.GetString("voice_type"),
	}

	reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	taskResp, err := client.TTS.CreateAsyncTask(reqCtx, req)
	if err != nil {
		return nil, fmt.Errorf("doubao tts v2 async: %w", err)
	}

	return &RunResult{Kind: task.Kind, Status: "ok", TaskID: taskResp.ID}, nil
}

func runDoubaoASRV1Recognize(ctx context.Context, c *Cortex, task Document) (*RunResult, error) {
	cred, err := c.ResolveCred(ctx, task.GetString("cred"))
	if err != nil {
		return nil, err
	}
	client, err := newDoubaoClient(cred)
	if err != nil {
		return nil, err
	}

	audioPath := task.GetString("audio")
	if audioPath == "" {
		return nil, fmt.Errorf("doubao asr v1: missing 'audio' field")
	}
	audioData, err := os.ReadFile(audioPath)
	if err != nil {
		return nil, fmt.Errorf("read audio: %w", err)
	}

	req := &ds.OneSentenceRequest{
		Audio:  audioData,
		Format: ds.AudioFormat(task.GetString("format")),
	}

	reqCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	result, err := client.ASR.RecognizeOneSentence(reqCtx, req)
	if err != nil {
		return nil, fmt.Errorf("doubao asr v1: %w", err)
	}

	return &RunResult{Kind: task.Kind, Status: "ok", Text: result.Text}, nil
}

func runDoubaoVoiceList(ctx context.Context, c *Cortex, task Document) (*RunResult, error) {
	cred, err := c.ResolveCred(ctx, task.GetString("cred"))
	if err != nil {
		return nil, err
	}

	consoleAK, _ := cred["console_ak"].(string)
	consoleSK, _ := cred["console_sk"].(string)
	if consoleAK == "" || consoleSK == "" {
		return nil, fmt.Errorf("doubao voice list requires console_ak and console_sk in cred")
	}

	appID, _ := cred["app_id"].(string)
	console := ds.NewConsole(consoleAK, consoleSK)

	reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	resp, err := console.ListVoiceCloneStatus(reqCtx, &ds.ListVoiceCloneStatusRequest{AppID: appID})
	if err != nil {
		return nil, fmt.Errorf("doubao voice list: %w", err)
	}

	return &RunResult{Kind: task.Kind, Status: "ok", Data: map[string]any{
		"total":  resp.Total,
		"voices": resp.Statuses,
	}}, nil
}

func runDoubaoASRV1Stream(ctx context.Context, c *Cortex, task Document) (*RunResult, error) {
	cred, err := c.ResolveCred(ctx, task.GetString("cred"))
	if err != nil {
		return nil, err
	}
	client, err := newDoubaoClient(cred)
	if err != nil {
		return nil, err
	}
	audioPath := task.GetString("audio")
	if audioPath == "" {
		return nil, fmt.Errorf("doubaospeech/asr/v1/stream: missing 'audio'")
	}
	audioData, err := os.ReadFile(audioPath)
	if err != nil {
		return nil, fmt.Errorf("read audio: %w", err)
	}
	config := &ds.StreamASRConfig{
		Format:     ds.AudioFormat(task.GetString("format")),
		SampleRate: ds.SampleRate(task.GetInt("sample_rate")),
	}
	reqCtx, cancel := context.WithTimeout(ctx, 300*time.Second)
	defer cancel()
	session, err := client.ASR.OpenStreamSession(reqCtx, config)
	if err != nil {
		return nil, fmt.Errorf("doubao asr v1 stream: open: %w", err)
	}
	defer session.Close()
	chunkSize := 3200
	for i := 0; i < len(audioData); i += chunkSize {
		end := i + chunkSize
		isLast := end >= len(audioData)
		if isLast {
			end = len(audioData)
		}
		session.SendAudio(reqCtx, audioData[i:end], isLast)
		if !isLast {
			time.Sleep(100 * time.Millisecond)
		}
	}
	var finalText string
	for chunk, err := range session.Recv() {
		if err != nil {
			return nil, fmt.Errorf("doubao asr v1 stream: recv: %w", err)
		}
		if chunk.Text != "" {
			finalText = chunk.Text
		}
		if chunk.IsFinal {
			break
		}
	}
	return &RunResult{Kind: task.Kind, Status: "ok", Text: finalText}, nil
}

func runDoubaoASRV2Stream(ctx context.Context, c *Cortex, task Document) (*RunResult, error) {
	cred, err := c.ResolveCred(ctx, task.GetString("cred"))
	if err != nil {
		return nil, err
	}
	client, err := newDoubaoClient(cred)
	if err != nil {
		return nil, err
	}
	audioPath := task.GetString("audio")
	if audioPath == "" {
		return nil, fmt.Errorf("doubaospeech/asr/v2/stream: missing 'audio'")
	}
	audioData, err := os.ReadFile(audioPath)
	if err != nil {
		return nil, fmt.Errorf("read audio: %w", err)
	}
	config := &ds.ASRV2Config{
		Format:     task.GetString("format"),
		SampleRate: task.GetInt("sample_rate"),
		Language:   task.GetString("language"),
		ResourceID: task.GetString("resource_id"),
	}
	reqCtx, cancel := context.WithTimeout(ctx, 300*time.Second)
	defer cancel()
	session, err := client.ASRV2.OpenStreamSession(reqCtx, config)
	if err != nil {
		return nil, fmt.Errorf("doubao asr v2 stream: open: %w", err)
	}
	defer session.Close()
	chunkSize := 3200
	for i := 0; i < len(audioData); i += chunkSize {
		end := i + chunkSize
		isLast := end >= len(audioData)
		if isLast {
			end = len(audioData)
		}
		session.SendAudio(reqCtx, audioData[i:end], isLast)
		if !isLast {
			time.Sleep(100 * time.Millisecond)
		}
	}
	var finalText string
	for result, err := range session.Recv() {
		if err != nil {
			return nil, fmt.Errorf("doubao asr v2 stream: recv: %w", err)
		}
		if result.Text != "" {
			finalText = result.Text
		}
	}
	return &RunResult{Kind: task.Kind, Status: "ok", Text: finalText}, nil
}

func runDoubaoASRV2File(ctx context.Context, c *Cortex, task Document) (*RunResult, error) {
	cred, err := c.ResolveCred(ctx, task.GetString("cred"))
	if err != nil {
		return nil, err
	}
	client, err := newDoubaoClient(cred)
	if err != nil {
		return nil, err
	}
	req := &ds.ASRV2AsyncRequest{
		AudioURL: task.GetString("audio_url"),
		Language: task.GetString("language"),
	}
	reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	result, err := client.ASRV2.SubmitAsync(reqCtx, req)
	if err != nil {
		return nil, fmt.Errorf("doubao asr v2 file: %w", err)
	}
	return &RunResult{Kind: task.Kind, Status: "ok", TaskID: result.TaskID}, nil
}

func runDoubaoVoiceTrain(ctx context.Context, c *Cortex, task Document) (*RunResult, error) {
	cred, err := c.ResolveCred(ctx, task.GetString("cred"))
	if err != nil {
		return nil, err
	}
	client, err := newDoubaoClient(cred)
	if err != nil {
		return nil, err
	}
	req := &ds.VoiceCloneTrainRequest{
		SpeakerID: task.GetString("speaker_id"),
		Language:  ds.Language(task.GetString("language")),
	}
	if urls, ok := task.Fields["audio_urls"].([]any); ok {
		for _, u := range urls {
			if s, ok := u.(string); ok {
				req.AudioURLs = append(req.AudioURLs, s)
			}
		}
	}
	reqCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	taskResp, err := client.VoiceClone.Train(reqCtx, req)
	if err != nil {
		return nil, fmt.Errorf("doubao voice train: %w", err)
	}
	return &RunResult{Kind: task.Kind, Status: "ok", TaskID: taskResp.ID}, nil
}

func runDoubaoRealtimeConnect(ctx context.Context, c *Cortex, task Document) (*RunResult, error) {
	cred, err := c.ResolveCred(ctx, task.GetString("cred"))
	if err != nil {
		return nil, err
	}
	client, err := newDoubaoClient(cred)
	if err != nil {
		return nil, err
	}
	config := &ds.RealtimeConfig{}
	reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	session, err := client.Realtime.Connect(reqCtx, config)
	if err != nil {
		return nil, fmt.Errorf("doubao realtime: %w", err)
	}
	defer session.Close()

	greeting := task.GetString("greeting")
	if greeting != "" {
		session.SayHello(reqCtx, greeting)
	}

	return &RunResult{Kind: task.Kind, Status: "ok", Text: "Connected", Data: map[string]any{"session_id": session.SessionID()}}, nil
}

func runDoubaoTranslationStream(ctx context.Context, c *Cortex, task Document) (*RunResult, error) {
	cred, err := c.ResolveCred(ctx, task.GetString("cred"))
	if err != nil {
		return nil, err
	}
	client, err := newDoubaoClient(cred)
	if err != nil {
		return nil, err
	}
	audioPath := task.GetString("audio")
	if audioPath == "" {
		return nil, fmt.Errorf("doubaospeech/translation/stream: missing 'audio'")
	}
	audioData, err := os.ReadFile(audioPath)
	if err != nil {
		return nil, fmt.Errorf("read audio: %w", err)
	}
	config := &ds.TranslationConfig{
		SourceLanguage: ds.Language(task.GetString("source_language")),
		TargetLanguage: ds.Language(task.GetString("target_language")),
	}
	reqCtx, cancel := context.WithTimeout(ctx, 300*time.Second)
	defer cancel()
	session, err := client.Translation.OpenSession(reqCtx, config)
	if err != nil {
		return nil, fmt.Errorf("doubao translation: open: %w", err)
	}
	defer session.Close()
	chunkSize := 3200
	for i := 0; i < len(audioData); i += chunkSize {
		end := i + chunkSize
		if end > len(audioData) {
			end = len(audioData)
		}
		isLast := end >= len(audioData)
		session.SendAudio(reqCtx, audioData[i:end], isLast)
		if !isLast {
			time.Sleep(100 * time.Millisecond)
		}
	}
	var source, target string
	for chunk, err := range session.Recv() {
		if err != nil {
			break
		}
		if chunk.SourceText != "" {
			source = chunk.SourceText
		}
		if chunk.TargetText != "" {
			target = chunk.TargetText
		}
		if chunk.IsFinal {
			break
		}
	}
	return &RunResult{Kind: task.Kind, Status: "ok", Data: map[string]any{"source": source, "target": target}}, nil
}

func runDoubaoPodcastHTTPSubmit(ctx context.Context, c *Cortex, task Document) (*RunResult, error) {
	cred, err := c.ResolveCred(ctx, task.GetString("cred"))
	if err != nil {
		return nil, err
	}
	client, err := newDoubaoClient(cred)
	if err != nil {
		return nil, err
	}
	req := &ds.PodcastTaskRequest{}
	if scripts, ok := task.Fields["script"].([]any); ok {
		for _, s := range scripts {
			if m, ok := s.(map[string]any); ok {
				req.Script = append(req.Script, ds.PodcastLine{
					SpeakerID: m["speaker"].(string),
					Text:      m["text"].(string),
				})
			}
		}
	}
	reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	taskResp, err := client.Podcast.CreateTask(reqCtx, req)
	if err != nil {
		return nil, fmt.Errorf("doubao podcast http: %w", err)
	}
	return &RunResult{Kind: task.Kind, Status: "ok", TaskID: taskResp.ID}, nil
}

func runDoubaoPodcastSAMI(ctx context.Context, c *Cortex, task Document) (*RunResult, error) {
	cred, err := c.ResolveCred(ctx, task.GetString("cred"))
	if err != nil {
		return nil, err
	}
	client, err := newDoubaoClient(cred)
	if err != nil {
		return nil, err
	}
	req := &ds.PodcastSAMIRequest{
		InputText: task.GetString("input_text"),
	}
	if si, ok := task.Fields["speaker_info"].(map[string]any); ok {
		req.SpeakerInfo = &ds.PodcastSpeakerInfo{}
		if speakers, ok := si["speakers"].([]any); ok {
			for _, s := range speakers {
				if str, ok := s.(string); ok {
					req.SpeakerInfo.Speakers = append(req.SpeakerInfo.Speakers, str)
				}
			}
		}
	}
	reqCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	session, err := client.Podcast.StreamSAMI(reqCtx, req)
	if err != nil {
		return nil, fmt.Errorf("doubao podcast sami: open: %w", err)
	}
	defer session.Close()
	var audioBuf bytes.Buffer
	for chunk, err := range session.Recv() {
		if err != nil {
			return nil, fmt.Errorf("doubao podcast sami: recv: %w", err)
		}
		if len(chunk.Audio) > 0 {
			audioBuf.Write(chunk.Audio)
		}
		if chunk.IsLast {
			break
		}
	}
	result := &RunResult{Kind: task.Kind, Status: "ok", AudioSize: audioBuf.Len()}
	if output := task.GetString("output"); output != "" && audioBuf.Len() > 0 {
		os.WriteFile(output, audioBuf.Bytes(), 0644)
		result.AudioFile = output
	}
	return result, nil
}

func runDoubaoMeetingCreate(ctx context.Context, c *Cortex, task Document) (*RunResult, error) {
	cred, err := c.ResolveCred(ctx, task.GetString("cred"))
	if err != nil {
		return nil, err
	}
	client, err := newDoubaoClient(cred)
	if err != nil {
		return nil, err
	}
	req := &ds.MeetingTaskRequest{
		AudioURL: task.GetString("audio_url"),
		Language: ds.Language(task.GetString("language")),
	}
	reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	taskResp, err := client.Meeting.CreateTask(reqCtx, req)
	if err != nil {
		return nil, fmt.Errorf("doubao meeting: %w", err)
	}
	return &RunResult{Kind: task.Kind, Status: "ok", TaskID: taskResp.ID}, nil
}

func runDoubaoMediaSubtitle(ctx context.Context, c *Cortex, task Document) (*RunResult, error) {
	cred, err := c.ResolveCred(ctx, task.GetString("cred"))
	if err != nil {
		return nil, err
	}
	client, err := newDoubaoClient(cred)
	if err != nil {
		return nil, err
	}
	req := &ds.SubtitleRequest{
		MediaURL: task.GetString("media_url"),
		Language: ds.Language(task.GetString("language")),
	}
	reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	taskResp, err := client.Media.ExtractSubtitle(reqCtx, req)
	if err != nil {
		return nil, fmt.Errorf("doubao media subtitle: %w", err)
	}
	return &RunResult{Kind: task.Kind, Status: "ok", TaskID: taskResp.ID}, nil
}
