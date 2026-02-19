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
	RegisterRunHandler("doubaospeech/voice/list", runDoubaoVoiceList)
}

func newDoubaoClient(cred map[string]any) (*ds.Client, error) {
	appID, _ := cred["app_id"].(string)
	if appID == "" {
		return nil, fmt.Errorf("doubaospeech cred missing app_id")
	}
	var opts []ds.Option
	if token, _ := cred["token"].(string); token != "" {
		opts = append(opts, ds.WithBearerToken(token))
	}
	if apiKey, _ := cred["api_key"].(string); apiKey != "" {
		appKey, _ := cred["app_key"].(string)
		if appKey == "" {
			appKey = appID
		}
		opts = append(opts, ds.WithV2APIKey(apiKey, appKey))
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
