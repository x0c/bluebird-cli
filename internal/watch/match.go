// Package watch 实现 watch create 的幂等收敛比较逻辑：判断一个已存在的 Watch
// 是否就是这次 create 想要确保存在的那个（同名同地点），以及其余字段是否完全一致。
// 后端没有幂等键，这套比较逻辑是 CLI 侧收敛的唯一依据，独立成包便于离线单测覆盖。
package watch

import (
	"sort"

	"github.com/x0c/bluebird-cli/internal/api"
	"github.com/x0c/bluebird-cli/internal/place"
)

// SameLocation 报告两个坐标是否落在同一 3 位小数（约 110 米）网格内。
func SameLocation(a, b api.WatchPlace) bool {
	return place.RoundKey(a.Latitude) == place.RoundKey(b.Latitude) &&
		place.RoundKey(a.Longitude) == place.RoundKey(b.Longitude)
}

// MatchesKey 报告 existing 是否是 desired 的收敛目标：同名 + 同地点网格。
func MatchesKey(existing api.Watch, desiredName string, desiredPlace api.WatchPlace) bool {
	return existing.Name == desiredName && SameLocation(existing.Place, desiredPlace)
}

// FieldsEqual 报告收敛到的 existing 是否与 desired 的其余字段完全一致：
// template/timeMode 精确比较、targetDate 只比日期部分、repeatRule 精确比较、
// scenes 按集合（忽略顺序）比较、verifyEnabled 精确比较。
func FieldsEqual(existing api.Watch, desired api.CreateWatchRequest) bool {
	if existing.Template != orDefault(desired.Template, "trip") {
		return false
	}
	if existing.TimeMode != orDefault(desired.TimeMode, "unspecified") {
		return false
	}
	if NormalizeDate(existing.TargetDate) != NormalizeDate(desired.TargetDate) {
		return false
	}
	if existing.RepeatRule != desired.RepeatRule {
		return false
	}
	if existing.VerifyEnabled != desired.VerifyEnabled {
		return false
	}
	return ScenesEqual(existing.Scenes, desired.Scenes)
}

// NormalizeDate 只保留日期部分（YYYY-MM-DD），兼容后端可能把日期规范化成带时间戳的字符串。
func NormalizeDate(s string) string {
	if len(s) >= 10 {
		return s[:10]
	}
	return s
}

// ScenesEqual 忽略顺序比较两个场景集合是否相同。
func ScenesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	ac := append([]string(nil), a...)
	bc := append([]string(nil), b...)
	sort.Strings(ac)
	sort.Strings(bc)
	for i := range ac {
		if ac[i] != bc[i] {
			return false
		}
	}
	return true
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
