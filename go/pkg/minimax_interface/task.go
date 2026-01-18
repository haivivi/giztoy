package minimax_interface

import (
	"context"
	"time"
)

// TaskStatus 任务状态
type TaskStatus string

const (
	TaskStatusPending    TaskStatus = "Pending"
	TaskStatusProcessing TaskStatus = "Processing"
	TaskStatusSuccess    TaskStatus = "Success"
	TaskStatusFailed     TaskStatus = "Failed"
)

// Task 表示一个异步任务
type Task[T any] struct {
	// ID 任务 ID
	ID string

	// 内部字段，用于查询状态
	// client *Client
}

// Wait 等待任务完成并返回结果
//
// 默认轮询间隔为 5 秒。如果需要自定义间隔，请使用 WaitWithInterval。
// 可以通过 context 控制超时。
//
// 示例:
//
//	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
//	defer cancel()
//	result, err := task.Wait(ctx)
func (t *Task[T]) Wait(ctx context.Context) (*T, error) {
	return t.WaitWithInterval(ctx, 5*time.Second)
}

// WaitWithInterval 以指定间隔轮询等待任务完成
func (t *Task[T]) WaitWithInterval(ctx context.Context, interval time.Duration) (*T, error) {
	// TODO: 实现
	panic("not implemented")
}

// Status 获取任务当前状态（不阻塞）
func (t *Task[T]) Status(ctx context.Context) (TaskStatus, error) {
	// TODO: 实现
	panic("not implemented")
}

// VideoResult 视频生成任务结果
type VideoResult struct {
	// FileID 生成的视频文件 ID
	FileID string `json:"file_id"`

	// DownloadURL 视频下载 URL（Video Agent 任务）
	DownloadURL string `json:"download_url,omitempty"`
}

// SpeechAsyncResult 异步语音合成任务结果
type SpeechAsyncResult struct {
	// FileID 生成的音频文件 ID
	FileID string `json:"file_id"`

	// AudioInfo 音频信息
	AudioInfo *AudioInfo `json:"extra_info"`

	// Subtitle 字幕信息（如果启用）
	Subtitle *Subtitle `json:"subtitle,omitempty"`
}

// Subtitle 字幕信息
type Subtitle struct {
	Segments []SubtitleSegment `json:"segments"`
}

// SubtitleSegment 字幕片段
type SubtitleSegment struct {
	StartTime int    `json:"start_time"` // 毫秒
	EndTime   int    `json:"end_time"`   // 毫秒
	Text      string `json:"text"`
}
