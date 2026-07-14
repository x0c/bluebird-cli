package cmdtree

import (
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/x0c/bluebird-cli/internal/output"
	"github.com/x0c/bluebird-cli/internal/version"
)

type describeFlag struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Default string `json:"default"`
	Usage   string `json:"usage"`
}

type describeCommand struct {
	Path    string         `json:"path"`
	Summary string         `json:"summary"`
	Flags   []describeFlag `json:"flags"`
}

type describeOutput struct {
	Commands  []describeCommand `json:"commands"`
	ExitCodes map[string]string `json:"exit_codes"`
	Env       []string          `json:"env"`
}

// 只列变量名，不写值——密钥/身份禁止出现在任何输出里。
var envNames = []string{
	"BLUEBIRD_BASE_URL",
	"BLUEBIRD_TOKEN",
	"BLUEBIRD_JWT_SECRET",
	"BLUEBIRD_JWT_ISSUER",
	"BLUEBIRD_USER_ID",
	"OPENCONDUCTOR_SUPABASE_JWT_SECRET",
}

var exitCodeMeanings = map[string]string{
	"0": "成功（含幂等 unchanged）",
	"1": "一般失败（5xx / 网络错误 / 429 限流 / 501 未启用）",
	"2": "用法错误（CLI 参数错 / 后端 400）",
	"3": "资源不存在（404 / 地名零结果 / 无位置）",
	"4": "权限 / 鉴权失败（401 / 403 / 未配置鉴权）",
	"5": "冲突 / 歧义（地名多结果 / watch 收敛冲突 / 后端 409）",
	"6": "超时（客户端超时 / 网关 504）",
}

// describeCmd 遍历实际命令树生成机器可读的自描述，与参数解析同源，杜绝文档漂移。
// 离线可用，零网络。
func (a *App) describeCmd(root *cobra.Command, opts *globalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "describe [command]",
		Short: "输出机器可读的命令/参数说明（离线，与实际命令树同源）",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			start := time.Now()
			jsonMode := output.DecideJSON(opts.JSON)

			target := root
			if len(args) == 1 {
				found, _, err := root.Find([]string{args[0]})
				if err != nil || found == nil {
					exit := output.Emit(a.Stdout, a.Stderr, version.Version, start, opts.renderOptions(jsonMode, nil),
						output.Result{Err: output.NewError(output.CodeNotFound, "命令不存在："+args[0])}, nil)
					return exitError(exit)
				}
				target = found
			}

			data := describeOutput{
				Commands:  walkCommands(target),
				ExitCodes: exitCodeMeanings,
				Env:       envNames,
			}
			exit := output.Emit(a.Stdout, a.Stderr, version.Version, start, opts.renderOptions(jsonMode, nil),
				output.Result{Data: data}, renderDescribeHuman)
			return exitError(exit)
		},
	}
	return cmd
}

func walkCommands(cmd *cobra.Command) []describeCommand {
	var out []describeCommand
	if cmd.Runnable() || cmd.HasSubCommands() {
		out = append(out, describeCommand{
			Path:    cmd.CommandPath(),
			Summary: cmd.Short,
			Flags:   collectFlags(cmd),
		})
	}
	for _, sub := range cmd.Commands() {
		if sub.Hidden || sub.Name() == "completion" || sub.Name() == "help" {
			continue
		}
		out = append(out, walkCommands(sub)...)
	}
	return out
}

// collectFlags 只返回命令自身定义的 flag（不含从父命令继承的全局 flag），
// 避免每个节点都重复列出 8 个全局 flag、放大 describe 的 token 体积；
// 全局 flag 只在根命令节点出现一次。cobra 自动加的 --help 统一过滤掉。
func collectFlags(cmd *cobra.Command) []describeFlag {
	var flags []describeFlag
	cmd.LocalNonPersistentFlags().VisitAll(func(f *pflag.Flag) {
		if f.Name == "help" {
			return
		}
		flags = append(flags, describeFlag{
			Name:    f.Name,
			Type:    f.Value.Type(),
			Default: f.DefValue,
			Usage:   f.Usage,
		})
	})
	if cmd.Parent() == nil {
		cmd.PersistentFlags().VisitAll(func(f *pflag.Flag) {
			if f.Name == "help" {
				return
			}
			flags = append(flags, describeFlag{
				Name:    f.Name,
				Type:    f.Value.Type(),
				Default: f.DefValue,
				Usage:   f.Usage,
			})
		})
	}
	return flags
}

func renderDescribeHuman(w io.Writer, data any, _ bool) error {
	out, ok := data.(describeOutput)
	if !ok {
		_, err := fmt.Fprintln(w, data)
		return err
	}
	for _, c := range out.Commands {
		if _, err := fmt.Fprintf(w, "%s — %s\n", c.Path, c.Summary); err != nil {
			return err
		}
	}
	return nil
}
