package doubao_speech_interface

import (
	"context"
	"time"
)

// TaskStatus 任务状态
type TaskStatus string

const (
	TaskStatusPending    TaskStatus = "pending"
	TaskStatusProcessing TaskStatus = "processing"
	TaskStatusSuccess    TaskStatus = "success"
	TaskStatusFailed     TaskStatus = "failed"
	TaskStatusCancelled  TaskStatus = "cancelled"
)

// Task 异步任务泛型类型
type Task[T any] struct {
	// ID 任务 ID
	ID string

	// client 内部引用，用于轮询
	client *Client
}

// Wait 等待任务完成
//
// 使用默认轮询间隔（5秒）等待任务完成
func (t *Task[T]) Wait(ctx context.Context) (*T, error) {
	return t.WaitWithInterval(ctx, 5*time.Second)
}

// WaitWithInterval 按指定间隔轮询等待任务完成
func (t *Task[T]) WaitWithInterval(ctx context.Context, interval time.Duration) (*T, error) {
	// 实现略
	return nil, nil
}

// Status 查询任务状态
func (t *Task[T]) Status(ctx context.Context) (TaskStatus, error) {
	// 实现略
	return "", nil
}
