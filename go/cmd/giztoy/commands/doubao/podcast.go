package doubao

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	ds "github.com/haivivi/giztoy/go/pkg/doubaospeech"
)

var podcastCmd = &cobra.Command{
	Use:   "podcast",
	Short: "Podcast synthesis service",
	Long: `Multi-speaker podcast audio synthesis.

Supports two modes:
  http  Async HTTP API (submit and poll)
  sami  Real-time WebSocket streaming (recommended)`,
}

// ============================================================================
// HTTP API
// ============================================================================

var podcastHTTPCmd = &cobra.Command{
	Use:   "http",
	Short: "Podcast HTTP API (async)",
}

var podcastHTTPSubmitCmd = &cobra.Command{
	Use:   "submit",
	Short: "Submit async podcast task",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireInputFile(); err != nil {
			return err
		}

		client, err := createClient()
		if err != nil {
			return err
		}

		var req ds.PodcastTaskRequest
		if err := loadRequest(inputFile, &req); err != nil {
			return err
		}

		printVerbose("Script segments: %d", len(req.Script))

		reqCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		task, err := client.Podcast.CreateTask(reqCtx, &req)
		if err != nil {
			return fmt.Errorf("failed to create podcast task: %w", err)
		}

		printSuccess("Podcast task submitted!")
		printInfo("Task ID: %s", task.ID)

		return outputResult(map[string]any{
			"api_type": "http",
			"task_id":  task.ID,
			"status":   "submitted",
		}, outputFile, outputJSON)
	},
}

var podcastHTTPStatusCmd = &cobra.Command{
	Use:   "status <task_id>",
	Short: "Query podcast task status",
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

		status, err := client.Podcast.GetTask(reqCtx, taskID)
		if err != nil {
			return fmt.Errorf("query task status failed: %w", err)
		}

		printInfo("Task ID: %s", status.TaskID)
		printInfo("Status: %s", status.Status)

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
		return outputResult(result, outputFile, outputJSON)
	},
}

// ============================================================================
// SAMI Podcast WebSocket
// ============================================================================

var podcastSAMICmd = &cobra.Command{
	Use:   "sami",
	Short: "SAMI Podcast WebSocket streaming (recommended)",
	Long: `SAMI Podcast real-time WebSocket streaming.

IMPORTANT: Requires specific speaker voices with _v2_saturn_bigtts suffix.

Example request file (podcast-sami.yaml):
  action: 0
  input_text: "分析当前大语言模型的发展"
  audio_config:
    format: mp3
    sample_rate: 24000
  speaker_info:
    random_order: true
    speakers:
      - zh_male_dayixiansheng_v2_saturn_bigtts
      - zh_female_mizaitongxue_v2_saturn_bigtts

Example:
  giztoy doubao podcast sami -f podcast-sami.yaml -o output.mp3`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireInputFile(); err != nil {
			return err
		}
		if outputFile == "" {
			return fmt.Errorf("output file is required, use -o flag")
		}

		client, err := createClient()
		if err != nil {
			return err
		}

		var req ds.PodcastSAMIRequest
		if err := loadRequest(inputFile, &req); err != nil {
			return err
		}

		printVerbose("Using SAMI Podcast API (WebSocket streaming)")
		if req.SpeakerInfo != nil {
			printVerbose("Speakers: %v", req.SpeakerInfo.Speakers)
		}

		reqCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		session, err := client.Podcast.StreamSAMI(reqCtx, &req)
		if err != nil {
			return fmt.Errorf("failed to open session: %w", err)
		}
		defer session.Close()

		printVerbose("Session opened")

		if req.Action == 0 {
			printInfo("Generating podcast summary (this may take 2-5 minutes for LLM processing)...")
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
				printVerbose("[%.1fs] +%d bytes (total: %.1f KB)",
					time.Since(startTime).Seconds(), len(chunk.Audio), float64(audioBuf.Len())/1024)
			}

			if chunk.Event != "" {
				elapsed := time.Since(startTime).Seconds()
				switch chunk.Event {
				case "PodcastRoundStart":
					roundCount++
					printInfo("[%.1fs] Starting round %d...", elapsed, roundCount)
				case "PodcastEnd":
					printInfo("[%.1fs] Podcast generation completed!", elapsed)
				default:
					printVerbose("[%.1fs] Event: %s", elapsed, chunk.Event)
				}
				lastProgressTime = time.Now()
			}

			if time.Since(lastProgressTime) > 30*time.Second && audioBuf.Len() == 0 {
				printInfo("[%.1fs] Still waiting for LLM processing...", time.Since(startTime).Seconds())
				lastProgressTime = time.Now()
			}

			if chunk.IsLast {
				break
			}
		}

		if audioBuf.Len() == 0 {
			return fmt.Errorf("no audio data received")
		}

		if err := outputBytes(audioBuf.Bytes(), outputFile); err != nil {
			return fmt.Errorf("failed to write audio: %w", err)
		}
		printSuccess("Audio saved to: %s (%s)", outputFile, formatBytes(int64(audioBuf.Len())))

		return outputResult(map[string]any{
			"api_type":    "sami",
			"audio_size":  audioBuf.Len(),
			"chunks":      chunkCount,
			"duration_s":  time.Since(startTime).Seconds(),
			"output_file": outputFile,
		}, "", outputJSON)
	},
}

func init() {
	podcastHTTPCmd.AddCommand(podcastHTTPSubmitCmd)
	podcastHTTPCmd.AddCommand(podcastHTTPStatusCmd)

	podcastCmd.AddCommand(podcastHTTPCmd)
	podcastCmd.AddCommand(podcastSAMICmd)
}
