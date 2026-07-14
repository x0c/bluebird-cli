// Package place 实现 bluebird 所有坐标类命令共用的地名解析策略：
// --place 与 --lat/--lon 互斥、地名零结果/单结果/多结果三分支。
// 不直接发网络请求（由调用方注入 search 函数），isTTY 也由调用方注入判断结果，
// 使整套歧义分支逻辑可以离线、确定性地被测试覆盖。
package place

import (
	"fmt"
	"math"

	"github.com/x0c/bluebird-cli/internal/api"
	"github.com/x0c/bluebird-cli/internal/output"
)

// Input 是一次地名/坐标解析的输入参数。
type Input struct {
	PlaceQuery string
	Lat        *float64
	Lon        *float64
	Elevation  *float64
	First      bool
}

// Outcome 区分「已得到坐标」与「仍需人工/Agent 决断的歧义」两种结果。
type Outcome int

const (
	// OutcomeCoordinates 表示已解析出确定坐标（直传坐标 / 零结果之外的确定命中 / --first）。
	OutcomeCoordinates Outcome = iota
	// OutcomeAmbiguous 表示地名有多个候选且既未传 --first 也不能交互确认，调用方需自行决定
	// 是否转入交互挑选（真实 TTY 场景）或直接报歧义错误。
	OutcomeAmbiguous
)

// Result 是 Resolve 的输出。
type Result struct {
	Outcome    Outcome
	Latitude   float64
	Longitude  float64
	Elevation  float64
	Resolved   *api.PlaceResult  // Outcome==OutcomeCoordinates 且来自地名搜索时非 nil，供 meta.resolved_place 使用
	Candidates []api.PlaceResult // Outcome==OutcomeAmbiguous 时的候选列表
}

// SearchFunc 是地名搜索的注入点，真实实现由 internal/api.Client.SearchPlaces 提供。
type SearchFunc func(query string) ([]api.PlaceResult, error)

// Resolve 按互斥规则解析出坐标；search 只在提供了 --place 时被调用。
func Resolve(in Input, search SearchFunc) (*Result, *output.CmdError) {
	hasPlace := in.PlaceQuery != ""
	hasCoord := in.Lat != nil && in.Lon != nil

	if hasPlace && hasCoord {
		return nil, output.NewError(output.CodeUsageError, "--place 与 --lat/--lon 不能同时使用，请二选一")
	}

	if hasCoord {
		elev := 0.0
		if in.Elevation != nil {
			elev = *in.Elevation
		}
		return &Result{Outcome: OutcomeCoordinates, Latitude: *in.Lat, Longitude: *in.Lon, Elevation: elev}, nil
	}

	if !hasPlace {
		// 坐标与地名都未提供；是否为错误由调用方根据具体命令语义决定
		// （conditions 允许省略走"最近上报位置"，astro/air 视为必填）。
		return nil, nil
	}

	candidates, err := search(in.PlaceQuery)
	if err != nil {
		if cmdErr, ok := err.(*output.CmdError); ok {
			return nil, cmdErr
		}
		return nil, output.NewError(output.CodeServerError, err.Error()).WithRetryable(true)
	}

	switch {
	case len(candidates) == 0:
		return nil, output.NewError(output.CodePlaceNotFound, fmt.Sprintf("找不到地点：%s", in.PlaceQuery)).
			WithHint("换个更常见的地名关键词，或直接用 --lat/--lon 传坐标")

	case len(candidates) == 1:
		p := candidates[0]
		return &Result{Outcome: OutcomeCoordinates, Latitude: p.Latitude, Longitude: p.Longitude, Elevation: p.Elevation, Resolved: &p}, nil

	case in.First:
		p := candidates[0]
		return &Result{Outcome: OutcomeCoordinates, Latitude: p.Latitude, Longitude: p.Longitude, Elevation: p.Elevation, Resolved: &p}, nil

	default:
		top := candidates
		if len(top) > 5 {
			top = top[:5]
		}
		return &Result{Outcome: OutcomeAmbiguous, Candidates: top}, nil
	}
}

// AmbiguousError 把候选列表转换为标准的歧义错误（码 5），供非交互场景直接返回给调用方。
func AmbiguousError(query string, candidates []api.PlaceResult) *output.CmdError {
	next := make([]string, 0, 2)
	if len(candidates) > 0 {
		p := candidates[0]
		next = append(next,
			fmt.Sprintf("--lat %g --lon %g（精确指定：%s）", p.Latitude, p.Longitude, p.DisplayName),
			fmt.Sprintf("加 --first 参数直接取第一个候选（%s）", p.DisplayName),
		)
	}
	return output.NewError(output.CodePlaceAmbiguous, fmt.Sprintf("地名「%s」有多个匹配结果，无法确定唯一地点", query)).
		WithHint("从候选中选一个精确坐标，或加 --first 取第一个").
		WithNextCommands(next...).
		WithDetails(map[string]any{"candidates": candidates})
}

// RoundKey 把坐标舍入到 3 位小数（约 110 米网格），用于 watch 幂等收敛的地点相等性比较。
func RoundKey(v float64) float64 {
	return math.Round(v*1000) / 1000
}
