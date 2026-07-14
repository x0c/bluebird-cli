package cmdtree

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/x0c/bluebird-cli/internal/api"
	"github.com/x0c/bluebird-cli/internal/output"
	"github.com/x0c/bluebird-cli/internal/scenes"
	"github.com/x0c/bluebird-cli/internal/version"
	"github.com/x0c/bluebird-cli/internal/watch"
)

func (a *App) watchCmd(opts *globalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "watch",
		Short: "管理持久化关注（地点 × 时间 × 场景订阅）",
	}
	cmd.AddCommand(
		a.watchListCmd(opts),
		a.watchOverviewCmd(opts),
		a.watchGetCmd(opts),
		a.watchOutlookCmd(opts),
		a.watchClimateCmd(opts),
		a.watchCreateCmd(opts),
		a.watchUpdateCmd(opts),
		a.watchDeleteCmd(opts),
	)
	return cmd
}

func (a *App) watchListCmd(opts *globalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "列出全部关注",
		RunE: func(cmd *cobra.Command, args []string) error {
			start := time.Now()
			jsonMode := output.DecideJSON(opts.JSON)
			client, _, cmdErr := a.newClientOrErr(opts)
			if cmdErr != nil {
				return a.emitErr(opts, jsonMode, start, cmdErr)
			}
			var data api.WatchListResponse
			if err := client.Get(cmd.Context(), "/api/watches", nil, &data); err != nil {
				return a.emitErr(opts, jsonMode, start, toCmdError(err))
			}
			exit := output.Emit(a.Stdout, a.Stderr, version.Version, start,
				opts.renderOptions(jsonMode, []string{"watches", "total"}),
				output.Result{Data: data}, renderWatchListHuman)
			return exitError(exit)
		},
	}
}

func (a *App) watchOverviewCmd(opts *globalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "overview",
		Short: "一次加载全部关注的最近提醒与下一个有效窗口（推荐入口，不重跑评分）",
		RunE: func(cmd *cobra.Command, args []string) error {
			start := time.Now()
			jsonMode := output.DecideJSON(opts.JSON)
			client, _, cmdErr := a.newClientOrErr(opts)
			if cmdErr != nil {
				return a.emitErr(opts, jsonMode, start, cmdErr)
			}
			var data map[string]any
			if err := client.Get(cmd.Context(), "/api/watches/overview", nil, &data); err != nil {
				return a.emitErr(opts, jsonMode, start, toCmdError(err))
			}
			exit := output.Emit(a.Stdout, a.Stderr, version.Version, start, opts.renderOptions(jsonMode, nil),
				output.Result{Data: data}, renderGenericJSONHuman)
			return exitError(exit)
		},
	}
}

func (a *App) watchGetCmd(opts *globalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "get <id>",
		Short: "查看单条关注详情",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			start := time.Now()
			jsonMode := output.DecideJSON(opts.JSON)
			client, _, cmdErr := a.newClientOrErr(opts)
			if cmdErr != nil {
				return a.emitErr(opts, jsonMode, start, cmdErr)
			}
			var data api.Watch
			if err := client.Get(cmd.Context(), "/api/watches/"+url.PathEscape(args[0]), nil, &data); err != nil {
				return a.emitErr(opts, jsonMode, start, toCmdError(err))
			}
			exit := output.Emit(a.Stdout, a.Stderr, version.Version, start, opts.renderOptions(jsonMode, nil),
				output.Result{Data: data}, renderWatchHuman)
			return exitError(exit)
		},
	}
}

func (a *App) watchOutlookCmd(opts *globalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "outlook <id>",
		Short: "查看关注未来 16 天的分层趋势",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			start := time.Now()
			jsonMode := output.DecideJSON(opts.JSON)
			client, _, cmdErr := a.newClientOrErr(opts)
			if cmdErr != nil {
				return a.emitErr(opts, jsonMode, start, cmdErr)
			}
			var data map[string]any
			if err := client.Get(cmd.Context(), "/api/watches/"+url.PathEscape(args[0])+"/outlook", nil, &data); err != nil {
				return a.emitErr(opts, jsonMode, start, toCmdError(err))
			}
			exit := output.Emit(a.Stdout, a.Stderr, version.Version, start, opts.renderOptions(jsonMode, nil),
				output.Result{Data: data}, renderGenericJSONHuman)
			return exitError(exit)
		},
	}
}

