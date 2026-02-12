package doubao

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	ds "github.com/haivivi/giztoy/go/pkg/doubaospeech"
)

var ttsCmd = &cobra.Command{
	Use:   "tts",
	Short: "Text-to-Speech synthesis service",
	Long: `Text-to-Speech (TTS) synthesis service.

Supports two API versions:
  v1  Classic API (/api/v1/tts)
  v2  BigModel API (/api/v3/tts/*) - Recommended

IMPORTANT: Speaker voice must match Resource ID!
  | Resource ID    | Speaker Suffix Required |
  |----------------|-------------------------|
  | seed-tts-2.0   | *_uranus_bigtts         |
  | seed-tts-1.0   | *_moon_bigtts           |`,
}

// ============================================================================
// V1 Commands
// ============================================================================

var ttsV1Cmd = &cobra.Command{
	Use:   "v1",
	Short: "TTS V1 API (Classic)",
	Long: `TTS V1 Classic API (/api/v1/tts).

Example request file (tts-v1.yaml):
  text: 你好，这是一段测试语音。
  voice_type: zh_female_cancan
  encoding: mp3
  sample_rate: 24000
  cluster: volcano_tts`,
}

var ttsV1SynthesizeCmd = &cobra.Command{
	Use:   "synthesize",
	Short: "V1 synchronous synthesis",
	Long: `Synthesize speech using V1 Classic API.

Example:
  giztoy doubao tts v1 synthesize -f tts-v1.yaml -o output.mp3`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireInputFile(); err != nil {
			return err
		}
		if outputFile == "" {
			return fmt.Errorf("output file is required, use -o flag")
		}

		svc, err := loadServiceConfig()
		if err != nil {
			return err
		}
		client, err := createClientWith(svc)
		if err != nil {
			return err
		}

		var req ds.TTSRequest
		if err := loadRequest(inputFile, &req); err != nil {
			return err
		}
		if req.VoiceType == "" && svc.DefaultVoice != "" {
			req.VoiceType = svc.DefaultVoice
		}

		printVerbose("Using V1 API (classic)")
		printVerbose("Voice type: %s", req.VoiceType)
		printVerbose("Text length: %d characters", len(req.Text))

		reqCtx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
		defer cancel()

		resp, err := client.TTS.Synthesize(reqCtx, &req)
		if err != nil {
			return fmt.Errorf("V1 synthesis failed: %w", err)
		}

		if len(resp.Audio) > 0 {
			if err := outputBytes(resp.Audio, outputFile); err != nil {
				return fmt.Errorf("failed to write audio: %w", err)
			}
			printSuccess("Audio saved to: %s (%s)", outputFile, formatBytes(int64(len(resp.Audio))))
		}

		result := map[string]any{
			"api_version": "v1",
			"audio_size":  len(resp.Audio),
			"duration":    resp.Duration,
			"output_file": outputFile,
		}
		return outputResult(result, "", outputJSON)
	},
}

var ttsV1StreamCmd = &cobra.Command{
	Use:   "stream",
	Short: "V1 streaming synthesis",
	Long: `Stream speech synthesis using V1 Classic API.

Example:
  giztoy doubao tts v1 stream -f tts-v1.yaml -o output.mp3`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireInputFile(); err != nil {
			return err
		}
		if outputFile == "" {
			return fmt.Errorf("output file is required, use -o flag")
		}

		svc, err := loadServiceConfig()
		if err != nil {
			return err
		}
		client, err := createClientWith(svc)
		if err != nil {
			return err
		}

		var req ds.TTSRequest
		if err := loadRequest(inputFile, &req); err != nil {
			return err
		}
		if req.VoiceType == "" && svc.DefaultVoice != "" {
			req.VoiceType = svc.DefaultVoice
		}

		printVerbose("Using V1 API (classic streaming)")
		printVerbose("Voice type: %s", req.VoiceType)

		reqCtx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
		defer cancel()

		var audioBuf bytes.Buffer
		var lastChunk *ds.TTSChunk

		for chunk, err := range client.TTS.SynthesizeStream(reqCtx, &req) {
			if err != nil {
				return fmt.Errorf("V1 streaming failed: %w", err)
			}
			if chunk.Audio != nil {
				audioBuf.Write(chunk.Audio)
			}
			lastChunk = chunk
		}

		if audioBuf.Len() > 0 {
			if err := outputBytes(audioBuf.Bytes(), outputFile); err != nil {
				return fmt.Errorf("failed to write audio: %w", err)
			}
			printSuccess("Audio saved to: %s (%s)", outputFile, formatBytes(int64(audioBuf.Len())))
		}

		result := map[string]any{
			"api_version": "v1",
			"audio_size":  audioBuf.Len(),
			"output_file": outputFile,
		}
		if lastChunk != nil {
			result["duration"] = lastChunk.Duration
		}
		return outputResult(result, "", outputJSON)
	},
}

