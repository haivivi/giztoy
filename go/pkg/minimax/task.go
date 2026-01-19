package minimax

import (
	"context"
	"fmt"
	"net/url"
	"time"
)

// taskType identifies the type of async task for polling.
type taskType int

const (
	taskTypeVideo taskType = iota
	taskTypeSpeechAsync
)

// Task represents an async operation that can be polled for completion.
type Task[T any] struct {
	// ID is the task identifier.
	ID string

	client   *Client
	taskType taskType
}

// NewVideoTask creates a Task for querying an existing video generation task.
func (c *Client) NewVideoTask(taskID string) *Task[VideoResult] {
	return &Task[VideoResult]{
		ID:       taskID,
		client:   c,
		taskType: taskTypeVideo,
	}
}

// NewSpeechAsyncTask creates a Task for querying an existing async speech task.
func (c *Client) NewSpeechAsyncTask(taskID string) *Task[SpeechAsyncResult] {
	return &Task[SpeechAsyncResult]{
		ID:       taskID,
		client:   c,
		taskType: taskTypeSpeechAsync,
	}
}

// Wait waits for the task to complete and returns the result.
//
// Uses a default polling interval of 5 seconds. Use WaitWithInterval
// for custom intervals.
//
// Example:
//
//	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
//	defer cancel()
//	result, err := task.Wait(ctx)
func (t *Task[T]) Wait(ctx context.Context) (*T, error) {
	return t.WaitWithInterval(ctx, 5*time.Second)
}

// WaitWithInterval waits for the task to complete with a custom polling interval.
func (t *Task[T]) WaitWithInterval(ctx context.Context, interval time.Duration) (*T, error) {
	// Query immediately before first ticker interval
	result, status, err := t.query(ctx)
	if err != nil {
		return nil, err
	}
	switch status {
	case TaskStatusSuccess:
		return result, nil
	case TaskStatusFailed:
		return nil, fmt.Errorf("task %s failed with status %s", t.ID, status)
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			result, status, err := t.query(ctx)
			if err != nil {
				return nil, err
			}

			switch status {
			case TaskStatusSuccess:
				return result, nil
			case TaskStatusFailed:
				return nil, fmt.Errorf("task %s failed with status %s", t.ID, status)
			case TaskStatusPending, TaskStatusQueueing, TaskStatusPreparing, TaskStatusProcessing:
				// Continue waiting
			default:
				return nil, fmt.Errorf("unknown task status: %s", status)
			}
		}
	}
}

// Status returns the current task status without blocking.
func (t *Task[T]) Status(ctx context.Context) (TaskStatus, error) {
	_, status, err := t.query(ctx)
	return status, err
}

// query queries the task status and result.
func (t *Task[T]) query(ctx context.Context) (*T, TaskStatus, error) {
	switch t.taskType {
	case taskTypeVideo:
		return t.queryVideoTask(ctx)
	case taskTypeSpeechAsync:
		return t.querySpeechAsyncTask(ctx)
	default:
		return nil, "", fmt.Errorf("unknown task type")
	}
}

// queryVideoTask queries a video generation task.
func (t *Task[T]) queryVideoTask(ctx context.Context) (*T, TaskStatus, error) {
	var resp struct {
		TaskID      string     `json:"task_id"`
		Status      TaskStatus `json:"status"`
		FileID      string     `json:"file_id,omitempty"`
		VideoWidth  int        `json:"video_width,omitempty"`
		VideoHeight int        `json:"video_height,omitempty"`
		BaseResp    *baseResp  `json:"base_resp,omitempty"`
	}

	// Use query parameter instead of path parameter
	err := t.client.http.request(ctx, "GET", "/v1/query/video_generation?task_id="+url.QueryEscape(t.ID), nil, &resp)
	if err != nil {
		return nil, "", err
	}

	if resp.Status == TaskStatusSuccess {
		// Get download URL from file retrieve API
		downloadURL := ""
		if resp.FileID != "" {
			var fileResp struct {
				File struct {
					DownloadURL string `json:"download_url"`
				} `json:"file"`
				BaseResp *baseResp `json:"base_resp,omitempty"`
			}
			fileErr := t.client.http.request(ctx, "GET", "/v1/files/retrieve?file_id="+url.QueryEscape(resp.FileID), nil, &fileResp)
			if fileErr == nil {
				downloadURL = fileResp.File.DownloadURL
			}
		}

		result := any(&VideoResult{
			FileID:      resp.FileID,
			DownloadURL: downloadURL,
			VideoWidth:  resp.VideoWidth,
			VideoHeight: resp.VideoHeight,
		})
		return result.(*T), resp.Status, nil
	}

	return nil, resp.Status, nil
}

// querySpeechAsyncTask queries an async speech task.
func (t *Task[T]) querySpeechAsyncTask(ctx context.Context) (*T, TaskStatus, error) {
	var resp struct {
		TaskID    int64      `json:"task_id"`
		Status    TaskStatus `json:"status"`
		FileID    int64      `json:"file_id,omitempty"`
		ExtraInfo *AudioInfo `json:"extra_info,omitempty"`
		Subtitle  *Subtitle  `json:"subtitle,omitempty"`
		BaseResp  *baseResp  `json:"base_resp,omitempty"`
	}

	// Use query parameter: /v1/query/t2a_async_query_v2?task_id=xxx
	err := t.client.http.request(ctx, "GET", "/v1/query/t2a_async_query_v2?task_id="+url.QueryEscape(t.ID), nil, &resp)
	if err != nil {
		return nil, "", err
	}

	if resp.Status == TaskStatusSuccess {
		result := any(&SpeechAsyncResult{
			FileID:    fmt.Sprintf("%d", resp.FileID),
			AudioInfo: resp.ExtraInfo,
			Subtitle:  resp.Subtitle,
		})
		return result.(*T), resp.Status, nil
	}

	return nil, resp.Status, nil
}