func (a *App) watchClimateCmd(opts *globalOptions) *cobra.Command {
	var month int
	cmd := &cobra.Command{
		Use:   "climate <id>",
		Short: "查看关注地点的多年历史气候画像（非预报）",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			start := time.Now()
			jsonMode := output.DecideJSON(opts.JSON)
			client, _, cmdErr := a.newClientOrErr(opts)
			if cmdErr != nil {
				return a.emitErr(opts, jsonMode, start, cmdErr)
			}
			query := url.Values{}
			if month > 0 {
				query.Set("month", strconv.Itoa(month))
			}
			var data map[string]any
			if err := client.Get(cmd.Context(), "/api/watches/"+url.PathEscape(args[0])+"/climate", query, &data); err != nil {
				return a.emitErr(opts, jsonMode, start, toCmdError(err))
			}
			exit := output.Emit(a.Stdout, a.Stderr, version.Version, start, opts.renderOptions(jsonMode, nil),
				output.Result{Data: data}, renderGenericJSONHuman)
			return exitError(exit)
		},
	}
	cmd.Flags().IntVar(&month, "month", 0, "月份 1-12（默认当月）")
	return cmd
}

// watchCreateCmd 的语义实际是 ensure：同名同地点（坐标 3 位小数网格）已存在时收敛为
// unchanged，不重复创建；存在但其余字段不同则报冲突（码 5），不做静默覆盖。
func (a *App) watchCreateCmd(opts *globalOptions) *cobra.Command {
	pf := &placeFlags{}
	var (
		name       string
		template   string
		timeMode   string
		dateStr    string
		repeatRule string
		sceneList  []string
		verify     bool
		dryRun     bool
	)
	cmd := &cobra.Command{
		Use:   "create",
		Short: "创建关注（幂等：同名同地点已存在时收敛为 unchanged，不重复创建）",
		RunE: func(cmd *cobra.Command, args []string) error {
			start := time.Now()
			jsonMode := output.DecideJSON(opts.JSON)

			if strings.TrimSpace(name) == "" {
				return a.emitErr(opts, jsonMode, start, output.NewError(output.CodeUsageError, "--name 不能为空"))
			}
			for _, s := range sceneList {
				if !scenes.Valid(s) {
					return a.emitErr(opts, jsonMode, start,
						output.NewError(output.CodeInvalidScene, "未知场景："+s).
							WithHint("合法场景："+strings.Join(scenes.IDs(), ", ")))
				}
			}
			if timeMode == "specific" && dateStr == "" {
				return a.emitErr(opts, jsonMode, start,
					output.NewError(output.CodeUsageError, "--time-mode specific 必须搭配 --date"))
			}

			client, _, cmdErr := a.newClientOrErr(opts)
			if cmdErr != nil {
				return a.emitErr(opts, jsonMode, start, cmdErr)
			}

			in := placeInputFrom(cmd, pf)
			resolved, cmdErr := a.resolvePlace(cmd.Context(), client, in, opts.NoInput)
			if cmdErr != nil {
				return a.emitErr(opts, jsonMode, start, cmdErr)
			}
			if resolved == nil {
				return a.emitErr(opts, jsonMode, start,
					output.NewError(output.CodeUsageError, "watch create 必须指定地点").
						WithHint("加 --place <地名> 或 --lat/--lon"))
			}

			var resolvedPlaceMeta any
			displayName := ""
			if resolved.Resolved != nil {
				displayName = resolved.Resolved.DisplayName
				resolvedPlaceMeta = resolved.Resolved
			}
			desired := api.CreateWatchRequest{
				Name:     name,
				Template: template,
				Place: api.WatchPlace{
					Latitude: resolved.Latitude, Longitude: resolved.Longitude,
					Elevation: resolved.Elevation, DisplayName: displayName,
				},
				TimeMode:      timeMode,
				TargetDate:    dateStr,
				RepeatRule:    repeatRule,
				Scenes:        sceneList,
				VerifyEnabled: verify,
			}

			var list api.WatchListResponse
			if err := client.Get(cmd.Context(), "/api/watches", nil, &list); err != nil {
				return a.emitErr(opts, jsonMode, start, toCmdError(err))
			}

			var existing *api.Watch
			for i := range list.Watches {
				if watch.MatchesKey(list.Watches[i], desired.Name, desired.Place) {
					existing = &list.Watches[i]
					break
				}
			}

			if existing != nil {
				if watch.FieldsEqual(*existing, desired) {
					exit := output.Emit(a.Stdout, a.Stderr, version.Version, start, opts.renderOptions(jsonMode, nil),
						output.Result{
							Data:          map[string]any{"status": "unchanged", "watch": existing},
							DryRun:        dryRun,
							ResolvedPlace: resolvedPlaceMeta,
						}, renderWatchCreateHuman)
					return exitError(exit)
				}
				conflictErr := output.NewError(output.CodeWatchConflict,
					fmt.Sprintf("同名同地点的关注已存在但字段不同（id=%s）", existing.ID)).
					WithHint("修改现有关注，或换一个 --name").
					WithNextCommands(
						fmt.Sprintf("bluebird watch update %s ...", existing.ID),
						"bluebird watch create --name <换个名字> ...",
					).
					WithDetails(map[string]any{"existing": existing})
				return a.emitErr(opts, jsonMode, start, conflictErr)
			}

			if dryRun {
				exit := output.Emit(a.Stdout, a.Stderr, version.Version, start, opts.renderOptions(jsonMode, nil),
					output.Result{
						Data:          map[string]any{"status": "would_create", "watch": desired},
						DryRun:        true,
						ResolvedPlace: resolvedPlaceMeta,
					}, renderWatchCreateHuman)
				return exitError(exit)
			}

			var created api.Watch
			if err := client.Post(cmd.Context(), "/api/watches", desired, &created); err != nil {
				return a.emitErr(opts, jsonMode, start, toCmdError(err))
			}
			exit := output.Emit(a.Stdout, a.Stderr, version.Version, start, opts.renderOptions(jsonMode, nil),
				output.Result{
					Data:          map[string]any{"status": "created", "watch": created},
					ResolvedPlace: resolvedPlaceMeta,
				}, renderWatchCreateHuman)
			return exitError(exit)
		},
	}
	addPlaceFlags(cmd, pf)
	cmd.Flags().StringVar(&name, "name", "", "关注名称（必填）")
	cmd.Flags().StringVar(&template, "template", "trip", "模板：trip/weekend/scene")
	cmd.Flags().StringVar(&timeMode, "time-mode", "unspecified", "时间模式：specific/repeating/unspecified")
	cmd.Flags().StringVar(&dateStr, "date", "", "目标日期 YYYY-MM-DD（time-mode=specific 时必填）")
	cmd.Flags().StringVar(&repeatRule, "repeat", "", "重复规则（如 weekly:sat,sun）")
	cmd.Flags().StringSliceVar(&sceneList, "scenes", nil, "关注场景，逗号分隔，如 sunset_glow,starry_sky")
	cmd.Flags().BoolVar(&verify, "verify", false, "开启命中账本记录（verifyEnabled）")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "只预演不实际创建，绝不发起写请求")
	return cmd
}

