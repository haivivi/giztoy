package commands

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/haivivi/giztoy/pkg/cli"
	ds "github.com/haivivi/giztoy/pkg/doubaospeech"
)

// ttsCmd is the root command for TTS services
var ttsCmd = &cobra.Command{
	Use:   "tts",
	Short: "Text-to-Speech synthesis service",
	Long: `Text-to-Speech (TTS) synthesis service.

Supports two API versions:
  - v1: Classic API (/api/v1/tts)
  - v2: BigModel API (/api/v3/tts/*) - Recommended

Commands:
  tts v1 synthesize   - V1 synchronous synthesis
  tts v1 stream       - V1 streaming synthesis
  tts v2 stream       - V2 HTTP streaming (recommended)
  tts v2 ws           - V2 WebSocket unidirectional
  tts v2 bidirectional - V2 WebSocket bidirectional
  tts v2 async        - V2 async long text synthesis

IMPORTANT: Speaker voice must match Resource ID!
  | Resource ID    | Speaker Suffix Required |
  |----------------|-------------------------|
  | seed-tts-2.0   | *_uranus_bigtts         |
  | seed-tts-1.0   | *_moon_bigtts           |

Example V2 request file (tts-v2.yaml):
  text: 你好，这是一段测试语音。
  speaker: zh_female_xiaohe_uranus_bigtts
  resource_id: seed-tts-2.0
  format: mp3
  sample_rate: 24000`,
}

// ============================================================================
// V1 Commands
// ============================================================================

var ttsV1Cmd = &cobra.Command{
	Use:   "v1",
	Short: "TTS V1 API (Classic)",
	Long: `TTS V1 Classic API (/api/v1/tts).

Uses Bearer Token authentication and cluster-based voice selection.

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
  doubaospeech tts v1 synthesize -f tts-v1.yaml -o output.mp3`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireInputFile(); err != nil {
			return err
		}

		outputPath := getOutputFile()
		if outputPath == "" {
			return fmt.Errorf("output file is required, use -o flag")
		}

		cliCtx, err := getContext()
		if err != nil {
			return err
		}

		client, err := createClient(cliCtx)
		if err != nil {
			return err
		}

		reqCtx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
		defer cancel()

		return runTTSV1Synthesize(reqCtx, client, cliCtx, outputPath)
	},
}

var ttsV1StreamCmd = &cobra.Command{
	Use:   "stream",
	Short: "V1 streaming synthesis",
	Long: `Stream speech synthesis using V1 Classic API.

Example:
  doubaospeech tts v1 stream -f tts-v1.yaml -o output.mp3`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireInputFile(); err != nil {
			return err
		}

		outputPath := getOutputFile()
		if outputPath == "" {
			return fmt.Errorf("output file is required, use -o flag")
		}

		cliCtx, err := getContext()
		if err != nil {
			return err
		}

		client, err := createClient(cliCtx)
		if err != nil {
			return err
		}

		reqCtx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
		defer cancel()

		return runTTSV1Stream(reqCtx, client, cliCtx, outputPath)
	},
}

// ============================================================================
// V2 Commands
// ============================================================================

var ttsV2Cmd = &cobra.Command{
	Use:   "v2",
	Short: "TTS V2 API (BigModel) - Recommended",
	Long: `TTS V2 BigModel API (/api/v3/tts/*).

Uses X-Api-* headers authentication and resource-based voice selection.

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
  doubaospeech tts v2 stream -f tts-v2.yaml -o output.mp3`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireInputFile(); err != nil {
			return err
		}

		outputPath := getOutputFile()
		if outputPath == "" {
			return fmt.Errorf("output file is required, use -o flag")
		}

		cliCtx, err := getContext()
		if err != nil {
			return err
		}

		client, err := createClient(cliCtx)
		if err != nil {
			return err
		}

		reqCtx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
		defer cancel()

		return runTTSV2Stream(reqCtx, client, cliCtx, outputPath)
	},
}

var ttsV2WSCmd = &cobra.Command{
	Use:   "ws",
	Short: "V2 WebSocket unidirectional streaming",
	Long: `Stream speech synthesis using V2 WebSocket API (unidirectional).

Endpoint: WSS /api/v3/tts/unidirectional

Example:
  doubaospeech tts v2 ws -f tts-v2.yaml -o output.mp3`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireInputFile(); err != nil {
			return err
		}

		outputPath := getOutputFile()
		if outputPath == "" {
			return fmt.Errorf("output file is required, use -o flag")
		}

		cliCtx, err := getContext()
		if err != nil {
			return err
		}

		printVerbose("Using context: %s", cliCtx.Name)
		printVerbose("WebSocket streaming to: %s", outputPath)

		// TODO: Implement WebSocket unidirectional streaming
		fmt.Println("[WebSocket unidirectional streaming not implemented yet]")
		fmt.Printf("Would stream speech via WebSocket to %s\n", outputPath)

		return nil
	},
}

