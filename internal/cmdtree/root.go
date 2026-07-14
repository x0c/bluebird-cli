package cmdtree

import (
	"time"

	"github.com/spf13/cobra"
	"github.com/x0c/bluebird-cli/internal/auth"
)

// Build 组装完整命令树，返回可直接 Execute 的根命令。
func (a *App) Build() *cobra.Command {
	opts := &globalOptions{}

	root := &cobra.Command{
		Use:           "bluebird",
		Short:         "Bluebird 天气发现服务命令行客户端",
		Long:          "bluebird 是 Bluebird（「等好天气」）天气发现服务的命令行客户端，面向人与大模型 Agent 双受众设计。",
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	root.SetOut(a.Stdout)
	root.SetErr(a.Stderr)

	root.PersistentFlags().BoolVar(&opts.JSON, "json", false, "以 JSON envelope 输出（stdout 非终端时自动开启）")
	root.PersistentFlags().StringSliceVar(&opts.Fields, "fields", nil, "按点路径裁剪输出字段，如 watch.id,watch.name")
	root.PersistentFlags().BoolVar(&opts.Compact, "compact", false, "输出精简结果（只减不增）")
	root.PersistentFlags().BoolVar(&opts.Quiet, "quiet", false, "人类模式下只输出关键裸值，便于管道传递")
	root.PersistentFlags().BoolVar(&opts.NoColor, "no-color", false, "关闭颜色输出")
	root.PersistentFlags().BoolVar(&opts.NoInput, "no-input", false, "关闭一切交互确认")
	root.PersistentFlags().BoolVar(&opts.NoInput, "yes", false, "同 --no-input，关闭一切交互确认")
	root.PersistentFlags().DurationVar(&opts.Timeout, "timeout", time.Duration(auth.DefaultTimeoutSeconds)*time.Second, "HTTP 请求超时（生产冷缓存首查可达 20s+，不建议调小）")

	root.AddCommand(
		a.versionCmd(opts),
		a.describeCmd(root, opts),
		a.doctorCmd(opts),
		a.conditionsCmd(opts),
		a.placeCmd(opts),
		a.astroCmd(opts),
		a.airCmd(opts),
		a.watchCmd(opts),
	)

	return root
}
