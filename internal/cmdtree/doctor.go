package cmdtree

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"
	"github.com/x0c/bluebird-cli/internal/api"
	"github.com/x0c/bluebird-cli/internal/auth"
	"github.com/x0c/bluebird-cli/internal/output"
	"github.com/x0c/bluebird-cli/internal/version"
)

type doctorCheck struct {
	Name   string `json:"name"`
	Status string `json:"status"` // ok | warn | fail | skip
	Detail string `json:"detail"`
}

type doctorOutput struct {
	Checks []doctorCheck `json:"checks"`
}

// doctorCmd 检查配置、鉴权自验与服务可达性；--no-probe 时只跑离线段（配置/自签自验），零网络。
func (a *App) doctorCmd(opts *globalOptions) *cobra.Command {
	var noProbe bool
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "检查配置、鉴权与服务可达性",
		RunE: func(cmd *cobra.Command, args []string) error {
			start := time.Now()
			jsonMode := output.DecideJSON(opts.JSON)
			cfg := a.authConfig()
			cfg.Timeout = opts.Timeout

			checks, cmdErr := runDoctorChecks(cmd.Context(), cfg, noProbe)
			var res output.Result
			if cmdErr != nil {
				cmdErr = cmdErr.WithDetails(checks)
				res = output.Result{Err: cmdErr}
			} else {
				res = output.Result{Data: doctorOutput{Checks: checks}}
			}
			exit := output.Emit(a.Stdout, a.Stderr, version.Version, start, opts.renderOptions(jsonMode, nil), res, renderDoctorHuman)
			return exitError(exit)
		},
	}
	cmd.Flags().BoolVar(&noProbe, "no-probe", false, "只跑离线检查（配置/鉴权自验），不发起网络请求")
	return cmd
}

func runDoctorChecks(ctx context.Context, cfg auth.Config, noProbe bool) ([]doctorCheck, *output.CmdError) {
	var checks []doctorCheck

	checks = append(checks, doctorCheck{Name: "base_url", Status: "ok", Detail: "当前生效地址：" + cfg.BaseURL})
	checks = append(checks, doctorCheck{Name: "BLUEBIRD_TOKEN", Status: statusFor(cfg.Token != ""), Detail: configuredDetail(cfg.Token != "")})
	secretConfigured := cfg.JWTSecret != ""
	userConfigured := cfg.UserID != ""
	checks = append(checks, doctorCheck{Name: "BLUEBIRD_JWT_SECRET（含回退 OPENCONDUCTOR_SUPABASE_JWT_SECRET）", Status: statusFor(secretConfigured), Detail: configuredDetail(secretConfigured)})
	checks = append(checks, doctorCheck{Name: "BLUEBIRD_USER_ID", Status: statusFor(userConfigured), Detail: configuredDetail(userConfigured)})

	if secretConfigured && userConfigured {
		if err := auth.VerifySelfSigned(cfg.JWTSecret, cfg.JWTIssuer, cfg.UserID); err != nil {
			checks = append(checks, doctorCheck{Name: "自签校验", Status: "fail", Detail: "自签 token 无法通过自验：" + err.Error()})
		} else {
			checks = append(checks, doctorCheck{Name: "自签校验", Status: "ok", Detail: "自签 token 签名与验签一致（不触网）"})
		}
	}

	if !cfg.Configured() {
		checks = append(checks, doctorCheck{Name: "鉴权路径", Status: "warn", Detail: "未配置任何鉴权路径，需鉴权命令将返回码 4"})
	}

	if noProbe {
		return checks, nil
	}

	healthzClient := api.NewUnauthenticatedClient(cfg.BaseURL, cfg.Timeout, version.Version)
	var healthzOut map[string]any
	if err := healthzClient.Get(ctx, "/healthz", nil, &healthzOut); err != nil {
		cmdErr, _ := err.(*output.CmdError)
		if cmdErr == nil {
			cmdErr = output.NewError(output.CodeNetworkError, err.Error()).WithRetryable(true)
		}
		checks = append(checks, doctorCheck{Name: "healthz", Status: "fail", Detail: cmdErr.Message})
		return checks, cmdErr
	}
	checks = append(checks, doctorCheck{Name: "healthz", Status: "ok", Detail: "GET /healthz 可达"})

	if !cfg.Configured() {
		checks = append(checks, doctorCheck{Name: "鉴权探测", Status: "skip", Detail: "未配置鉴权，跳过在线探测"})
		return checks, nil
	}

	client, err := api.NewClient(cfg, version.Version)
	if err != nil {
		cmdErr := output.NewError(output.CodeAuthNotConfigured, "无法生成令牌："+err.Error())
		checks = append(checks, doctorCheck{Name: "鉴权探测", Status: "fail", Detail: cmdErr.Message})
		return checks, cmdErr
	}

	var watchesOut any
	if err := client.Get(ctx, "/api/watches", nil, &watchesOut); err != nil {
		cmdErr, _ := err.(*output.CmdError)
		if cmdErr == nil {
			cmdErr = output.NewError(output.CodeNetworkError, err.Error()).WithRetryable(true)
		}
		checks = append(checks, doctorCheck{Name: "鉴权探测", Status: "fail", Detail: fmt.Sprintf("令牌来源 %s：%s", client.TokenSource(), cmdErr.Message)})
		return checks, cmdErr
	}
	checks = append(checks, doctorCheck{Name: "鉴权探测", Status: "ok", Detail: fmt.Sprintf("令牌来源 %s，鉴权通过", client.TokenSource())})

	return checks, nil
}

func statusFor(configured bool) string {
	if configured {
		return "ok"
	}
	return "warn"
}

func configuredDetail(configured bool) string {
	if configured {
		return "已配置"
	}
	return "未配置"
}

func renderDoctorHuman(w io.Writer, data any, _ bool) error {
	out, ok := data.(doctorOutput)
	if !ok {
		_, err := fmt.Fprintln(w, data)
		return err
	}
	for _, c := range out.Checks {
		if _, err := fmt.Fprintf(w, "[%s] %s：%s\n", c.Status, c.Name, c.Detail); err != nil {
			return err
		}
	}
	return nil
}
