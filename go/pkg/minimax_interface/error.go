package minimax_interface

import (
	"errors"
	"fmt"
)

// Error 表示 MiniMax API 返回的错误
type Error struct {
	// StatusCode API 错误码
	StatusCode int `json:"status_code"`

	// StatusMsg 错误消息
	StatusMsg string `json:"status_msg"`

	// TraceID 请求追踪 ID，用于问题排查
	TraceID string `json:"trace_id"`

	// HTTPStatus HTTP 状态码
	HTTPStatus int `json:"-"`
}

// Error 实现 error 接口
func (e *Error) Error() string {
	return fmt.Sprintf("minimax: %s (code=%d, trace=%s)", e.StatusMsg, e.StatusCode, e.TraceID)
}

// IsRateLimit 判断是否为限流错误
func (e *Error) IsRateLimit() bool {
	// TODO: 确认实际错误码
	return e.StatusCode == 1002 || e.HTTPStatus == 429
}

// IsInvalidAPIKey 判断是否为无效 API Key 错误
func (e *Error) IsInvalidAPIKey() bool {
	// TODO: 确认实际错误码
	return e.StatusCode == 1001 || e.HTTPStatus == 401
}

// IsInsufficientQuota 判断是否为配额不足错误
func (e *Error) IsInsufficientQuota() bool {
	// TODO: 确认实际错误码
	return e.StatusCode == 1003
}

// IsInvalidRequest 判断是否为无效请求错误
func (e *Error) IsInvalidRequest() bool {
	return e.StatusCode >= 2000 && e.StatusCode < 3000
}

// IsServerError 判断是否为服务端错误
func (e *Error) IsServerError() bool {
	return e.StatusCode >= 5000 || e.HTTPStatus >= 500
}

// Retryable 判断该错误是否可重试
func (e *Error) Retryable() bool {
	return e.IsRateLimit() || e.IsServerError()
}

// AsError 从 error 中提取 *Error
//
// 示例:
//
//	if e, ok := minimax.AsError(err); ok {
//	    if e.IsRateLimit() {
//	        // 处理限流
//	    }
//	}
func AsError(err error) (*Error, bool) {
	var e *Error
	if errors.As(err, &e) {
		return e, true
	}
	return nil, false
}
