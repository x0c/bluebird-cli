package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"

	"github.com/x0c/bluebird-cli/internal/output"
)

// backendErrorBody 是后端统一的错误响应体 {"error": "..."}。
type backendErrorBody struct {
	Error string `json:"error"`
}

// MapHTTPError 把后端 HTTP 状态码 + 响应体映射为 CmdError；后端 error 字段已经是中文，直接透传。
func MapHTTPError(status int, raw []byte) *output.CmdError {
	message := extractBackendMessage(raw)
	switch {
	case status == 400:
		return output.NewError(output.CodeBadRequest, orDefault(message, "请求参数不合法"))
	case status == 401 || status == 403:
		return output.NewError(output.CodeUnauthorized, orDefault(message, "鉴权失败"))
	case status == 404:
		return output.NewError(output.CodeNotFound, orDefault(message, "资源不存在"))
	case status == 409:
		return output.NewError(output.CodeWatchConflict, orDefault(message, "存在冲突"))
	case status == 429:
		return output.NewError(output.CodeRateLimited, orDefault(message, "触发上游限流，请稍后重试")).WithRetryable(true)
	case status == 501:
		return output.NewError(output.CodeFeatureUnavailable, orDefault(message, "该功能未在服务端启用"))
	case status == 504:
		return output.NewError(output.CodeTimeout, orDefault(message, "网关超时")).WithRetryable(true)
	case status >= 500:
		return output.NewError(output.CodeServerError, orDefault(message, "服务端错误")).WithRetryable(true)
	default:
		return output.NewError(output.CodeServerError, fmt.Sprintf("未预期的响应状态码 %d：%s", status, orDefault(message, "无详情"))).WithRetryable(true)
	}
}

func extractBackendMessage(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	var body backendErrorBody
	if err := json.Unmarshal(raw, &body); err != nil {
		return ""
	}
	return body.Error
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

// MapTransportError 把网络层错误（超时、连接失败等）映射为 CmdError。
func MapTransportError(err error) *output.CmdError {
	if errors.Is(err, context.DeadlineExceeded) {
		return output.NewError(output.CodeTimeout, "请求超时："+err.Error()).WithRetryable(true)
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return output.NewError(output.CodeTimeout, "请求超时："+err.Error()).WithRetryable(true)
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) && urlErr.Timeout() {
		return output.NewError(output.CodeTimeout, "请求超时："+err.Error()).WithRetryable(true)
	}
	return output.NewError(output.CodeNetworkError, "网络请求失败："+err.Error()).WithRetryable(true)
}

// NewDecodeError 包装响应体 JSON 解析失败。
func NewDecodeError(err error) *output.CmdError {
	return output.NewError(output.CodeServerError, "解析服务端响应失败："+err.Error())
}

// NewMarshalError 包装请求体序列化失败（属于调用方构造请求的问题）。
func NewMarshalError(err error) *output.CmdError {
	return output.NewError(output.CodeUsageError, "构造请求失败："+err.Error())
}

// NewRequestError 包装 http.NewRequestWithContext 失败（如非法 URL）。
func NewRequestError(err error) *output.CmdError {
	return output.NewError(output.CodeUsageError, "构造请求失败："+err.Error())
}
