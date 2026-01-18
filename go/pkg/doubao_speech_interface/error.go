package doubao_speech_interface

import (
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
}

func (e *Error) Error() string {
	return fmt.Sprintf("doubao_speech: %s (code=%d, trace_id=%s, log_id=%s, http_status=%d)",
		e.Message, e.Code, e.TraceID, e.LogID, e.HTTPStatus)
}

// IsAuthError 是否为认证错误
func (e *Error) IsAuthError() bool {
	return e.HTTPStatus == http.StatusUnauthorized || e.HTTPStatus == http.StatusForbidden
}

// IsRateLimit 是否为限流错误
func (e *Error) IsRateLimit() bool {
	return e.HTTPStatus == http.StatusTooManyRequests
}

// IsQuotaExceeded 是否为配额超限
func (e *Error) IsQuotaExceeded() bool {
	return e.HTTPStatus == http.StatusPaymentRequired
}

// IsInvalidParam 是否为参数错误
func (e *Error) IsInvalidParam() bool {
	return e.HTTPStatus == http.StatusBadRequest
}

// IsServerError 是否为服务端错误
func (e *Error) IsServerError() bool {
	return e.HTTPStatus >= http.StatusInternalServerError
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
