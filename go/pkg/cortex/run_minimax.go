package cortex

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/haivivi/giztoy/go/pkg/minimax"
)

func mapKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func init() {
	RegisterRunHandler("minimax/text/chat", runMinimaxTextChat)
	RegisterRunHandler("minimax/text/chat-stream", runMinimaxTextChatStream)
	RegisterRunHandler("minimax/speech/synthesize", runMinimaxSpeechSynthesize)
	RegisterRunHandler("minimax/speech/stream", runMinimaxSpeechStream)
	RegisterRunHandler("minimax/speech/async", runMinimaxSpeechAsync)
	RegisterRunHandler("minimax/video/t2v", runMinimaxVideoT2V)
	RegisterRunHandler("minimax/image/generate", runMinimaxImageGenerate)
	RegisterRunHandler("minimax/music/generate", runMinimaxMusicGenerate)
	RegisterRunHandler("minimax/voice/list", runMinimaxVoiceList)
	RegisterRunHandler("minimax/file/list", runMinimaxFileList)
}

func newMinimaxClient(cred map[string]any) (*minimax.Client, error) {
	apiKey, _ := cred["api_key"].(string)
	if apiKey == "" {
		// Try alternate key names that go-yaml might produce
		for k, v := range cred {
			if strings.Contains(k, "api") && strings.Contains(k, "key") {
				if s, ok := v.(string); ok {
					apiKey = s
					break
				}
			}
		}
	}
	if apiKey == "" {
		return nil, fmt.Errorf("minimax cred missing api_key (keys: %v)", mapKeys(cred))
	}
	var opts []minimax.Option
	if baseURL, _ := cred["base_url"].(string); baseURL != "" {
		opts = append(opts, minimax.WithBaseURL(baseURL))
	}
	return minimax.NewClient(apiKey, opts...), nil
}

func runMinimaxTextChat(ctx context.Context, c *Cortex, task Document) (*RunResult, error) {
	credRef := task.GetString("cred")
	if credRef == "" {
		return nil, fmt.Errorf("minimax/text/chat: missing 'cred' field")
	}
	cred, err := c.ResolveCred(ctx, credRef)
	if err != nil {
		return nil, err
	}
	client, err := newMinimaxClient(cred)
	if err != nil {
		return nil, err
	}

	var req minimax.ChatCompletionRequest
	req.Model = task.GetString("model")
	if req.Model == "" {
		req.Model = minimax.ModelM2_1
	}
	msgs, _ := task.Fields["messages"].([]any)
	for _, m := range msgs {
		mm, _ := m.(map[string]any)
		req.Messages = append(req.Messages, minimax.Message{
			Role:    mm["role"].(string),
			Content: mm["content"].(string),
		})
	}
	if mt := task.GetInt("max_tokens"); mt > 0 {
		req.MaxTokens = mt
	}

	reqCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	resp, err := client.Text.CreateChatCompletion(reqCtx, &req)
	if err != nil {
		return nil, fmt.Errorf("minimax chat: %w", err)
	}

	text := ""
	if len(resp.Choices) > 0 && resp.Choices[0].Message != nil {
		if s, ok := resp.Choices[0].Message.Content.(string); ok {
			text = s
		}
	}

	return &RunResult{
		Kind:   task.Kind,
		Status: "ok",
		Text:   text,
		Data: map[string]any{
			"model": resp.Model,
			"usage": resp.Usage,
		},
	}, nil
}

func runMinimaxTextChatStream(ctx context.Context, c *Cortex, task Document) (*RunResult, error) {
	cred, err := c.ResolveCred(ctx, task.GetString("cred"))
	if err != nil {
		return nil, err
	}
	client, err := newMinimaxClient(cred)
	if err != nil {
		return nil, err
	}

	var req minimax.ChatCompletionRequest
	req.Model = task.GetString("model")
	if req.Model == "" {
		req.Model = minimax.ModelM2_1
	}
	msgs, _ := task.Fields["messages"].([]any)
	for _, m := range msgs {
		mm, _ := m.(map[string]any)
		req.Messages = append(req.Messages, minimax.Message{
			Role:    mm["role"].(string),
			Content: mm["content"].(string),
		})
	}

	reqCtx, cancel := context.WithTimeout(ctx, 300*time.Second)
	defer cancel()

	var fullContent string
	for chunk, err := range client.Text.CreateChatCompletionStream(reqCtx, &req) {
		if err != nil {
			return nil, fmt.Errorf("minimax stream: %w", err)
		}
		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta != nil {
			fullContent += chunk.Choices[0].Delta.Content
		}
	}

	return &RunResult{Kind: task.Kind, Status: "ok", Text: fullContent}, nil
}