func (a *App) watchUpdateCmd(opts *globalOptions) *cobra.Command {
	pf := &placeFlags{}
	var (
		name        string
		template    string
		timeMode    string
		dateStr     string
		repeatRule  string
		sceneList   []string
		clearScenes bool
		verify      bool
		dryRun      bool
	)
	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "部分更新关注（只传要改的字段，PUT 天然幂等）",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			start := time.Now()
			jsonMode := output.DecideJSON(opts.JSON)
			id := args[0]

			client, _, cmdErr := a.newClientOrErr(opts)
			if cmdErr != nil {
				return a.emitErr(opts, jsonMode, start, cmdErr)
			}

			req := api.UpdateWatchRequest{}
			if cmd.Flags().Changed("name") {
				req.Name = &name
			}
			if cmd.Flags().Changed("template") {
				req.Template = &template
			}
			if cmd.Flags().Changed("time-mode") {
				req.TimeMode = &timeMode
			}
			if cmd.Flags().Changed("date") {
				req.TargetDate = &dateStr
			}
			if cmd.Flags().Changed("repeat") {
				req.RepeatRule = &repeatRule
			}
			if cmd.Flags().Changed("verify") {
				v := verify
				req.VerifyEnabled = &v
			}
			if clearScenes {
				req.Scenes = []string{}
			} else if cmd.Flags().Changed("scenes") {
				for _, s := range sceneList {
					if !scenes.Valid(s) {
						return a.emitErr(opts, jsonMode, start,
							output.NewError(output.CodeInvalidScene, "未知场景："+s).
								WithHint("合法场景："+strings.Join(scenes.IDs(), ", ")))
					}
				}
				req.Scenes = sceneList
			}

			var resolvedPlaceMeta any
			hasPlaceFlag := cmd.Flags().Changed("place") || (cmd.Flags().Changed("lat") && cmd.Flags().Changed("lon"))
			if hasPlaceFlag {
				in := placeInputFrom(cmd, pf)
				resolved, cmdErr := a.resolvePlace(cmd.Context(), client, in, opts.NoInput)
				if cmdErr != nil {
					return a.emitErr(opts, jsonMode, start, cmdErr)
				}
				if resolved != nil {
					displayName := ""
					if resolved.Resolved != nil {
						displayName = resolved.Resolved.DisplayName
						resolvedPlaceMeta = resolved.Resolved
					}
					req.Place = &api.WatchPlace{
						Latitude: resolved.Latitude, Longitude: resolved.Longitude,
						Elevation: resolved.Elevation, DisplayName: displayName,
					}
				}
			}

			if dryRun {
				exit := output.Emit(a.Stdout, a.Stderr, version.Version, start, opts.renderOptions(jsonMode, nil),
					output.Result{
						Data:          map[string]any{"status": "would_update", "id": id, "changes": req},
						DryRun:        true,
						ResolvedPlace: resolvedPlaceMeta,
					}, renderWatchCreateHuman)
				return exitError(exit)
			}

			var updated api.Watch
			if err := client.Put(cmd.Context(), "/api/watches/"+url.PathEscape(id), req, &updated); err != nil {
				return a.emitErr(opts, jsonMode, start, toCmdError(err))
			}
			exit := output.Emit(a.Stdout, a.Stderr, version.Version, start, opts.renderOptions(jsonMode, nil),
				output.Result{
					Data:          map[string]any{"status": "applied", "watch": updated},
					ResolvedPlace: resolvedPlaceMeta,
				}, renderWatchCreateHuman)
			return exitError(exit)
		},
	}
	addPlaceFlags(cmd, pf)
	cmd.Flags().StringVar(&name, "name", "", "新名称")
	cmd.Flags().StringVar(&template, "template", "", "模板：trip/weekend/scene")
	cmd.Flags().StringVar(&timeMode, "time-mode", "", "时间模式：specific/repeating/unspecified")
	cmd.Flags().StringVar(&dateStr, "date", "", "目标日期 YYYY-MM-DD")
	cmd.Flags().StringVar(&repeatRule, "repeat", "", "重复规则")
	cmd.Flags().StringSliceVar(&sceneList, "scenes", nil, "关注场景，逗号分隔，整体替换现有场景列表")
	cmd.Flags().BoolVar(&clearScenes, "clear-scenes", false, "清空场景列表（与 --scenes 互斥，优先生效）")
	cmd.Flags().BoolVar(&verify, "verify", false, "开启/关闭命中账本记录")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "只预演不实际修改，绝不发起写请求")
	return cmd
}

