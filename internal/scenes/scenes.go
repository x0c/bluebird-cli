// Package scenes 是 Bluebird 12 个气象场景枚举的单一权威来源，
// 供参数校验、人类渲染、describe 三处同源引用，避免各处各自维护一份漂移的列表。
package scenes

// Scene 是一个气象场景枚举项。
type Scene struct {
	ID     string `json:"id"`
	NameZH string `json:"name_zh"`
}

// All 是后端支持的全部 12 个场景，顺序与 backend AGENTS.md 的产品优先级一致。
var All = []Scene{
	{ID: "crystal_sky", NameZH: "水晶天"},
	{ID: "sunset_glow", NameZH: "晚霞"},
	{ID: "sunrise_glow", NameZH: "朝霞"},
	{ID: "crepuscular_rays", NameZH: "云隙光"},
	{ID: "rainbow", NameZH: "彩虹"},
	{ID: "fog_mood", NameZH: "雾境"},
	{ID: "snow_rime_frost", NameZH: "雪凇"},
	{ID: "outdoor_comfort", NameZH: "户外舒适"},
	{ID: "starry_sky", NameZH: "星空"},
	{ID: "moonrise", NameZH: "月升"},
	{ID: "blue_hour", NameZH: "蓝调时刻"},
	{ID: "light_pillar", NameZH: "光柱"},
}

var nameByID = buildIndex()

func buildIndex() map[string]string {
	m := make(map[string]string, len(All))
	for _, s := range All {
		m[s.ID] = s.NameZH
	}
	return m
}

// Valid 报告 id 是否是已知场景。
func Valid(id string) bool {
	_, ok := nameByID[id]
	return ok
}

// NameZH 返回场景中文名；未知 id 返回空字符串。
func NameZH(id string) string {
	return nameByID[id]
}

// IDs 返回全部场景 id，用于参数校验的错误提示与 describe 输出。
func IDs() []string {
	ids := make([]string, len(All))
	for i, s := range All {
		ids[i] = s.ID
	}
	return ids
}
