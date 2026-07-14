// Package cmdtree 组装 bluebird 的 cobra 命令树。
// 所有命令通过 App 接收注入的环境变量读取函数与 IO，不直接依赖 os.Getenv/os.Stdout，
// 使得整棵命令树可以在测试里离线、隔离地被驱动。
package cmdtree

import (
	"io"
	"time"

	"github.com/x0c/bluebird-cli/internal/auth"
	"github.com/x0c/bluebird-cli/internal/output"
)

// App 持有构建命令树所需的全部外部依赖注入点：测试可注入内存 buffer 与假环境变量，
// 使整棵命令树离线、隔离地被驱动，无需触碰真实用户环境。
type App struct {
	Stdout io.Writer
	Stderr io.Writer
	Stdin  io.Reader
	Getenv func(string) string
}

// globalOptions 是绑定到根命令 persistent flags 的全局选项。
type globalOptions struct {
	JSON    bool
	Fields  []string
	Compact bool
	Quiet   bool
	NoColor bool
	NoInput bool
	Timeout time.Duration
}

func (o *globalOptions) renderOptions(jsonMode bool, compactFields []string) output.RenderOptions {
	return output.RenderOptions{
		JSONMode:      jsonMode,
		Fields:        o.Fields,
		Compact:       o.Compact,
		CompactFields: compactFields,
		Quiet:         o.Quiet,
	}
}

// authConfig 从注入的 Getenv 加载鉴权配置。
func (a *App) authConfig() auth.Config {
	return auth.LoadConfig(a.Getenv)
}
