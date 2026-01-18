package doubao_speech_interface

import (
	"context"
	"time"
)

// ConsoleService 控制台管理服务接口
//
// 控制台接口使用火山引擎 OpenAPI 标准鉴权，与语音 API 的鉴权方式不同。
// 详见：https://www.volcengine.com/docs/6369/65269
type ConsoleService interface {
	// Timbre 音色管理
	Timbre() TimbreService

	// APIKey API Key 管理
	APIKey() APIKeyService

	// Service 服务管理
	Service() ServiceManageService

	// Monitoring 监控
	Monitoring() MonitoringService

	// VoiceCloneManage 声音复刻管理（控制台）
	VoiceCloneManage() VoiceCloneManageService
}

// ================== 音色管理 ==================

// TimbreService 音色管理服务
type TimbreService interface {
	// ListBigModelTTSTimbres 获取大模型音色列表
	ListBigModelTTSTimbres(ctx context.Context, req *ListTimbresRequest) (*ListTimbresResponse, error)

	// ListSpeakers 获取音色列表（新接口，推荐使用）
	ListSpeakers(ctx context.Context, req *ListSpeakersRequest) (*ListSpeakersResponse, error)
}

// ListTimbresRequest 获取音色列表请求
type ListTimbresRequest struct {
	// PageNumber 页码，默认 1
	PageNumber int `json:"PageNumber,omitempty"`

	// PageSize 每页数量，默认 20
	PageSize int `json:"PageSize,omitempty"`

	// TimbreType 音色类型筛选
	TimbreType string `json:"TimbreType,omitempty"`
}

// ListTimbresResponse 获取音色列表响应
type ListTimbresResponse struct {
	// Total 总数
	Total int `json:"Total"`

	// Timbres 音色列表
	Timbres []TimbreInfo `json:"Timbres"`
}

// TimbreInfo 音色信息
type TimbreInfo struct {
	// TimbreId 音色 ID
	TimbreId string `json:"TimbreId"`

	// TimbreName 音色名称
	TimbreName string `json:"TimbreName"`

	// Language 语言
	Language string `json:"Language"`

	// Gender 性别：male/female
	Gender string `json:"Gender"`

	// Description 描述
	Description string `json:"Description,omitempty"`
}

// ListSpeakersRequest 获取音色列表请求（新接口）
type ListSpeakersRequest struct {
	// PageNumber 页码
	PageNumber int `json:"PageNumber,omitempty"`

	// PageSize 每页数量
	PageSize int `json:"PageSize,omitempty"`

	// SpeakerType 音色类型
	SpeakerType string `json:"SpeakerType,omitempty"`

	// Language 语言筛选
	Language string `json:"Language,omitempty"`
}

// ListSpeakersResponse 获取音色列表响应（新接口）
type ListSpeakersResponse struct {
	// Total 总数
	Total int `json:"Total"`

	// Speakers 音色列表
	Speakers []SpeakerInfo `json:"Speakers"`
}

// SpeakerInfo 音色信息（新接口）
type SpeakerInfo struct {
	// SpeakerId 音色 ID
	SpeakerId string `json:"SpeakerId"`

	// SpeakerName 音色名称
	SpeakerName string `json:"SpeakerName"`

	// Language 语言
	Language string `json:"Language"`

	// Gender 性别
	Gender string `json:"Gender"`

	// SampleAudioUrl 示例音频 URL
	SampleAudioUrl string `json:"SampleAudioUrl,omitempty"`
}

// ================== API Key 管理 ==================

// APIKeyService API Key 管理服务
type APIKeyService interface {
	// List 获取 API Key 列表
	List(ctx context.Context) (*ListAPIKeysResponse, error)

	// Create 创建 API Key
	Create(ctx context.Context, req *CreateAPIKeyRequest) (*CreateAPIKeyResponse, error)

	// Update 更新 API Key
	Update(ctx context.Context, req *UpdateAPIKeyRequest) error

	// Delete 删除 API Key
	Delete(ctx context.Context, apiKeyID string) error
}

// ListAPIKeysResponse 获取 API Key 列表响应
type ListAPIKeysResponse struct {
	// APIKeys API Key 列表
	APIKeys []APIKeyInfo `json:"APIKeys"`
}

// APIKeyInfo API Key 信息
type APIKeyInfo struct {
	// APIKeyId API Key ID
	APIKeyId string `json:"APIKeyId"`

	// Name 名称
	Name string `json:"Name"`

	// Status 状态：active/inactive
	Status string `json:"Status"`

	// Description 描述
	Description string `json:"Description,omitempty"`

	// CreatedAt 创建时间
	CreatedAt time.Time `json:"CreatedAt"`

	// ExpiredAt 过期时间
	ExpiredAt time.Time `json:"ExpiredAt,omitempty"`
}

