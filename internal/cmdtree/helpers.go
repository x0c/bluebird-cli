package cmdtree

import (
	"time"

	"github.com/x0c/bluebird-cli/internal/api"
	"github.com/x0c/bluebird-cli/internal/auth"
	"github.com/x0c/bluebird-cli/internal/output"
	"github.com/x0c/bluebird-cli/internal/version"
)

// exitError 把 output.Emit 返回的退出码包装成 cobra RunE 可返回的 error，
// 让 main.go 能在 Execute() 之后取出真实退出码（cobra 本身不保留自定义码）。
func exitError(code int) error {
	return output.AsExitError(code)
}

// toCmdError 把 api.Client 返回的 error 归一化为 *output.CmdError；
// api 包内所有错误路径实际已经是 *output.CmdError，这里兜底处理理论上不会出现的其他类型。
func toCmdError(err error) *output.CmdError {
	if cmdErr, ok := err.(*output.CmdError); ok {
		return cmdErr
	}
	return output.NewError(output.CodeServerError, err.Error()).WithRetryable(true)
}

// newClientOrErr 构造一个已鉴权的 API 客户端；鉴权未配置时直接给出码 4 的 CmdError。
func (a *App) newClientOrErr(opts *globalOptions) (*api.Client, auth.Config, *output.CmdError) {
	cfg := a.authConfig()
	cfg.Timeout = opts.Timeout
	client, err := api.NewClient(cfg, version.Version)
	if err != nil {
		return nil, cfg, output.NewError(output.CodeAuthNotConfigured, "鉴权未配置").
			WithHint("设置 BLUEBIRD_TOKEN，或 BLUEBIRD_JWT_SECRET（可回退 OPENCONDUCTOR_SUPABASE_JWT_SECRET）+ BLUEBIRD_USER_ID")
	}
	return client, cfg, nil
}

// emitErr 是「构造阶段就失败」场景的简写：不需要 dry-run/resolved-place meta 时直接输出错误信封。
func (a *App) emitErr(opts *globalOptions, jsonMode bool, start time.Time, cmdErr *output.CmdError) error {
	exit := output.Emit(a.Stdout, a.Stderr, version.Version, start, opts.renderOptions(jsonMode, nil),
		output.Result{Err: cmdErr}, nil)
	return exitError(exit)
}
