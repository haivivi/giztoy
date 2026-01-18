package doubaospeech

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

// Error 豆包语音 API 错误
type Error struct {
	// Code 业务错误码
	Code int `json:"code"`

	// Message 错误消息
	Message string `json:"message"`

	// TraceID 请求追踪 ID
	TraceID string `json:"trace_id,omitempty"`

	// LogID 日志 ID（从响应头 X-Tt-Logid 获取）
	LogID string `json:"log_id,omitempty"`

	// HTTPStatus HTTP 状态码
	HTTPStatus int `json:"-"`

	// ReqID 请求 ID
	ReqID string `json:"reqid,omitempty"`
}

func (e *Error) Error() string {
	return fmt.Sprintf("doubaospeech: %s (code=%d, trace_id=%s, log_id=%s, http_status=%d)",
		e.Message, e.Code, e.TraceID, e.LogID, e.HTTPStatus)
}

// IsAuthError 是否为认证错误
func (e *Error) IsAuthError() bool {
	return e.HTTPStatus == http.StatusUnauthorized || e.HTTPStatus == http.StatusForbidden
}

// IsRateLimit 是否为限流错误
func (e *Error) IsRateLimit() bool {
	return e.HTTPStatus == http.StatusTooManyRequests || e.Code == 3003
}

// IsQuotaExceeded 是否为配额超限
func (e *Error) IsQuotaExceeded() bool {
	return e.HTTPStatus == http.StatusPaymentRequired || e.Code == 3004
}

// IsInvalidParam 是否为参数错误
func (e *Error) IsInvalidParam() bool {
	return e.HTTPStatus == http.StatusBadRequest || e.Code == 3001
}

// IsServerError 是否为服务端错误
func (e *Error) IsServerError() bool {
	return e.HTTPStatus >= http.StatusInternalServerError || e.Code == 3005
}

// Retryable 是否可重试
func (e *Error) Retryable() bool {
	return e.IsRateLimit() || e.IsServerError()
}

// AsError 尝试将 error 转换为 *Error
func AsError(err error) (*Error, bool) {
	var e *Error
	if errors.As(err, &e) {
		return e, true
	}
	return nil, false
}

// API 响应状态码
const (
	CodeSuccess      = 3000 // 成功
	CodeParamError   = 3001 // 参数错误
	CodeAuthError    = 3002 // 认证失败
	CodeRateLimit    = 3003 // 频率限制
	CodeQuotaExceed  = 3004 // 余额不足
	CodeServerError  = 3005 // 服务内部错误
	CodeASRSuccess   = 1000 // ASR 成功
)

// apiResponse API 通用响应结构
type apiResponse struct {
	ReqID    string          `json:"reqid"`
	Code     int             `json:"code"`
	Message  string          `json:"message"`
	Sequence int32           `json:"sequence"`
	Data     string          `json:"data"` // Base64 encoded data
	Addition json.RawMessage `json:"addition,omitempty"`
}

// asyncTaskResponse 异步任务响应结构
type asyncTaskResponse struct {
	ReqID    string `json:"reqid"`
	TaskID   string `json:"task_id"`
	Status   string `json:"status"`
	Progress int    `json:"progress,omitempty"`
	Code     int    `json:"code"`
	Message  string `json:"message"`
	AudioURL string `json:"audio_url,omitempty"`
}

// parseAPIError 解析 API 错误
func parseAPIError(statusCode int, body []byte, logID string) error {
	var resp apiResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return &Error{
			Code:       statusCode,
			Message:    string(body),
			HTTPStatus: statusCode,
			LogID:      logID,
		}
	}

	if resp.Code != CodeSuccess && resp.Code != CodeASRSuccess {
		return &Error{
			Code:       resp.Code,
			Message:    resp.Message,
			HTTPStatus: statusCode,
			LogID:      logID,
			ReqID:      resp.ReqID,
		}
	}

	return nil
}

// newAPIError 创建 API 错误
func newAPIError(code int, message string) *Error {
	return &Error{
		Code:    code,
		Message: message,
	}
}

// wrapError 包装错误
func wrapError(err error, message string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", message, err)
}
