package cmdtree

import (
	"fmt"
	"io"
	"net/url"
	"strconv"
	"time"

	"github.com/spf13/cobra"
	"github.com/x0c/bluebird-cli/internal/output"
	"github.com/x0c/bluebird-cli/internal/scenes"
	"github.com/x0c/bluebird-cli/internal/version"
)

func (a *App) conditionsCmd(opts *globalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "conditions",
		Short: "查询好天气状态评分与推荐窗口",
	}
	cmd.AddCommand(a.conditionsNowCmd(opts), a.conditionsWindowsCmd(opts))
	return cmd
}

func (a *App) conditionsNowCmd(opts *globalOptions) *cobra.Command {
	pf := &placeFlags{}
	var hours int
	cmd := &cobra.Command{
		Use:   "now",
		Short: "查询当前好天气状态评分（可省略地点，使用最近上报位置）",
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runConditions(cmd, opts, pf, hours, "/api/conditions/now")
		},
	}
	addPlaceFlags(cmd, pf)
	cmd.Flags().IntVar(&hours, "hours", 24, "评分覆盖的小时数（1-168）")
	return cmd
}

func (a *App) conditionsWindowsCmd(opts *globalOptions) *cobra.Command {
	pf := &placeFlags{}
	var hours int
	cmd := &cobra.Command{
		Use:   "windows",
		Short: "查询未来可推荐的好天气窗口（可省略地点，使用最近上报位置）",
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runConditions(cmd, opts, pf, hours, "/api/conditions/windows")
		},
	}
	addPlaceFlags(cmd, pf)
	cmd.Flags().IntVar(&hours, "hours", 72, "评分覆盖的小时数（1-168）")
	return cmd
}

func (a *App) runConditions(cmd *cobra.Command, opts *globalOptions, pf *placeFlags, hours int, path string) error {
	start := time.Now()
	jsonMode := output.DecideJSON(opts.JSON)

	client, _, cmdErr := a.newClientOrErr(opts)
	if cmdErr != nil {
		return a.emitErr(opts, jsonMode, start, cmdErr)
	}

	in := placeInputFrom(cmd, pf)
	result, cmdErr := a.resolvePlace(cmd.Context(), client, in, opts.NoInput)
	if cmdErr != nil {
		return a.emitErr(opts, jsonMode, start, cmdErr)
	}

	query := url.Values{}
	if hours > 0 {
		query.Set("hours", strconv.Itoa(hours))
	}
	var resolvedPlace any
	if result != nil {
		query.Set("lat", strconv.FormatFloat(result.Latitude, 'f', -1, 64))
		query.Set("lon", strconv.FormatFloat(result.Longitude, 'f', -1, 64))
		if result.Elevation != 0 {
			query.Set("elevation", strconv.FormatFloat(result.Elevation, 'f', -1, 64))
		}
		if result.Resolved != nil {
			resolvedPlace = result.Resolved
		}
	}

	var data map[string]any
	if err := client.Get(cmd.Context(), path, query, &data); err != nil {
		mapped := toCmdError(err)
		if mapped.Code == output.CodeNotFound {
			mapped = output.NewError(output.CodeNoLocation, "未提供地点，且没有最近上报的位置可用").
				WithHint("加 --place <地名> 或 --lat/--lon 指定地点")
		}
		return a.emitErr(opts, jsonMode, start, mapped)
	}

	exit := output.Emit(a.Stdout, a.Stderr, version.Version, start,
		opts.renderOptions(jsonMode, []string{"scores", "windows"}),
		output.Result{Data: data, ResolvedPlace: resolvedPlace}, renderConditionsHuman)
	return exitError(exit)
}

func renderConditionsHuman(w io.Writer, data any, _ bool) error {
	m, ok := data.(map[string]any)
	if !ok {
		_, err := fmt.Fprintln(w, data)
		return err
	}

	wrote := false
	if scoresRaw, ok := m["scores"].([]any); ok && len(scoresRaw) > 0 {
		wrote = true
		if _, err := fmt.Fprintln(w, "当前评估："); err != nil {
			return err
		}
		for _, s := range scoresRaw {
			sm, _ := s.(map[string]any)
			scene, _ := sm["scene"].(string)
			reason, _ := sm["reason"].(string)
			name := sceneOrRaw(scene)
			if _, err := fmt.Fprintf(w, "  - %s：%s\n", name, reason); err != nil {
				return err
			}
		}
	}
	if windowsRaw, ok := m["windows"].([]any); ok && len(windowsRaw) > 0 {
		wrote = true
		if _, err := fmt.Fprintln(w, "推荐窗口："); err != nil {
			return err
		}
		for _, wi := range windowsRaw {
			wm, _ := wi.(map[string]any)
			scene, _ := wm["scene"].(string)
			summary, _ := wm["summary"].(string)
			startStr, _ := wm["start"].(string)
			endStr, _ := wm["end"].(string)
			name := sceneOrRaw(scene)
			if _, err := fmt.Fprintf(w, "  - %s：%s ~ %s %s\n", name, formatLocalTime(startStr), formatLocalTime(endStr), summary); err != nil {
				return err
			}
		}
	}
	if !wrote {
		if _, err := fmt.Fprintln(w, "暂无可展示的评估结果。"); err != nil {
			return err
		}
	}
	return nil
}

func sceneOrRaw(id string) string {
	if name := scenes.NameZH(id); name != "" {
		return name
	}
	return id
}

func formatLocalTime(s string) string {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return s
	}
	return t.Local().Format("01-02 15:04")
}
