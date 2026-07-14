package api

// 本文件手工镜像后端契约中 CLI 需要按字段读写的类型（地名解析、watch 幂等收敛）。
// 其余只读透传的端点（conditions/astro/air）直接解码为 map[string]any 转发给调用方，
// 避免手写结构体与后端字段名长期漂移。以 backend/docs/API_CONTRACTS_KNOWLEDGE_BASE.md 为准。

// PlaceResult 对应 GET /api/places/search 的单条候选地点。
type PlaceResult struct {
	DisplayName string  `json:"displayName"`
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
	Elevation   float64 `json:"elevation"`
	Country     string  `json:"country"`
}

// PlacesSearchResponse 是 GET /api/places/search 的响应体。
type PlacesSearchResponse struct {
	Results []PlaceResult `json:"results"`
	Total   int           `json:"total"`
}

// WatchPlace 是 Watch 内嵌的地点对象。
type WatchPlace struct {
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
	Elevation   float64 `json:"elevation,omitempty"`
	DisplayName string  `json:"displayName,omitempty"`
}

// Watch 对应后端 model.Watch；只镜像 CLI 需要读取比对的字段，
// 其余后端字段（如聚合缓存列）经 json 保留在 Extra 里一并透传给调用方。
type Watch struct {
	ID            string     `json:"id"`
	Name          string     `json:"name"`
	Template      string     `json:"template"`
	Place         WatchPlace `json:"place"`
	TimeMode      string     `json:"timeMode"`
	TargetDate    string     `json:"targetDate,omitempty"`
	RepeatRule    string     `json:"repeatRule,omitempty"`
	Scenes        []string   `json:"scenes"`
	VerifyEnabled bool       `json:"verifyEnabled,omitempty"`
}

// WatchListResponse 是 GET /api/watches 的响应体。
type WatchListResponse struct {
	Watches []Watch `json:"watches"`
	Total   int     `json:"total"`
}

// CreateWatchRequest 是 POST /api/watches 的请求体。
type CreateWatchRequest struct {
	Name          string     `json:"name"`
	Template      string     `json:"template,omitempty"`
	Place         WatchPlace `json:"place"`
	TimeMode      string     `json:"timeMode,omitempty"`
	TargetDate    string     `json:"targetDate,omitempty"`
	RepeatRule    string     `json:"repeatRule,omitempty"`
	Scenes        []string   `json:"scenes,omitempty"`
	VerifyEnabled bool       `json:"verifyEnabled,omitempty"`
}

// UpdateWatchRequest 是 PUT /api/watches/{id} 的请求体；指针字段为 nil 表示不改该字段。
type UpdateWatchRequest struct {
	Name          *string     `json:"name,omitempty"`
	Template      *string     `json:"template,omitempty"`
	Place         *WatchPlace `json:"place,omitempty"`
	TimeMode      *string     `json:"timeMode,omitempty"`
	TargetDate    *string     `json:"targetDate,omitempty"`
	RepeatRule    *string     `json:"repeatRule,omitempty"`
	Scenes        []string    `json:"scenes,omitempty"`
	VerifyEnabled *bool       `json:"verifyEnabled,omitempty"`
}