// ============================================================================
// V2 Commands
// ============================================================================

var ttsV2Cmd = &cobra.Command{
	Use:   "v2",
	Short: "TTS V2 API (BigModel) - Recommended",
	Long: `TTS V2 BigModel API (/api/v3/tts/*).

IMPORTANT: Speaker voice must match Resource ID!
  | Resource ID    | Speaker Suffix Required | Example                           |
  |----------------|-------------------------|-----------------------------------|
  | seed-tts-2.0   | *_uranus_bigtts         | zh_female_xiaohe_uranus_bigtts    |
  | seed-tts-1.0   | *_moon_bigtts           | zh_female_shuangkuaisisi_moon_bigtts |

Example request file (tts-v2.yaml):
  text: 你好，这是一段测试语音。
  speaker: zh_female_xiaohe_uranus_bigtts
  resource_id: seed-tts-2.0
  format: mp3
  sample_rate: 24000`,
}

var ttsV2StreamCmd = &cobra.Command{
	Use:   "stream",
	Short: "V2 HTTP streaming synthesis (recommended)",
	Long: `Stream speech synthesis using V2 HTTP API.

Endpoint: POST /api/v3/tts/unidirectional

Example:
  giztoy doubao tts v2 stream -f tts-v2.yaml -o output.mp3`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireInputFile(); err != nil {
			return err
		}
		if outputFile == "" {
			return fmt.Errorf("output file is required, use -o flag")
		}

		svc, err := loadServiceConfig()
		if err != nil {
			return err
		}
		client, err := createClientWith(svc)
		if err != nil {
			return err
		}

		var req ds.TTSV2Request
		if err := loadRequest(inputFile, &req); err != nil {
			return err
		}
		if req.Speaker == "" && svc.DefaultVoice != "" {
			req.Speaker = svc.DefaultVoice
		}

		printVerbose("Using V2 API (BigModel HTTP streaming)")
		printVerbose("Speaker: %s", req.Speaker)
		printVerbose("Resource ID: %s", req.ResourceID)

		reqCtx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
		defer cancel()

		var audioBuf bytes.Buffer
		chunkCount := 0

		for chunk, err := range client.TTSV2.Stream(reqCtx, &req) {
			if err != nil {
				return fmt.Errorf("V2 streaming failed: %w", err)
			}
			if len(chunk.Audio) > 0 {
				audioBuf.Write(chunk.Audio)
				chunkCount++
			}
			if chunk.IsLast {
				break
			}
		}

		if audioBuf.Len() > 0 {
			if err := outputBytes(audioBuf.Bytes(), outputFile); err != nil {
				return fmt.Errorf("failed to write audio: %w", err)
			}
			printSuccess("Audio saved to: %s (%s)", outputFile, formatBytes(int64(audioBuf.Len())))
		}

		result := map[string]any{
			"api_version": "v2",
			"audio_size":  audioBuf.Len(),
			"chunks":      chunkCount,
			"output_file": outputFile,
		}
		return outputResult(result, "", outputJSON)
	},
}

