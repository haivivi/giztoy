package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	mm "github.com/haivivi/giztoy/pkg/minimax_interface"
)

var fileCmd = &cobra.Command{
	Use:   "file",
	Short: "File management service",
	Long: `File management service.

Upload, download, list, and delete files.

Supported file purposes:
  - voice_clone: Audio for voice cloning
  - voice_clone_demo: Demo audio for voice cloning
  - t2a_async: Text file for async TTS
  - fine-tune: Fine-tuning data
  - assistants: Assistant files`,
}

var fileListCmd = &cobra.Command{
	Use:   "list",
	Short: "List uploaded files",
	Long: `List all uploaded files.

Examples:
  minimax -c myctx file list
  minimax -c myctx file list --purpose voice_clone
  minimax -c myctx file list --json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := getContext()
		if err != nil {
			return err
		}

		purpose, err := cmd.Flags().GetString("purpose")
		if err != nil {
			return fmt.Errorf("failed to read 'purpose' flag: %w", err)
		}
		limit, err := cmd.Flags().GetInt("limit")
		if err != nil {
			return fmt.Errorf("failed to read 'limit' flag: %w", err)
		}

		printVerbose("Using context: %s", ctx.Name)
		if purpose != "" {
			printVerbose("Purpose filter: %s", purpose)
		}

		// TODO: Implement actual API call
		result := map[string]any{
			"_note":    "API call not implemented yet",
			"_context": ctx.Name,
			"purpose":  purpose,
			"limit":    limit,
			"files": []map[string]any{
				{"file_id": "file-123", "filename": "audio.mp3", "purpose": "voice_clone"},
			},
		}

		return outputResult(result, getOutputFile(), isJSONOutput())
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

		// Validate file exists
		info, err := os.Stat(filePath)
		if err != nil {
			return fmt.Errorf("cannot access file: %w", err)
		}

		printVerbose("Using context: %s", ctx.Name)
		printVerbose("File: %s (%s)", filePath, formatBytes(int(info.Size())))
		printVerbose("Purpose: %s", purpose)

		// TODO: Implement actual API call
		result := map[string]any{
			"_note":    "API call not implemented yet",
			"_context": ctx.Name,
			"file":     filePath,
			"purpose":  purpose,
			"size":     info.Size(),
			"file_id":  "placeholder-file-id",
		}

		return outputResult(result, getOutputFile(), isJSONOutput())
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

		// TODO: Implement actual API call
		fmt.Printf("[Not implemented] Would download file %s to %s\n", fileID, outputPath)

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

		// TODO: Implement actual API call
		result := map[string]any{
			"_note":    "API call not implemented yet",
			"_context": ctx.Name,
			"file_id":  fileID,
			"file": mm.FileInfo{
				FileID:    fileID,
				Filename:  "example.mp3",
				Bytes:     12345,
				CreatedAt: 1705555555,
				Purpose:   "voice_clone",
			},
		}

		return outputResult(result, getOutputFile(), isJSONOutput())
	},
}

var fileDeleteCmd = &cobra.Command{
	Use:   "delete <file_id>",
	Short: "Delete a file",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fileID := args[0]

		ctx, err := getContext()
		if err != nil {
			return err
		}

		printVerbose("Using context: %s", ctx.Name)
		printVerbose("Deleting file: %s", fileID)

		// TODO: Implement actual API call
		fmt.Printf("[Not implemented] Would delete file: %s\n", fileID)

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

		// TODO: Implement actual API calls
		result := map[string]any{
			"_note":    "API call not implemented yet",
			"_context": ctx.Name,
			"files":    files,
			"purpose":  purpose,
		}

		return outputResult(result, getOutputFile(), isJSONOutput())
	},
}

func init() {
	fileListCmd.Flags().String("purpose", "", "Filter by purpose")
	fileListCmd.Flags().Int("limit", 100, "Maximum number of files to return")

	fileUploadCmd.Flags().String("purpose", "", "File purpose (required)")

	fileUploadLocalCmd.Flags().String("purpose", "", "File purpose (required)")
	fileUploadLocalCmd.Flags().String("ext", "", "File extension filter (e.g. .mp3)")

	fileCmd.AddCommand(fileListCmd)
	fileCmd.AddCommand(fileUploadCmd)
	fileCmd.AddCommand(fileDownloadCmd)
	fileCmd.AddCommand(fileGetCmd)
	fileCmd.AddCommand(fileDeleteCmd)
	fileCmd.AddCommand(fileUploadLocalCmd)
}
