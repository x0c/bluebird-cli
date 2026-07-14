package cmdtree

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"time"

	"github.com/x0c/bluebird-cli/internal/api"
)

// fakeWatch 是集成测试用的内存 watch 记录，字段与 api.Watch 对齐。
type fakeWatch struct {
	ID            string         `json:"id"`
	Name          string         `json:"name"`
	Template      string         `json:"template"`
	Place         api.WatchPlace `json:"place"`
	TimeMode      string         `json:"timeMode"`
	TargetDate    string         `json:"targetDate,omitempty"`
	RepeatRule    string         `json:"repeatRule,omitempty"`
	Scenes        []string       `json:"scenes"`
	VerifyEnabled bool           `json:"verifyEnabled,omitempty"`
}

// fakeBackend 是对 Bluebird 后端关键行为的最小内存模拟，供集成测试驱动真实 cmdtree 命令树：
// 覆盖鉴权失败、watch CRUD、写请求计数（供 dry-run 断言）、以及后端「删除不存在的关注返回
// 500 而非 404」这个已知缺陷（用来回归验证 watch delete 的 GET-先探测workaround 仍然有效）。
type fakeBackend struct {
	mu           sync.Mutex
	watches      map[string]*fakeWatch
	nextID       int
	writeCount   int
	forceStatus  map[string]int
	slowPaths    map[string]time.Duration
	placeResults []api.PlaceResult
}

func newFakeBackend() *fakeBackend {
	return &fakeBackend{
		watches:     map[string]*fakeWatch{},
		forceStatus: map[string]int{},
		slowPaths:   map[string]time.Duration{},
	}
}

func (f *fakeBackend) server() *httptest.Server {
	return httptest.NewServer(f)
}

func (f *fakeBackend) WriteCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.writeCount
}

func (f *fakeBackend) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if delay, ok := f.slowPaths[r.Method+" "+r.URL.Path]; ok {
		time.Sleep(delay)
	}

	if r.URL.Path != "/healthz" {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			writeFakeErr(w, http.StatusUnauthorized, "请先登录")
			return
		}
		if authHeader == "Bearer invalid-token" {
			writeFakeErr(w, http.StatusUnauthorized, "JWT 验证失败")
			return
		}
	}

	if status, ok := f.forceStatus[r.Method+" "+r.URL.Path]; ok {
		writeFakeErr(w, status, fmt.Sprintf("模拟状态码 %d", status))
		return
	}

	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/healthz":
		writeFakeJSON(w, http.StatusOK, map[string]any{"status": "ok"})

	case r.Method == http.MethodGet && r.URL.Path == "/api/places/search":
		writeFakeJSON(w, http.StatusOK, map[string]any{"results": f.placeResults, "total": len(f.placeResults)})

	case r.Method == http.MethodGet && r.URL.Path == "/api/watches/overview":
		writeFakeJSON(w, http.StatusOK, map[string]any{"items": []any{}, "total": 0})

	case r.Method == http.MethodGet && r.URL.Path == "/api/watches":
		list := make([]*fakeWatch, 0, len(f.watches))
		for _, wt := range f.watches {
			list = append(list, wt)
		}
		writeFakeJSON(w, http.StatusOK, map[string]any{"watches": list, "total": len(list)})

	case r.Method == http.MethodPost && r.URL.Path == "/api/watches":
		f.writeCount++
		var req api.CreateWatchRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		if strings.TrimSpace(req.Name) == "" {
			writeFakeErr(w, http.StatusBadRequest, "关注名称不能为空")
			return
		}
		f.nextID++
		id := fmt.Sprintf("test-watch-%d", f.nextID)
		wt := &fakeWatch{
			ID: id, Name: req.Name, Template: fakeOrDefault(req.Template, "trip"), Place: req.Place,
			TimeMode: fakeOrDefault(req.TimeMode, "unspecified"), TargetDate: req.TargetDate,
			RepeatRule: req.RepeatRule, Scenes: req.Scenes, VerifyEnabled: req.VerifyEnabled,
		}
		f.watches[id] = wt
		writeFakeJSON(w, http.StatusCreated, wt)

	case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/outlook"):
		id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/watches/"), "/outlook")
		if _, ok := f.watches[id]; !ok {
			writeFakeErr(w, http.StatusNotFound, "关注不存在或无权限访问")
			return
		}
		writeFakeJSON(w, http.StatusOK, map[string]any{"watchId": id, "days": []any{}})

	case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/climate"):
		id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/watches/"), "/climate")
		if _, ok := f.watches[id]; !ok {
			writeFakeErr(w, http.StatusNotFound, "关注不存在或无权限访问")
			return
		}
		writeFakeJSON(w, http.StatusOK, map[string]any{"watchId": id, "summary": "测试气候画像"})

	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/watches/"):
		id := strings.TrimPrefix(r.URL.Path, "/api/watches/")
		wt, ok := f.watches[id]
		if !ok {
			writeFakeErr(w, http.StatusNotFound, "关注不存在或无权限访问")
			return
		}
		writeFakeJSON(w, http.StatusOK, wt)

	case r.Method == http.MethodPut && strings.HasPrefix(r.URL.Path, "/api/watches/"):
		id := strings.TrimPrefix(r.URL.Path, "/api/watches/")
		wt, ok := f.watches[id]
		if !ok {
			writeFakeErr(w, http.StatusNotFound, "关注不存在或无权限访问")
			return
		}
		f.writeCount++
		var req api.UpdateWatchRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.Name != nil {
			wt.Name = *req.Name
		}
		if req.Template != nil {
			wt.Template = *req.Template
		}
		if req.TimeMode != nil {
			wt.TimeMode = *req.TimeMode
		}
		if req.TargetDate != nil {
			wt.TargetDate = *req.TargetDate
		}
		if req.RepeatRule != nil {
			wt.RepeatRule = *req.RepeatRule
		}
		if req.VerifyEnabled != nil {
			wt.VerifyEnabled = *req.VerifyEnabled
		}
		if req.Place != nil {
			wt.Place = *req.Place
		}
		if req.Scenes != nil {
			wt.Scenes = req.Scenes
		}
		writeFakeJSON(w, http.StatusOK, wt)

	case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/api/watches/"):
		id := strings.TrimPrefix(r.URL.Path, "/api/watches/")
		f.writeCount++
		if _, ok := f.watches[id]; !ok {
			// 复刻真实后端已知缺陷：删除不存在的关注返回 500 而非 404（见 AGENTS.md 已知取舍）。
			writeFakeErr(w, http.StatusInternalServerError, "删除失败")
			return
		}
		delete(f.watches, id)
		w.WriteHeader(http.StatusNoContent)

	case r.Method == http.MethodGet && r.URL.Path == "/api/conditions/now":
		writeFakeJSON(w, http.StatusOK, map[string]any{"scores": []any{}, "windows": []any{}})

	case r.Method == http.MethodGet && r.URL.Path == "/api/astro/calendar":
		writeFakeJSON(w, http.StatusOK, map[string]any{"days": []any{}})

	default:
		writeFakeErr(w, http.StatusNotFound, "fake backend 未实现："+r.Method+" "+r.URL.Path)
	}
}

func writeFakeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeFakeErr(w http.ResponseWriter, status int, msg string) {
	writeFakeJSON(w, status, map[string]string{"error": msg})
}

func fakeOrDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