var ttsV2BidirectionalCmd = &cobra.Command{
	Use:   "bidirectional",
	Short: "V2 WebSocket bidirectional streaming",
	Long: `Bidirectional streaming synthesis using V2 WebSocket API.

Endpoint: WSS /api/v3/tts/bidirection

Allows sending text incrementally while receiving audio output.
Useful for synthesizing text from streaming sources (e.g., LLM output).

Example:
  doubaospeech tts v2 bidirectional -f tts-v2.yaml -o output.mp3`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireInputFile(); err != nil {
			return err
		}

		outputPath := getOutputFile()
		if outputPath == "" {
			return fmt.Errorf("output file is required, use -o flag")
		}

		cliCtx, err := getContext()
		if err != nil {
			return err
		}

		client, err := createClient(cliCtx)
		if err != nil {
			return err
		}

		reqCtx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()

		return runTTSV2Bidirectional(reqCtx, client, cliCtx, outputPath)
	},
}

var ttsV2AsyncCmd = &cobra.Command{
	Use:   "async",
	Short: "V2 async long text synthesis",
	Long: `Create an asynchronous synthesis task for long text.

Endpoint: POST /api/v3/tts/async/submit

Supports text up to 10,000 characters. Returns a task ID for tracking.
Use 'doubaospeech tts v2 status <task_id>' to check the task status.

Example:
  doubaospeech tts v2 async -f tts-async.yaml
  doubaospeech tts v2 async -f tts-async.yaml --json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireInputFile(); err != nil {
			return err
		}

		cliCtx, err := getContext()
		if err != nil {
			return err
		}

		var req ds.AsyncTTSRequest
		if err := loadRequest(getInputFile(), &req); err != nil {
			return err
		}

		printVerbose("Using context: %s", cliCtx.Name)
		printVerbose("Text length: %d characters", len(req.Text))

		// TODO: Implement async API call
		result := map[string]any{
			"_note":    "API call not implemented yet",
			"_context": cliCtx.Name,
			"request":  req,
			"task_id":  "placeholder-task-id",
		}

		return outputResult(result, getOutputFile(), isJSONOutput())
	},
}

var ttsV2StatusCmd = &cobra.Command{
	Use:   "status <task_id>",
	Short: "Query async task status",
	Long: `Query the status of an asynchronous synthesis task.

Example:
  doubaospeech tts v2 status task_12345
  doubaospeech tts v2 status task_12345 --json`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		taskID := args[0]

		cliCtx, err := getContext()
		if err != nil {
			return err
		}

		printVerbose("Using context: %s", cliCtx.Name)
		printVerbose("Querying task: %s", taskID)

		// TODO: Implement task status query
		result := map[string]any{
			"_note":    "API call not implemented yet",
			"_context": cliCtx.Name,
			"task_id":  taskID,
			"status":   "pending",
		}

		return outputResult(result, getOutputFile(), isJSONOutput())
	},
}

// ============================================================================
// Implementation Functions
// ============================================================================

func runTTSV1Synthesize(ctx context.Context, client *ds.Client, cliCtx *cli.Context, outputPath string) error {
	var req ds.TTSRequest
	if err := loadRequest(getInputFile(), &req); err != nil {
		return err
	}

	if req.VoiceType == "" && cliCtx.DefaultVoice != "" {
		req.VoiceType = cliCtx.DefaultVoice
	}

	printVerbose("Using V1 API (classic)")
	printVerbose("Voice type: %s", req.VoiceType)
	printVerbose("Text length: %d characters", len(req.Text))

	resp, err := client.TTS.Synthesize(ctx, &req)
	if err != nil {
		return fmt.Errorf("V1 synthesis failed: %w", err)
	}

	if len(resp.Audio) > 0 {
		if err := outputBytes(resp.Audio, outputPath); err != nil {
			return fmt.Errorf("failed to write audio file: %w", err)
		}
		printSuccess("Audio saved to: %s (%s)", outputPath, formatBytes(int64(len(resp.Audio))))
	}

	result := map[string]any{
		"api_version": "v1",
		"audio_size":  len(resp.Audio),
		"duration":    resp.Duration,
		"output_file": outputPath,
	}

	return outputResult(result, "", isJSONOutput())
}

func runTTSV1Stream(ctx context.Context, client *ds.Client, cliCtx *cli.Context, outputPath string) error {
	var req ds.TTSRequest
	if err := loadRequest(getInputFile(), &req); err != nil {
		return err
	}

	if req.VoiceType == "" && cliCtx.DefaultVoice != "" {
		req.VoiceType = cliCtx.DefaultVoice
	}

	printVerbose("Using V1 API (classic streaming)")
	printVerbose("Voice type: %s", req.VoiceType)
	printVerbose("Text length: %d characters", len(req.Text))

	var audioBuf bytes.Buffer
	var lastChunk *ds.TTSChunk

	for chunk, err := range client.TTS.SynthesizeStream(ctx, &req) {
		if err != nil {
			return fmt.Errorf("V1 streaming failed: %w", err)
		}
		if chunk.Audio != nil {
			audioBuf.Write(chunk.Audio)
		}
		lastChunk = chunk
	}

	if audioBuf.Len() > 0 {
		if err := outputBytes(audioBuf.Bytes(), outputPath); err != nil {
			return fmt.Errorf("failed to write audio file: %w", err)
		}
		printSuccess("Audio saved to: %s (%s)", outputPath, formatBytes(int64(audioBuf.Len())))
	}

	result := map[string]any{
		"api_version": "v1",
		"audio_size":  audioBuf.Len(),
		"output_file": outputPath,
	}
	if lastChunk != nil {
		result["duration"] = lastChunk.Duration
	}

	return outputResult(result, "", isJSONOutput())
}

func runTTSV2Stream(ctx context.Context, client *ds.Client, cliCtx *cli.Context, outputPath string) error {
	var req ds.TTSV2Request
	if err := loadRequest(getInputFile(), &req); err != nil {
		return err
	}

	if req.Speaker == "" && cliCtx.DefaultVoice != "" {
		req.Speaker = cliCtx.DefaultVoice
	}

	printVerbose("Using V2 API (BigModel HTTP streaming)")
	printVerbose("Speaker: %s", req.Speaker)
	printVerbose("Resource ID: %s", req.ResourceID)
	printVerbose("Text length: %d characters", len(req.Text))

	var audioBuf bytes.Buffer
	chunkCount := 0

	for chunk, err := range client.TTSV2.Stream(ctx, &req) {
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
		if err := outputBytes(audioBuf.Bytes(), outputPath); err != nil {
			return fmt.Errorf("failed to write audio file: %w", err)
		}
		printSuccess("Audio saved to: %s (%s)", outputPath, formatBytes(int64(audioBuf.Len())))
	}

	result := map[string]any{
		"api_version": "v2",
		"audio_size":  audioBuf.Len(),
		"chunks":      chunkCount,
		"output_file": outputPath,
	}

	return outputResult(result, "", isJSONOutput())
}

func runTTSV2Bidirectional(ctx context.Context, client *ds.Client, cliCtx *cli.Context, outputPath string) error {
	var req ds.TTSV2Request
	if err := loadRequest(getInputFile(), &req); err != nil {
		return err
	}

	if req.Speaker == "" && cliCtx.DefaultVoice != "" {
		req.Speaker = cliCtx.DefaultVoice
	}

	// Default resource ID if not specified
	resourceID := req.ResourceID
	if resourceID == "" {
		resourceID = ds.ResourceTTSV2
	}

	printVerbose("Using V2 API (BigModel WebSocket bidirectional)")
	printVerbose("Speaker: %s", req.Speaker)
	printVerbose("Resource ID: %s", resourceID)
	printVerbose("Text length: %d characters", len(req.Text))

	// Open bidirectional session
	session, err := client.TTSV2.OpenSession(ctx, &ds.TTSV2SessionConfig{
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

	// Send text
	if err := session.SendText(ctx, req.Text, true); err != nil {
		return fmt.Errorf("failed to send text: %w", err)
	}

	// Receive audio
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
		if err := outputBytes(audioBuf.Bytes(), outputPath); err != nil {
			return fmt.Errorf("failed to write audio file: %w", err)
		}
		printSuccess("Audio saved to: %s (%s)", outputPath, formatBytes(int64(audioBuf.Len())))
	}

	result := map[string]any{
		"api_version": "v2-bidirectional",
		"audio_size":  audioBuf.Len(),
		"chunks":      chunkCount,
		"output_file": outputPath,
	}

	return outputResult(result, "", isJSONOutput())
}

// ============================================================================
// Init
// ============================================================================

func init() {
	// V1 subcommands
	ttsV1Cmd.AddCommand(ttsV1SynthesizeCmd)
	ttsV1Cmd.AddCommand(ttsV1StreamCmd)

	// V2 subcommands
	ttsV2Cmd.AddCommand(ttsV2StreamCmd)
	ttsV2Cmd.AddCommand(ttsV2WSCmd)
	ttsV2Cmd.AddCommand(ttsV2BidirectionalCmd)
	ttsV2Cmd.AddCommand(ttsV2AsyncCmd)
	ttsV2Cmd.AddCommand(ttsV2StatusCmd)

	// Add v1 and v2 to tts
	ttsCmd.AddCommand(ttsV1Cmd)
	ttsCmd.AddCommand(ttsV2Cmd)
}
