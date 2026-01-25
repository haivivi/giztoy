package commands

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	ds "github.com/haivivi/giztoy/pkg/doubaospeech"
)

// podcastCmd is the root command for Podcast services
var podcastCmd = &cobra.Command{
	Use:   "podcast",
	Short: "Podcast synthesis service",
	Long: `Podcast synthesis service.

Create multi-speaker podcast audio from scripts.

Supports two API types:
  - http: Async HTTP API (submit and poll)
  - sami: Real-time WebSocket streaming (recommended)

Commands:
  podcast http submit  - Submit async HTTP task
  podcast http status  - Query HTTP task status
  podcast sami         - SAMI Podcast WebSocket streaming

SAMI Podcast requires specific speakers:
  - zh_male_dayixiansheng_v2_saturn_bigtts
  - zh_female_mizaitongxue_v2_saturn_bigtts
  - zh_male_liufei_v2_saturn_bigtts
  - zh_male_xiaolei_v2_saturn_bigtts`,
}

// ============================================================================
// HTTP API Commands
// ============================================================================

var podcastHTTPCmd = &cobra.Command{
	Use:   "http",
	Short: "Podcast HTTP API (async)",
	Long: `Podcast HTTP API for async synthesis.

Submit a podcast task and poll for results.

Example request file (podcast-http.yaml):
  script:
    - speaker_id: zh_male_yangguang
      text: 大家好，欢迎收听今天的节目。
    - speaker_id: zh_female_cancan
      text: 是的，今天我们要聊的话题非常有趣。
  encoding: mp3
  sample_rate: 24000`,
}

var podcastHTTPSubmitCmd = &cobra.Command{
	Use:   "submit",
	Short: "Submit async podcast task",
	Long: `Submit a podcast synthesis task (async).

Returns a task ID for tracking.
Use 'doubaospeech podcast http status <task_id>' to check the task status.

Example:
  doubaospeech podcast http submit -f podcast.yaml`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireInputFile(); err != nil {
			return err
		}

		cliCtx, err := getContext()
		if err != nil {
			return err
		}

		client, err := createClient(cliCtx)
		if err != nil {
			return err
		}

		var req ds.PodcastTaskRequest
		if err := loadRequest(getInputFile(), &req); err != nil {
			return err
		}

		printVerbose("Using HTTP API (async)")
		printVerbose("Script segments: %d", len(req.Script))

		reqCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		task, err := client.Podcast.CreateTask(reqCtx, &req)
		if err != nil {
			return fmt.Errorf("failed to create podcast task: %w", err)
		}

		printSuccess("Podcast task submitted successfully!")
		printInfo("Task ID: %s", task.ID)
		printInfo("Use 'doubaospeech podcast http status %s' to check status", task.ID)

		result := map[string]any{
			"api_type": "http",
			"task_id":  task.ID,
			"status":   "submitted",
		}

		return outputResult(result, getOutputFile(), isJSONOutput())
	},
}

var podcastHTTPStatusCmd = &cobra.Command{
	Use:   "status <task_id>",
	Short: "Query podcast task status",
	Long: `Query the status of a podcast synthesis task.

Example:
  doubaospeech podcast http status task_12345
  doubaospeech podcast http status task_12345 --json`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		taskID := args[0]

		cliCtx, err := getContext()
		if err != nil {
			return err
		}

		client, err := createClient(cliCtx)
		if err != nil {
			return err
		}

		printVerbose("Using context: %s", cliCtx.Name)
		printVerbose("Querying task: %s", taskID)

		reqCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		status, err := client.Podcast.GetTask(reqCtx, taskID)
		if err != nil {
			return fmt.Errorf("failed to query task status: %w", err)
		}

		// Display status
		printInfo("Task ID: %s", status.TaskID)
		printInfo("Status: %s", status.Status)
		if status.Progress > 0 {
			printInfo("Progress: %d%%", status.Progress)
		}
		if status.Result != nil && status.Result.AudioURL != "" {
			printSuccess("Audio URL: %s", status.Result.AudioURL)
			if status.Result.Duration > 0 {
				printInfo("Duration: %ds", status.Result.Duration)
			}
		}

		result := map[string]any{
			"api_type": "http",
			"task_id":  status.TaskID,
			"status":   status.Status,
			"progress": status.Progress,
		}
		if status.Result != nil {
			result["audio_url"] = status.Result.AudioURL
			result["duration"] = status.Result.Duration
		}

		return outputResult(result, getOutputFile(), isJSONOutput())
	},
}

// ============================================================================
// SAMI Podcast WebSocket Command
// ============================================================================

