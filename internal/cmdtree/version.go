package cmdtree

import (
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"
	"github.com/x0c/bluebird-cli/internal/output"
	"github.com/x0c/bluebird-cli/internal/version"
)

type versionInfo struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
	Date    string `json:"date"`
}

// versionCmd 离线可用，零网络。
func (a *App) versionCmd(opts *globalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "打印 CLI 版本信息（离线，零网络）",
		RunE: func(cmd *cobra.Command, args []string) error {
			start := time.Now()
			jsonMode := output.DecideJSON(opts.JSON)
			info := versionInfo{Version: version.Version, Commit: version.Commit, Date: version.Date}
			exit := output.Emit(a.Stdout, a.Stderr, version.Version, start, opts.renderOptions(jsonMode, nil),
				output.Result{Data: info}, renderVersionHuman)
			return exitError(exit)
		},
	}
}

func renderVersionHuman(w io.Writer, data any, _ bool) error {
	info, ok := data.(versionInfo)
	if !ok {
		_, err := fmt.Fprintln(w, data)
		return err
	}
	_, err := fmt.Fprintf(w, "bluebird %s (commit %s, built %s)\n", info.Version, info.Commit, info.Date)
	return err
}