// watchDeleteCmd 是幂等删除：目标已不存在时收敛为 unchanged，不当作错误。
//
// 后端 DELETE /api/watches/{id} 对"目标不存在"和"真实内部错误"返回同一个 500
// + 固定文案「删除失败」，响应体上无法区分（backend/internal/store/store.go 的
// DeleteWatch：RowsAffected==0 与真实数据库错误都被包成同一种 error，handler 又统一
// 映射成 500）。因此这里改用有明确 404 语义的 GET 先探测是否存在，只有存在时才真正
// 发起 DELETE；这不是绕过服务端校验，只是选择一个能可靠区分「不存在」的只读端点。
func (a *App) watchDeleteCmd(opts *globalOptions) *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "删除关注（幂等：目标已不存在时收敛为 unchanged）",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			start := time.Now()
			jsonMode := output.DecideJSON(opts.JSON)
			id := args[0]

			client, _, cmdErr := a.newClientOrErr(opts)
			if cmdErr != nil {
				return a.emitErr(opts, jsonMode, start, cmdErr)
			}

			var existing api.Watch
			if err := client.Get(cmd.Context(), "/api/watches/"+url.PathEscape(id), nil, &existing); err != nil {
				mapped := toCmdError(err)
				if mapped.Code == output.CodeNotFound {
					exit := output.Emit(a.Stdout, a.Stderr, version.Version, start, opts.renderOptions(jsonMode, nil),
						output.Result{Data: map[string]any{"status": "unchanged", "id": id}, DryRun: dryRun}, renderWatchCreateHuman)
					return exitError(exit)
				}
				return a.emitErr(opts, jsonMode, start, mapped)
			}

			if dryRun {
				exit := output.Emit(a.Stdout, a.Stderr, version.Version, start, opts.renderOptions(jsonMode, nil),
					output.Result{Data: map[string]any{"status": "would_delete", "id": id, "watch": existing}, DryRun: true}, renderWatchCreateHuman)
				return exitError(exit)
			}

			if err := client.Delete(cmd.Context(), "/api/watches/"+url.PathEscape(id)); err != nil {
				return a.emitErr(opts, jsonMode, start, toCmdError(err))
			}
			exit := output.Emit(a.Stdout, a.Stderr, version.Version, start, opts.renderOptions(jsonMode, nil),
				output.Result{Data: map[string]any{"status": "deleted", "id": id}}, renderWatchCreateHuman)
			return exitError(exit)
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "只预演不实际删除，绝不发起写请求")
	return cmd
}

