package cmdtree

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"time"

	"github.com/spf13/cobra"
	"github.com/x0c/bluebird-cli/internal/output"
	"github.com/x0c/bluebird-cli/internal/version"
)

func (a *App) airCmd(opts *globalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "air",
		Short: "空气质量查询",
	}
	cmd.AddCommand(a.airCurrentCmd(opts), a.airLatestCmd(opts), a.airStationsCmd(opts))
	return cmd
}

// airCurrentCmd 查询当前空气质量：后端按用户最近上报的位置查附近站点，
// 不接受 lat/lng 参数（与 stations 不同，这里没有地点 flag 是刻意的，不是遗漏）。
func (a *App) airCurrentCmd(opts *globalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "current",
		Short: "查询当前空气质量（按最近上报位置，不接受 --place/--lat/--lon）",
		RunE: func(cmd *cobra.Command, args []string) error {
			start := time.Now()
			jsonMode := output.DecideJSON(opts.JSON)
			client, _, cmdErr := a.newClientOrErr(opts)
			if cmdErr != nil {
				return a.emitErr(opts, jsonMode, start, cmdErr)
			}
			raw, err := client.GetRaw(cmd.Context(), "/api/air-quality/current", nil)
			if err != nil {
				mapped := toCmdError(err)
				if mapped.Code == output.CodeNotFound {
					mapped = output.NewError(output.CodeNoLocation, "还没有最近上报的位置").
						WithHint("该端点不支持指定坐标；需要先有一次真实位置上报（如通过 iOS 客户端），或改用 bluebird air stations --place 查询附近站点")
				}
				return a.emitErr(opts, jsonMode, start, mapped)
			}
			exit := output.Emit(a.Stdout, a.Stderr, version.Version, start, opts.renderOptions(jsonMode, nil),
				output.Result{Data: emptyOrDecode(raw)}, renderAirHuman)
			return exitError(exit)
		},
	}
}

func (a *App) airStationsCmd(opts *globalOptions) *cobra.Command {
	pf := &placeFlags{}
	cmd := &cobra.Command{
		Use:   "stations",
		Short: "查询附近空气质量监测站（必须指定地点）",
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runAirStationsQuery(cmd, opts, pf)
		},
	}
	addPlaceFlags(cmd, pf)
	return cmd
}

func (a *App) airLatestCmd(opts *globalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "latest",
		Short: "查询最近一次记录的空气质量（不需要地点）",
		RunE: func(cmd *cobra.Command, args []string) error {
			start := time.Now()
			jsonMode := output.DecideJSON(opts.JSON)
			client, _, cmdErr := a.newClientOrErr(opts)
			if cmdErr != nil {
				return a.emitErr(opts, jsonMode, start, cmdErr)
			}
			raw, err := client.GetRaw(cmd.Context(), "/api/air-quality/latest", nil)
			if err != nil {
				return a.emitErr(opts, jsonMode, start, toCmdError(err))
			}
			exit := output.Emit(a.Stdout, a.Stderr, version.Version, start, opts.renderOptions(jsonMode, nil),
				output.Result{Data: emptyOrDecode(raw)}, renderAirHuman)
			return exitError(exit)
		},
	}
}

// runAirStationsQuery 是 stations 端点的坐标查询逻辑；该端点参数名是 lat+lng（不是 lon），
// 只在这里做一次转换，不让这个契约细节泄漏到命令面的 --lon flag。
func (a *App) runAirStationsQuery(cmd *cobra.Command, opts *globalOptions, pf *placeFlags) error {
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
	if result == nil {
		return a.emitErr(opts, jsonMode, start,
			output.NewError(output.CodeUsageError, "该命令必须指定地点").WithHint("加 --place <地名> 或 --lat/--lon"))
	}

	query := url.Values{}
	query.Set("lat", strconv.FormatFloat(result.Latitude, 'f', -1, 64))
	query.Set("lng", strconv.FormatFloat(result.Longitude, 'f', -1, 64))

	raw, err := client.GetRaw(cmd.Context(), "/api/air-quality/stations", query)
	if err != nil {
		return a.emitErr(opts, jsonMode, start, toCmdError(err))
	}

	var resolvedPlace any
	if result.Resolved != nil {
		resolvedPlace = result.Resolved
	}
	exit := output.Emit(a.Stdout, a.Stderr, version.Version, start, opts.renderOptions(jsonMode, nil),
		output.Result{Data: emptyOrDecode(raw), ResolvedPlace: resolvedPlace}, renderAirHuman)
	return exitError(exit)
}

func emptyOrDecode(raw []byte) any {
	if len(raw) == 0 {
		return map[string]any{"available": false}
	}
	var data any
	if err := json.Unmarshal(raw, &data); err != nil {
		return map[string]any{"available": false}
	}
	return data
}

func renderAirHuman(w io.Writer, data any, _ bool) error {
	switch v := data.(type) {
	case map[string]any:
		if available, ok := v["available"].(bool); ok && !available {
			_, err := fmt.Fprintln(w, "暂无可用的空气质量数据。")
			return err
		}
		raw, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(w, string(raw))
		return err
	case []any:
		if len(v) == 0 {
			_, err := fmt.Fprintln(w, "附近暂无监测站。")
			return err
		}
		raw, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(w, string(raw))
		return err
	default:
		_, err := fmt.Fprintln(w, data)
		return err
	}
}