// CreateAPIKeyRequest 创建 API Key 请求
type CreateAPIKeyRequest struct {
	// Name API Key 名称（必填）
	Name string `json:"Name"`

	// ExpiredAt 过期时间
	ExpiredAt time.Time `json:"ExpiredAt,omitempty"`

	// Description 描述
	Description string `json:"Description,omitempty"`
}

// CreateAPIKeyResponse 创建 API Key 响应
type CreateAPIKeyResponse struct {
	// APIKeyId API Key ID
	APIKeyId string `json:"APIKeyId"`

	// APIKeySecret API Key Secret（仅创建时返回一次）
	APIKeySecret string `json:"APIKeySecret"`

	// Name 名称
	Name string `json:"Name"`
}

// UpdateAPIKeyRequest 更新 API Key 请求
type UpdateAPIKeyRequest struct {
	// APIKeyId API Key ID（必填）
	APIKeyId string `json:"APIKeyId"`

	// Name 新名称
	Name string `json:"Name,omitempty"`

	// Status 状态：active/inactive
	Status string `json:"Status,omitempty"`

	// ExpiredAt 新过期时间
	ExpiredAt time.Time `json:"ExpiredAt,omitempty"`
}

// ================== 服务管理 ==================

// ServiceManageService 服务管理服务
type ServiceManageService interface {
	// Status 查询服务状态
	Status(ctx context.Context) (*ServiceStatusResponse, error)

	// Activate 开通服务
	Activate(ctx context.Context, serviceID string) error

	// Pause 暂停服务
	Pause(ctx context.Context, serviceID string) error

	// Resume 恢复服务
	Resume(ctx context.Context, serviceID string) error

	// Terminate 停用服务（不可逆）
	Terminate(ctx context.Context, serviceID string) error
}

// ServiceStatusResponse 服务状态响应
type ServiceStatusResponse struct {
	// Status 整体状态
	Status ServiceState `json:"Status"`

	// ActivatedAt 开通时间
	ActivatedAt time.Time `json:"ActivatedAt,omitempty"`

	// Services 各服务状态
	Services []ServiceInfo `json:"Services"`
}

// ServiceState 服务状态枚举
type ServiceState string

const (
	ServiceStateActive     ServiceState = "active"     // 正常运行
	ServiceStatePaused     ServiceState = "paused"     // 已暂停
	ServiceStateTerminated ServiceState = "terminated" // 已停用
	ServiceStatePending    ServiceState = "pending"    // 待开通
)

// ServiceInfo 服务信息
type ServiceInfo struct {
	// ServiceId 服务 ID
	ServiceId string `json:"ServiceId"`

	// ServiceName 服务名称
	ServiceName string `json:"ServiceName"`

	// Status 状态
	Status ServiceState `json:"Status"`
}

// ================== 监控 ==================

// MonitoringService 监控服务
type MonitoringService interface {
	// GetQuota 查询配额信息
	GetQuota(ctx context.Context, serviceID string) (*QuotaResponse, error)

	// GetUsage 查询调用量统计
	GetUsage(ctx context.Context, req *UsageRequest) (*UsageResponse, error)

	// GetQPS 实时查询 QPS 和并发
	GetQPS(ctx context.Context) (*QPSResponse, error)
}

// QuotaResponse 配额响应
type QuotaResponse struct {
	// Quotas 配额列表
	Quotas []QuotaInfo `json:"Quotas"`
}

// QuotaInfo 配额信息
type QuotaInfo struct {
	// ServiceId 服务 ID
	ServiceId string `json:"ServiceId"`

	// ServiceName 服务名称
	ServiceName string `json:"ServiceName"`

	// QPS QPS 配额
	QPS QuotaLimit `json:"QPS"`

	// Concurrency 并发配额
	Concurrency QuotaLimit `json:"Concurrency"`

	// CharacterQuota 字符配额（TTS）
	CharacterQuota *CharacterQuota `json:"CharacterQuota,omitempty"`
}

// QuotaLimit 配额限制
type QuotaLimit struct {
	// Limit 限制值
	Limit int `json:"Limit"`

	// Used 已使用
	Used int `json:"Used"`
}

// CharacterQuota 字符配额
type CharacterQuota struct {
	// Total 总量
	Total int64 `json:"Total"`

	// Used 已使用
	Used int64 `json:"Used"`

	// Remaining 剩余
	Remaining int64 `json:"Remaining"`
}

