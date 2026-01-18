package minimax_interface

import (
	"context"
	"io"
)

// FileService 文件管理服务接口
type FileService interface {
	// Upload 上传文件
	Upload(ctx context.Context, file io.Reader, filename string, purpose FilePurpose) (*FileInfo, error)

	// List 获取文件列表
	List(ctx context.Context, opts *FileListOptions) (*FileListResponse, error)

	// Get 获取单个文件信息
	Get(ctx context.Context, fileID string) (*FileInfo, error)

	// Download 下载文件内容
	//
	// 返回的 io.ReadCloser 需要调用方关闭。
	Download(ctx context.Context, fileID string) (io.ReadCloser, error)

	// Delete 删除文件
	Delete(ctx context.Context, fileID string) error
}

// FilePurpose 文件用途
type FilePurpose string

const (
	FilePurposeVoiceClone     FilePurpose = "voice_clone"
	FilePurposeVoiceCloneDemo FilePurpose = "voice_clone_demo"
	FilePurposeT2AAsync       FilePurpose = "t2a_async"
	FilePurposeFineTune       FilePurpose = "fine-tune"
	FilePurposeAssistants     FilePurpose = "assistants"
)

// FileInfo 文件信息
type FileInfo struct {
	// FileID 文件 ID
	FileID string `json:"file_id"`

	// Filename 文件名
	Filename string `json:"filename"`

	// Bytes 文件大小（字节）
	Bytes int64 `json:"bytes"`

	// CreatedAt 创建时间（Unix 时间戳）
	CreatedAt int64 `json:"created_at"`

	// Purpose 文件用途
	Purpose string `json:"purpose"`

	// Status 文件状态
	Status string `json:"status,omitempty"`
}

// FileListOptions 文件列表查询选项
type FileListOptions struct {
	// Purpose 按用途筛选
	Purpose FilePurpose `json:"purpose,omitempty"`

	// Limit 返回数量限制
	Limit int `json:"limit,omitempty"`

	// After 分页游标
	After string `json:"after,omitempty"`
}

// FileListResponse 文件列表响应
type FileListResponse struct {
	// Data 文件列表
	Data []FileInfo `json:"data"`

	// HasMore 是否有更多
	HasMore bool `json:"has_more"`
}