var podcastSAMICmd = &cobra.Command{
	Use:   "sami",
	Short: "SAMI Podcast WebSocket streaming (recommended)",
	Long: `SAMI Podcast real-time WebSocket streaming.

Endpoint: WSS /api/v3/sami/podcasttts
Resource ID: volc.service_type.10050

IMPORTANT: SAMI Podcast requires specific speaker voices with _v2_saturn_bigtts suffix:
  - zh_male_dayixiansheng_v2_saturn_bigtts (大一先生)
  - zh_female_mizaitongxue_v2_saturn_bigtts (米仔同学)
  - zh_male_liufei_v2_saturn_bigtts (刘飞)
  - zh_male_xiaolei_v2_saturn_bigtts (小雷)

Example request file (podcast-sami.yaml):
  action: 0  # 0=summary generation
  input_text: "分析当前大语言模型的发展，包括 GPT、Claude 等模型的最新进展。"
  audio_config:
    format: mp3
    sample_rate: 24000
  speaker_info:
    random_order: true
    speakers:
      - zh_male_dayixiansheng_v2_saturn_bigtts
      - zh_female_mizaitongxue_v2_saturn_bigtts

Example:
  doubaospeech podcast sami -f podcast-sami.yaml -o output.mp3`,
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

		var req ds.PodcastSAMIRequest
		if err := loadRequest(getInputFile(), &req); err != nil {
			return err
		}

		printVerbose("Using SAMI Podcast API (WebSocket streaming)")
		if req.SpeakerInfo != nil {
			printVerbose("Speakers: %v", req.SpeakerInfo.Speakers)
		}
		printVerbose("Input text length: %d characters", len(req.InputText))

		reqCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		return runPodcastSAMI(reqCtx, client, &req, outputPath)
	},
}

func runPodcastSAMI(ctx context.Context, client *ds.Client, req *ds.PodcastSAMIRequest, outputPath string) error {
	session, err := client.Podcast.StreamSAMI(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to open session: %w", err)
	}
	defer session.Close()

	printVerbose("Session opened")

	// Note: SAMI Podcast with action=0 (summary generation) requires LLM processing
	// which can take several minutes. Show progress indicators to user.
	if req.Action == 0 {
		printInfo("📝 Generating podcast summary (this may take 2-5 minutes for LLM processing)...")
	}

	var audioBuf bytes.Buffer
	chunkCount := 0
	roundCount := 0
	startTime := time.Now()
	lastProgressTime := startTime

	for chunk, err := range session.Recv() {
		if err != nil {
			return fmt.Errorf("receive error: %w", err)
		}

		if len(chunk.Audio) > 0 {
			audioBuf.Write(chunk.Audio)
			chunkCount++
			elapsed := time.Since(startTime).Seconds()
			printVerbose("[%.1fs] 🔊 +%d bytes (total: %.1f KB)", elapsed, len(chunk.Audio), float64(audioBuf.Len())/1024)
		}

		// Show progress for long-running operations
		if chunk.Event != "" {
			elapsed := time.Since(startTime).Seconds()
			switch chunk.Event {
			case "SessionStarted":
				printVerbose("[%.1fs] ✅ Session started", elapsed)
			case "PodcastRoundStart":
				roundCount++
				printInfo("[%.1fs] 🎙️ Starting round %d...", elapsed, roundCount)
			case "PodcastRoundEnd":
				printVerbose("[%.1fs] ✅ Round %d completed", elapsed, roundCount)
			case "PodcastEnd":
				printInfo("[%.1fs] 🎉 Podcast generation completed!", elapsed)
			case "SessionFinished":
				printVerbose("[%.1fs] ✅ Session finished", elapsed)
			case "UsageResponse":
				printVerbose("[%.1fs] 📊 Usage info received", elapsed)
			default:
				printVerbose("[%.1fs] Event: %s", elapsed, chunk.Event)
			}
			lastProgressTime = time.Now()
		}

		// Show waiting indicator during long silences (every 30s)
		if time.Since(lastProgressTime) > 30*time.Second && audioBuf.Len() == 0 {
			elapsed := time.Since(startTime).Seconds()
			printInfo("[%.1fs] ⏳ Still waiting for LLM processing...", elapsed)
			lastProgressTime = time.Now()
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
	} else {
		return fmt.Errorf("no audio data received")
	}

	result := map[string]any{
		"api_type":    "sami",
		"audio_size":  audioBuf.Len(),
		"chunks":      chunkCount,
		"duration_s":  time.Since(startTime).Seconds(),
		"output_file": outputPath,
	}

	return outputResult(result, "", isJSONOutput())
}

// ============================================================================
// Init
// ============================================================================

func init() {
	// HTTP subcommands
	podcastHTTPCmd.AddCommand(podcastHTTPSubmitCmd)
	podcastHTTPCmd.AddCommand(podcastHTTPStatusCmd)

	// Add http and sami to podcast
	podcastCmd.AddCommand(podcastHTTPCmd)
	podcastCmd.AddCommand(podcastSAMICmd)
}