func renderWatchListHuman(w io.Writer, data any, _ bool) error {
	list, ok := data.(api.WatchListResponse)
	if !ok {
		return renderGenericJSONHuman(w, data, false)
	}
	if len(list.Watches) == 0 {
		_, err := fmt.Fprintln(w, "暂无关注。")
		return err
	}
	for _, wt := range list.Watches {
		if _, err := fmt.Fprintf(w, "  %s  %-20s  %s  场景:%s\n", wt.ID, wt.Name, wt.Place.DisplayName, strings.Join(wt.Scenes, ",")); err != nil {
			return err
		}
	}
	return nil
}

func renderWatchHuman(w io.Writer, data any, _ bool) error {
	wt, ok := data.(api.Watch)
	if !ok {
		return renderGenericJSONHuman(w, data, false)
	}
	_, err := fmt.Fprintf(w, "%s\n地点：%s (%.4f, %.4f)\n模板：%s  时间模式：%s\n场景：%s\n",
		wt.Name, wt.Place.DisplayName, wt.Place.Latitude, wt.Place.Longitude, wt.Template, wt.TimeMode, strings.Join(wt.Scenes, ", "))
	return err
}

func renderWatchCreateHuman(w io.Writer, data any, _ bool) error {
	m, ok := data.(map[string]any)
	if !ok {
		return renderGenericJSONHuman(w, data, false)
	}
	status, _ := m["status"].(string)
	_, err := fmt.Fprintf(w, "状态：%s\n", status)
	return err
}

func renderGenericJSONHuman(w io.Writer, data any, _ bool) error {
	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(w, string(raw))
	return err
}
