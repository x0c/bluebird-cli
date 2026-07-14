package output

// 退出码语义（对外契约，发布后只加不改不删）：
//
//	0 成功（含幂等 unchanged）
//	1 一般失败（5xx / 网络错误 / 429 限流 / 501 未启用）
//	2 用法错误（CLI 参数错 / 后端 400）
//	3 资源不存在（404 / 地名零结果 / 无位置）
//	4 权限 / 鉴权失败（401 / 403 / 未配置鉴权）
//	5 冲突 / 歧义（地名多结果 / watch 收敛冲突 / 后端 409）
//	6 超时（客户端超时 / 网关 504）
const (
	ExitOK       = 0
	ExitGeneral  = 1
	ExitUsage    = 2
	ExitNotFound = 3
	ExitAuth     = 4
	ExitConflict = 5
	ExitTimeout  = 6
)

// 稳定英文错误短码，供 error.code 使用；新增枚举值只加不改。
const (
	CodeUsageError         = "usage_error"
	CodeInvalidScene       = "invalid_scene"
	CodeBadRequest         = "bad_request"
	CodeNotFound           = "not_found"
	CodePlaceNotFound      = "place_not_found"
	CodeNoLocation         = "no_location"
	CodeUnauthorized       = "unauthorized"
	CodeAuthNotConfigured  = "auth_not_configured"
	CodePlaceAmbiguous     = "place_ambiguous"
	CodeWatchConflict      = "watch_conflict"
	CodeServerError        = "server_error"
	CodeNetworkError       = "network_error"
	CodeRateLimited        = "rate_limited"
	CodeFeatureUnavailable = "feature_unavailable"
	CodeTimeout            = "timeout"
)

// codeToExit 是 error.code -> 退出码的单一权威映射。
var codeToExit = map[string]int{
	CodeUsageError:         ExitUsage,
	CodeInvalidScene:       ExitUsage,
	CodeBadRequest:         ExitUsage,
	CodeNotFound:           ExitNotFound,
	CodePlaceNotFound:      ExitNotFound,
	CodeNoLocation:         ExitNotFound,
	CodeUnauthorized:       ExitAuth,
	CodeAuthNotConfigured:  ExitAuth,
	CodePlaceAmbiguous:     ExitConflict,
	CodeWatchConflict:      ExitConflict,
	CodeServerError:        ExitGeneral,
	CodeNetworkError:       ExitGeneral,
	CodeRateLimited:        ExitGeneral,
	CodeFeatureUnavailable: ExitGeneral,
	CodeTimeout:            ExitTimeout,
}

// ExitCodeFor 返回给定 error.code 对应的进程退出码；未知短码退化为 ExitGeneral。
func ExitCodeFor(code string) int {
	if code == "" {
		return ExitOK
	}
	if exit, ok := codeToExit[code]; ok {
		return exit
	}
	return ExitGeneral
}