var ttsV2BidirectionalCmd = &cobra.Command{
	Use:   "bidirectional",
	Short: "V2 WebSocket bidirectional streaming",
	Long: `Bidirectional streaming synthesis using V2 WebSocket API.

Allows sending text incrementally while receiving audio output.

Example:
  giztoy doubao tts v2 bidirectional -f tts-v2.yaml -o output.mp3`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireInputFile(); err != nil {
			return err
		}
		if outputFile == "" {
			return fmt.Errorf("output file is required, use -o flag")
		}

		svc, err := loadServiceConfig()
		if err != nil {
			return err
		}
		client, err := createClientWith(svc)
		if err != nil {
			return err
		}

		var req ds.TTSV2Request
		if err := loadRequest(inputFile, &req); err != nil {
			return err
		}
		if req.Speaker == "" && svc.DefaultVoice != "" {
			req.Speaker = svc.DefaultVoice
		}

		resourceID := req.ResourceID
		if resourceID == "" {
			resourceID = ds.ResourceTTSV2
		}

		printVerbose("Using V2 API (BigModel WebSocket bidirectional)")
		printVerbose("Speaker: %s", req.Speaker)
		printVerbose("Resource ID: %s", resourceID)

		reqCtx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()

		session, err := client.TTSV2.OpenSession(reqCtx, &ds.TTSV2SessionConfig{
			Speaker:    req.Speaker,
			ResourceID: resourceID,
			Format:     req.Format,
			SampleRate: req.SampleRate,
		})
		if err != nil {
			return fmt.Errorf("failed to open session: %w", err)
		}
		defer session.Close()

		printVerbose("Session opened")

		if err := session.SendText(reqCtx, req.Text, true); err != nil {
			return fmt.Errorf("failed to send text: %w", err)
		}

		var audioBuf bytes.Buffer
		chunkCount := 0

		for chunk, err := range session.Recv() {
			if err != nil {
				return fmt.Errorf("receive error: %w", err)
			}
			if len(chunk.Audio) > 0 {
				audioBuf.Write(chunk.Audio)
				chunkCount++
			}
			if chunk.IsLast {
				break
			}
		}

		if audioBuf.Len() > 0 {
			if err := outputBytes(audioBuf.Bytes(), outputFile); err != nil {
				return fmt.Errorf("failed to write audio: %w", err)
			}
			printSuccess("Audio saved to: %s (%s)", outputFile, formatBytes(int64(audioBuf.Len())))
		}

		result := map[string]any{
			"api_version": "v2-bidirectional",
			"audio_size":  audioBuf.Len(),
			"chunks":      chunkCount,
			"output_file": outputFile,
		}
		return outputResult(result, "", outputJSON)
	},
}

var ttsV2AsyncCmd = &cobra.Command{
	Use:   "async",
	Short: "V2 async long text synthesis",
	Long: `Create an asynchronous synthesis task for long text (up to 10,000 chars).

Returns a task ID. Use 'giztoy doubao tts v2 status <task_id>' to check.

Example:
  giztoy doubao tts v2 async -f tts-async.yaml`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireInputFile(); err != nil {
			return err
		}

		svc, err := loadServiceConfig()
		if err != nil {
			return err
		}
		client, err := createClientWith(svc)
		if err != nil {
			return err
		}

		var req ds.AsyncTTSRequest
		if err := loadRequest(inputFile, &req); err != nil {
			return err
		}
		if req.VoiceType == "" && svc.DefaultVoice != "" {
			req.VoiceType = svc.DefaultVoice
		}

		printVerbose("Text length: %d characters", len(req.Text))

		reqCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		task, err := client.TTS.CreateAsyncTask(reqCtx, &req)
		if err != nil {
			return fmt.Errorf("async TTS submit failed: %w", err)
		}

		printSuccess("Async TTS task submitted!")
		printInfo("Task ID: %s", task.ID)

		result := map[string]any{
			"api_version": "v1-async",
			"task_id":     task.ID,
			"status":      "submitted",
		}
		return outputResult(result, outputFile, outputJSON)
	},
}

var ttsV2StatusCmd = &cobra.Command{
	Use:   "status <task_id>",
	Short: "Query async task status",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		taskID := args[0]

		client, err := createClient()
		if err != nil {
			return err
		}

		printVerbose("Querying task: %s", taskID)

		reqCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		status, err := client.TTS.GetAsyncTask(reqCtx, taskID)
		if err != nil {
			return fmt.Errorf("query task status failed: %w", err)
		}

		printInfo("Task ID: %s", taskID)
		printInfo("Status: %s", status.Status)
		if status.AudioURL != "" {
			printSuccess("Audio URL: %s", status.AudioURL)
		}

		result := map[string]any{
			"api_version": "v1-async",
			"task_id":     taskID,
			"status":      status.Status,
		}
		if status.AudioURL != "" {
			result["audio_url"] = status.AudioURL
			result["audio_duration"] = status.AudioDuration
		}
		return outputResult(result, outputFile, outputJSON)
	},
}

func init() {
	ttsV1Cmd.AddCommand(ttsV1SynthesizeCmd)
	ttsV1Cmd.AddCommand(ttsV1StreamCmd)

	ttsV2Cmd.AddCommand(ttsV2StreamCmd)
	ttsV2Cmd.AddCommand(ttsV2BidirectionalCmd)
	ttsV2Cmd.AddCommand(ttsV2AsyncCmd)
	ttsV2Cmd.AddCommand(ttsV2StatusCmd)

	ttsCmd.AddCommand(ttsV1Cmd)
	ttsCmd.AddCommand(ttsV2Cmd)
}