// UsageRequest 调用量查询请求
type UsageRequest struct {
	// ServiceId 服务 ID
	ServiceId string `json:"ServiceId,omitempty"`

	// StartTime 开始时间
	StartTime time.Time `json:"StartTime"`

	// EndTime 结束时间
	EndTime time.Time `json:"EndTime"`

	// Granularity 粒度：hour/day/month
	Granularity UsageGranularity `json:"Granularity,omitempty"`
}

// UsageGranularity 统计粒度
type UsageGranularity string

const (
	UsageGranularityHour  UsageGranularity = "hour"
	UsageGranularityDay   UsageGranularity = "day"
	UsageGranularityMonth UsageGranularity = "month"
)

// UsageResponse 调用量响应
type UsageResponse struct {
	// ServiceId 服务 ID
	ServiceId string `json:"ServiceId"`

	// StartTime 开始时间
	StartTime time.Time `json:"StartTime"`

	// EndTime 结束时间
	EndTime time.Time `json:"EndTime"`

	// Granularity 粒度
	Granularity UsageGranularity `json:"Granularity"`

	// DataPoints 数据点
	DataPoints []UsageDataPoint `json:"DataPoints"`

	// Summary 汇总
	Summary UsageSummary `json:"Summary"`
}

// UsageDataPoint 调用量数据点
type UsageDataPoint struct {
	// Timestamp 时间戳
	Timestamp time.Time `json:"Timestamp"`

	// Requests 请求数
	Requests int64 `json:"Requests"`

	// Characters 字符数
	Characters int64 `json:"Characters,omitempty"`

	// Duration 音频时长（毫秒）
	Duration int64 `json:"Duration,omitempty"`

	// SuccessRate 成功率
	SuccessRate float64 `json:"SuccessRate"`
}

// UsageSummary 调用量汇总
type UsageSummary struct {
	// TotalRequests 总请求数
	TotalRequests int64 `json:"TotalRequests"`

	// TotalCharacters 总字符数
	TotalCharacters int64 `json:"TotalCharacters,omitempty"`

	// TotalDuration 总音频时长（毫秒）
	TotalDuration int64 `json:"TotalDuration,omitempty"`

	// AverageSuccessRate 平均成功率
	AverageSuccessRate float64 `json:"AverageSuccessRate"`
}

// QPSResponse QPS 响应
type QPSResponse struct {
	// CurrentQPS 当前 QPS
	CurrentQPS int `json:"CurrentQPS"`

	// MaxQPS 最大 QPS
	MaxQPS int `json:"MaxQPS"`

	// CurrentConcurrency 当前并发
	CurrentConcurrency int `json:"CurrentConcurrency"`

	// MaxConcurrency 最大并发
	MaxConcurrency int `json:"MaxConcurrency"`

	// Timestamp 时间戳
	Timestamp time.Time `json:"Timestamp"`
}

// ================== 声音复刻管理（控制台） ==================

// VoiceCloneManageService 声音复刻管理服务（控制台）
type VoiceCloneManageService interface {
	// ListTrainStatus 查询 SpeakerID 状态信息
	ListTrainStatus(ctx context.Context, speakerID string) (*VoiceCloneTrainStatusResponse, error)

	// BatchListTrainStatus 分页查询 SpeakerID 状态
	BatchListTrainStatus(ctx context.Context, req *BatchListTrainStatusRequest) (*BatchListTrainStatusResponse, error)
}

// VoiceCloneTrainStatusResponse 训练状态响应
type VoiceCloneTrainStatusResponse struct {
	// SpeakerId 音色 ID
	SpeakerId string `json:"SpeakerId"`

	// Status 状态
	Status VoiceCloneStatusType `json:"Status"`

	// Progress 进度（0-100）
	Progress int `json:"Progress,omitempty"`

	// Message 消息
	Message string `json:"Message,omitempty"`

	// CreatedAt 创建时间
	CreatedAt time.Time `json:"CreatedAt"`

	// UpdatedAt 更新时间
	UpdatedAt time.Time `json:"UpdatedAt"`
}

// BatchListTrainStatusRequest 批量查询训练状态请求
type BatchListTrainStatusRequest struct {
	// PageNumber 页码
	PageNumber int `json:"PageNumber,omitempty"`

	// PageSize 每页数量
	PageSize int `json:"PageSize,omitempty"`

	// Status 状态筛选
	Status VoiceCloneStatusType `json:"Status,omitempty"`
}

// BatchListTrainStatusResponse 批量查询训练状态响应
type BatchListTrainStatusResponse struct {
	// Total 总数
	Total int `json:"Total"`

	// Items 状态列表
	Items []VoiceCloneTrainStatusResponse `json:"Items"`
}
