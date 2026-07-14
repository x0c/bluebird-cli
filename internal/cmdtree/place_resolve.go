package cmdtree

import (
	"bufio"
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/x0c/bluebird-cli/internal/api"
	"github.com/x0c/bluebird-cli/internal/output"
	"github.com/x0c/bluebird-cli/internal/place"
)

// placeFlags 是坐标类命令（conditions/astro/air/watch create）共用的地点参数。
type placeFlags struct {
	Place     string
	Lat       float64
	Lon       float64
	Elevation float64
	First     bool
}

func addPlaceFlags(cmd *cobra.Command, pf *placeFlags) {
	cmd.Flags().StringVar(&pf.Place, "place", "", "地名（与 --lat/--lon 互斥，由 CLI 内部先做地名搜索解析为坐标）")
	cmd.Flags().Float64Var(&pf.Lat, "lat", 0, "纬度")
	cmd.Flags().Float64Var(&pf.Lon, "lon", 0, "经度")
	cmd.Flags().Float64Var(&pf.Elevation, "elevation", 0, "海拔（米，可选，仅与 --lat/--lon 一起使用）")
	cmd.Flags().BoolVar(&pf.First, "first", false, "地名有多个候选时直接取第一个，跳过歧义确认")
}

func placeInputFrom(cmd *cobra.Command, pf *placeFlags) place.Input {
	in := place.Input{PlaceQuery: pf.Place, First: pf.First}
	if cmd.Flags().Changed("lat") && cmd.Flags().Changed("lon") {
		lat, lon := pf.Lat, pf.Lon
		in.Lat, in.Lon = &lat, &lon
		if cmd.Flags().Changed("elevation") {
			elev := pf.Elevation
			in.Elevation = &elev
		}
	}
	return in
}

// resolvePlace 先走 place.Resolve 的纯逻辑分支；歧义时若 stdin 是真实终端且未禁交互，
// 现场用编号列表让人选，否则返回码 5 歧义错误（供 Agent 一次往返自愈）。
func (a *App) resolvePlace(ctx context.Context, client *api.Client, in place.Input, noInput bool) (*place.Result, *output.CmdError) {
	search := func(query string) ([]api.PlaceResult, error) {
		return client.SearchPlaces(ctx, query)
	}
	result, cmdErr := place.Resolve(in, search)
	if cmdErr != nil {
		return nil, cmdErr
	}
	if result == nil || result.Outcome == place.OutcomeCoordinates {
		return result, nil
	}

	if !noInput && output.StdinIsTTY() {
		chosen, err := a.promptPlaceChoice(result.Candidates)
		if err != nil {
			return nil, output.NewError(output.CodeUsageError, "交互选择失败："+err.Error())
		}
		return &place.Result{
			Outcome: place.OutcomeCoordinates, Latitude: chosen.Latitude, Longitude: chosen.Longitude,
			Elevation: chosen.Elevation, Resolved: chosen,
		}, nil
	}
	return nil, place.AmbiguousError(in.PlaceQuery, result.Candidates)
}

func (a *App) promptPlaceChoice(candidates []api.PlaceResult) (*api.PlaceResult, error) {
	for i, c := range candidates {
		_, _ = fmt.Fprintf(a.Stdout, "  [%d] %s (%.4f, %.4f)\n", i+1, c.DisplayName, c.Latitude, c.Longitude)
	}
	_, _ = fmt.Fprint(a.Stdout, "请选择编号：")
	reader := bufio.NewReader(a.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return nil, err
	}
	idx, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil || idx < 1 || idx > len(candidates) {
		return nil, fmt.Errorf("无效的编号：%s", strings.TrimSpace(line))
	}
	return &candidates[idx-1], nil
}
