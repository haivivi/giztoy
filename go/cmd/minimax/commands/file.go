package commands

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/haivivi/giztoy/go/pkg/minimax"
)

var fileCmd = &cobra.Command{
	Use:   "file",
	Short: "File management service",
	Long: `File management service.

Upload, download, list, and delete files.

Supported file purposes:
  - voice_clone: Audio for voice cloning
  - prompt_audio: Demo/prompt audio for voice cloning
  - t2a_async_input: Text file for async TTS`,
}

var fileListCmd = &cobra.Command{
	Use:   "list",
	Short: "List uploaded files",
	Long: `List uploaded files by purpose category.

The --purpose flag is required. Valid values:
  - voice_clone: Voice cloning source audio files
  - prompt_audio: Voice cloning example/prompt audio files
  - t2a_async_input: Async TTS input files

Examples:
  minimax -c myctx file list --purpose voice_clone
  minimax -c myctx file list --purpose t2a_async_input --json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := getContext()
		if err != nil {
			return err
		}

		purpose, err := cmd.Flags().GetString("purpose")
		if err != nil {
			return fmt.Errorf("failed to read 'purpose' flag: %w", err)
		}
		if purpose == "" {
			return fmt.Errorf("--purpose is required (voice_clone, prompt_audio, or t2a_async_input)")
		}

		printVerbose("Using context: %s", ctx.Name)
		printVerbose("Purpose: %s", purpose)

		// Create API client
		client := createClient(ctx)

		// Call API
		reqCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		resp, err := client.File.List(reqCtx, minimax.FilePurpose(purpose))
		if err != nil {
			return fmt.Errorf("list files failed: %w", err)
		}

		return outputResult(resp, getOutputFile(), isJSONOutput())
	},
}

var fileUploadCmd = &cobra.Command{
	Use:   "upload <file_path>",
	Short: "Upload a file",
	Long: `Upload a file for use with other APIs.

Examples:
  minimax -c myctx file upload audio.mp3 --purpose voice_clone
  minimax -c myctx file upload text.txt --purpose t2a_async --json`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		filePath := args[0]

		ctx, err := getContext()
		if err != nil {
			return err
		}

		purpose, err := cmd.Flags().GetString("purpose")
		if err != nil {
			return fmt.Errorf("failed to read 'purpose' flag: %w", err)
		}
		if purpose == "" {
			return fmt.Errorf("--purpose is required")
		}

		// Open file
		file, err := os.Open(filePath)
		if err != nil {
			return fmt.Errorf("cannot open file: %w", err)
		}
		defer file.Close()

		info, err := file.Stat()
		if err != nil {
			return fmt.Errorf("cannot stat file: %w", err)
		}

		printVerbose("Using context: %s", ctx.Name)
		printVerbose("File: %s (%s)", filePath, formatBytes(int(info.Size())))
		printVerbose("Purpose: %s", purpose)

		// Create API client
		client := createClient(ctx)

		// Call API
		reqCtx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
		defer cancel()

		resp, err := client.File.Upload(reqCtx, file, info.Name(), minimax.FilePurpose(purpose))
		if err != nil {
			return fmt.Errorf("upload failed: %w", err)
		}

		printSuccess("File uploaded: %s", resp.FileID)

		return outputResult(resp, getOutputFile(), isJSONOutput())
	},
}

var fileDownloadCmd = &cobra.Command{
	Use:   "download <file_id>",
	Short: "Download a file",
	Long: `Download a file by its ID.

Examples:
  minimax -c myctx file download file-123 -o downloaded.mp3`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fileID := args[0]

		outputPath := getOutputFile()
		if outputPath == "" {
			return fmt.Errorf("output file is required, use -o flag")
		}

		ctx, err := getContext()
		if err != nil {
			return err
		}

		printVerbose("Using context: %s", ctx.Name)
		printVerbose("File ID: %s", fileID)
		printVerbose("Output: %s", outputPath)

		// Create API client
		client := createClient(ctx)

		// Call API
		reqCtx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
		defer cancel()

		reader, err := client.File.Download(reqCtx, fileID)
		if err != nil {
			return fmt.Errorf("download failed: %w", err)
		}
		defer reader.Close()

		// Create output file
		outFile, err := os.Create(outputPath)
		if err != nil {
			return fmt.Errorf("failed to create output file: %w", err)
		}
		defer outFile.Close()

		// Copy data
		n, err := io.Copy(outFile, reader)
		if err != nil {
			return fmt.Errorf("failed to write file: %w", err)
		}

		printSuccess("Downloaded %s to %s", formatBytes(int(n)), outputPath)

		return nil
	},
}

var fileGetCmd = &cobra.Command{
	Use:   "get <file_id>",
	Short: "Get file information",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fileID := args[0]

		ctx, err := getContext()
		if err != nil {
			return err
		}

		printVerbose("Using context: %s", ctx.Name)
		printVerbose("File ID: %s", fileID)

		// Create API client
		client := createClient(ctx)

		// Call API
		reqCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		resp, err := client.File.Get(reqCtx, fileID)
		if err != nil {
			return fmt.Errorf("get file failed: %w", err)
		}

		return outputResult(resp, getOutputFile(), isJSONOutput())
	},
}

var fileDeleteCmd = &cobra.Command{
	Use:   "delete <file_id>",
	Short: "Delete a file",
	Long: `Delete a file by its ID.

The --purpose flag must match the purpose used when uploading the file.

Examples:
  minimax -c myctx file delete 123456 --purpose voice_clone
  minimax -c myctx file delete 123456 --purpose prompt_audio`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fileID := args[0]

		ctx, err := getContext()
		if err != nil {
			return err
		}

		purpose, err := cmd.Flags().GetString("purpose")
		if err != nil {
			return fmt.Errorf("failed to read 'purpose' flag: %w", err)
		}
		if purpose == "" {
			return fmt.Errorf("--purpose is required")
		}

		printVerbose("Using context: %s", ctx.Name)
		printVerbose("Deleting file: %s (purpose: %s)", fileID, purpose)

		// Create API client
		client := createClient(ctx)

		// Call API
		reqCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := client.File.Delete(reqCtx, fileID, minimax.FilePurpose(purpose)); err != nil {
			return fmt.Errorf("delete file failed: %w", err)
		}

		printSuccess("File deleted: %s", fileID)

		return nil
	},
}

var fileUploadLocalCmd = &cobra.Command{
	Use:   "upload-dir <directory>",
	Short: "Upload all files in a directory",
	Long: `Upload all files in a directory with the specified purpose.

Examples:
  minimax -c myctx file upload-dir ./audio-files --purpose voice_clone --ext .mp3`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dirPath := args[0]

		ctx, err := getContext()
		if err != nil {
			return err
		}

		purpose, err := cmd.Flags().GetString("purpose")
		if err != nil {
			return fmt.Errorf("failed to read 'purpose' flag: %w", err)
		}
		if purpose == "" {
			return fmt.Errorf("--purpose is required")
		}

		ext, err := cmd.Flags().GetString("ext")
		if err != nil {
			return fmt.Errorf("failed to read 'ext' flag: %w", err)
		}

		// Find files
		var files []string
		err = filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() {
				if ext == "" || filepath.Ext(path) == ext {
					files = append(files, path)
				}
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("failed to scan directory: %w", err)
		}

		printVerbose("Using context: %s", ctx.Name)
		printVerbose("Directory: %s", dirPath)
		printVerbose("Files found: %d", len(files))

		// Create API client
		client := createClient(ctx)

		// Upload files
		var results []map[string]any
		for _, filePath := range files {
			file, err := os.Open(filePath)
			if err != nil {
				printError("Failed to open %s: %v", filePath, err)
				continue
			}

			info, _ := file.Stat()
			printInfo("Uploading: %s (%s)", filePath, formatBytes(int(info.Size())))

			reqCtx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
			resp, err := client.File.Upload(reqCtx, file, info.Name(), minimax.FilePurpose(purpose))
			cancel()
			file.Close()

			if err != nil {
				printError("Failed to upload %s: %v", filePath, err)
				continue
			}

			results = append(results, map[string]any{
				"file":    filePath,
				"file_id": resp.FileID,
			})
			printSuccess("Uploaded: %s -> %s", filePath, resp.FileID)
		}

		return outputResult(map[string]any{
			"uploaded": len(results),
			"total":    len(files),
			"files":    results,
		}, getOutputFile(), isJSONOutput())
	},
}

func init() {
	fileListCmd.Flags().String("purpose", "", "File purpose (required): voice_clone, prompt_audio, t2a_async_input")

	fileUploadCmd.Flags().String("purpose", "", "File purpose (required)")

	fileDeleteCmd.Flags().String("purpose", "", "File purpose (required): voice_clone, prompt_audio, t2a_async_input")

	fileUploadLocalCmd.Flags().String("purpose", "", "File purpose (required)")
	fileUploadLocalCmd.Flags().String("ext", "", "File extension filter (e.g. .mp3)")

	fileCmd.AddCommand(fileListCmd)
	fileCmd.AddCommand(fileUploadCmd)
	fileCmd.AddCommand(fileDownloadCmd)
	fileCmd.AddCommand(fileGetCmd)
	fileCmd.AddCommand(fileDeleteCmd)
	fileCmd.AddCommand(fileUploadLocalCmd)
}