func runMinimaxSpeechSynthesize(ctx context.Context, c *Cortex, task Document) (*RunResult, error) {
	cred, err := c.ResolveCred(ctx, task.GetString("cred"))
	if err != nil {
		return nil, err
	}
	client, err := newMinimaxClient(cred)
	if err != nil {
		return nil, err
	}

	req := &minimax.SpeechRequest{
		Model: task.GetString("model"),
		Text:  task.GetString("text"),
		VoiceSetting: &minimax.VoiceSetting{
			VoiceID: task.GetString("voice_id"),
		},
	}
	if req.Model == "" {
		req.Model = minimax.ModelSpeech26HD
	}

	reqCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	resp, err := client.Speech.Synthesize(reqCtx, req)
	if err != nil {
		return nil, fmt.Errorf("minimax speech: %w", err)
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

func runMinimaxSpeechStream(ctx context.Context, c *Cortex, task Document) (*RunResult, error) {
	cred, err := c.ResolveCred(ctx, task.GetString("cred"))
	if err != nil {
		return nil, err
	}
	client, err := newMinimaxClient(cred)
	if err != nil {
		return nil, err
	}

	req := &minimax.SpeechRequest{
		Model: task.GetString("model"),
		Text:  task.GetString("text"),
		VoiceSetting: &minimax.VoiceSetting{
			VoiceID: task.GetString("voice_id"),
		},
	}
	if req.Model == "" {
		req.Model = minimax.ModelSpeech26HD
	}

	reqCtx, cancel := context.WithTimeout(ctx, 300*time.Second)
	defer cancel()

	var audioBuf bytes.Buffer
	for chunk, err := range client.Speech.SynthesizeStream(reqCtx, req) {
		if err != nil {
			return nil, fmt.Errorf("minimax speech stream: %w", err)
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

func runMinimaxSpeechAsync(ctx context.Context, c *Cortex, task Document) (*RunResult, error) {
	cred, err := c.ResolveCred(ctx, task.GetString("cred"))
	if err != nil {
		return nil, err
	}
	client, err := newMinimaxClient(cred)
	if err != nil {
		return nil, err
	}

	req := &minimax.AsyncSpeechRequest{
		Model: task.GetString("model"),
		Text:  task.GetString("text"),
		VoiceSetting: &minimax.VoiceSetting{
			VoiceID: task.GetString("voice_id"),
		},
	}
	if req.Model == "" {
		req.Model = minimax.ModelSpeech26HD
	}

	reqCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	taskResp, err := client.Speech.CreateAsyncTask(reqCtx, req)
	if err != nil {
		return nil, fmt.Errorf("minimax async speech: %w", err)
	}

	return &RunResult{Kind: task.Kind, Status: "ok", TaskID: taskResp.ID}, nil
}

func runMinimaxVideoT2V(ctx context.Context, c *Cortex, task Document) (*RunResult, error) {
	cred, err := c.ResolveCred(ctx, task.GetString("cred"))
	if err != nil {
		return nil, err
	}
	client, err := newMinimaxClient(cred)
	if err != nil {
		return nil, err
	}

	req := &minimax.TextToVideoRequest{
		Model:  task.GetString("model"),
		Prompt: task.GetString("prompt"),
	}
	if req.Model == "" {
		req.Model = minimax.ModelHailuo23
	}

	reqCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	taskResp, err := client.Video.CreateTextToVideo(reqCtx, req)
	if err != nil {
		return nil, fmt.Errorf("minimax video t2v: %w", err)
	}

	return &RunResult{Kind: task.Kind, Status: "ok", TaskID: taskResp.ID}, nil
}

func runMinimaxImageGenerate(ctx context.Context, c *Cortex, task Document) (*RunResult, error) {
	cred, err := c.ResolveCred(ctx, task.GetString("cred"))
	if err != nil {
		return nil, err
	}
	client, err := newMinimaxClient(cred)
	if err != nil {
		return nil, err
	}

	req := &minimax.ImageGenerateRequest{
		Model:  task.GetString("model"),
		Prompt: task.GetString("prompt"),
	}
	if req.Model == "" {
		req.Model = minimax.ModelImage01
	}

	reqCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	resp, err := client.Image.Generate(reqCtx, req)
	if err != nil {
		return nil, fmt.Errorf("minimax image: %w", err)
	}

	data := map[string]any{"images_count": len(resp.Images)}
	if len(resp.Images) > 0 {
		data["first_url"] = resp.Images[0].URL
	}
	return &RunResult{Kind: task.Kind, Status: "ok", Data: data}, nil
}

func runMinimaxMusicGenerate(ctx context.Context, c *Cortex, task Document) (*RunResult, error) {
	cred, err := c.ResolveCred(ctx, task.GetString("cred"))
	if err != nil {
		return nil, err
	}
	client, err := newMinimaxClient(cred)
	if err != nil {
		return nil, err
	}

	req := &minimax.MusicRequest{
		Model:  task.GetString("model"),
		Prompt: task.GetString("prompt"),
		Lyrics: task.GetString("lyrics"),
	}
	if req.Model == "" {
		req.Model = minimax.ModelMusic20
	}

	reqCtx, cancel := context.WithTimeout(ctx, 300*time.Second)
	defer cancel()

	resp, err := client.Music.Generate(reqCtx, req)
	if err != nil {
		return nil, fmt.Errorf("minimax music: %w", err)
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

func runMinimaxVoiceList(ctx context.Context, c *Cortex, task Document) (*RunResult, error) {
	cred, err := c.ResolveCred(ctx, task.GetString("cred"))
	if err != nil {
		return nil, err
	}
	client, err := newMinimaxClient(cred)
	if err != nil {
		return nil, err
	}

	reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	resp, err := client.Voice.List(reqCtx, minimax.VoiceTypeAll)
	if err != nil {
		return nil, fmt.Errorf("minimax voice list: %w", err)
	}

	return &RunResult{Kind: task.Kind, Status: "ok", Data: map[string]any{"voices": resp}}, nil
}

func runMinimaxFileList(ctx context.Context, c *Cortex, task Document) (*RunResult, error) {
	cred, err := c.ResolveCred(ctx, task.GetString("cred"))
	if err != nil {
		return nil, err
	}
	client, err := newMinimaxClient(cred)
	if err != nil {
		return nil, err
	}

	purpose := task.GetString("purpose")
	if purpose == "" {
		purpose = "voice_clone"
	}

	reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	resp, err := client.File.List(reqCtx, minimax.FilePurpose(purpose))
	if err != nil {
		return nil, fmt.Errorf("minimax file list: %w", err)
	}

	return &RunResult{Kind: task.Kind, Status: "ok", Data: map[string]any{"files": resp}}, nil
}
