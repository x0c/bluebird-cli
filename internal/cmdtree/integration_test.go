package cmdtree

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/x0c/bluebird-cli/internal/api"
	"github.com/x0c/bluebird-cli/internal/output"
)

func newTestApp(baseURL string, extraEnv map[string]string) (*App, *bytes.Buffer) {
	stdout := &bytes.Buffer{}
	env := map[string]string{
		"BLUEBIRD_BASE_URL": baseURL,
		"BLUEBIRD_TOKEN":    "test-token",
	}
	for k, v := range extraEnv {
		env[k] = v
	}
	app := &App{
		Stdout: stdout,
		Stderr: &bytes.Buffer{},
		Stdin:  strings.NewReader(""),
		Getenv: func(k string) string { return env[k] },
	}
	return app, stdout
}

func exitCodeOf(err error) int {
	if err == nil {
		return output.ExitOK
	}
	var ee *output.ExitError
	if errors.As(err, &ee) {
		return ee.Code
	}
	return -999
}

// runCmd 总是显式带 --json，使测试结果不依赖 go test 进程本身的 stdout 是否为 TTY。
func runCmd(t *testing.T, app *App, stdout *bytes.Buffer, args ...string) (map[string]any, int) {
	t.Helper()
	stdout.Reset()
	root := app.Build()
	root.SetArgs(append(args, "--json"))
	err := root.Execute()
	exitCode := exitCodeOf(err)
	var env map[string]any
	if stdout.Len() > 0 {
		if uerr := json.Unmarshal(stdout.Bytes(), &env); uerr != nil {
			t.Fatalf("stdout 不是合法 JSON: %v\nraw: %s", uerr, stdout.String())
		}
	}
	return env, exitCode
}

func TestIntegration_DryRunWatchCreate_NoWriteRequests(t *testing.T) {
	backend := newFakeBackend()
	server := backend.server()
	defer server.Close()
	app, stdout := newTestApp(server.URL, nil)

	env, exit := runCmd(t, app, stdout, "watch", "create", "--name", "测试关注", "--lat", "30", "--lon", "120", "--dry-run")

	if exit != output.ExitOK {
		t.Fatalf("exit = %d, want 0; envelope=%+v", exit, env)
	}
	if !env["ok"].(bool) {
		t.Fatalf("expected ok=true, got %+v", env)
	}
	data := env["data"].(map[string]any)
	if data["status"] != "would_create" {
		t.Fatalf("status = %v, want would_create", data["status"])
	}
	if backend.WriteCount() != 0 {
		t.Fatalf("dry-run must not send any write request, got writeCount=%d", backend.WriteCount())
	}
}

func TestIntegration_WatchCreate_SecondCallConvergesUnchanged(t *testing.T) {
	backend := newFakeBackend()
	server := backend.server()
	defer server.Close()
	app, stdout := newTestApp(server.URL, nil)

	env1, exit1 := runCmd(t, app, stdout, "watch", "create", "--name", "西湖晚霞", "--lat", "30.25", "--lon", "120.15", "--scenes", "sunset_glow")
	if exit1 != output.ExitOK {
		t.Fatalf("first create exit = %d, want 0; envelope=%+v", exit1, env1)
	}
	data1 := env1["data"].(map[string]any)
	if data1["status"] != "created" {
		t.Fatalf("first create status = %v, want created", data1["status"])
	}
	if backend.WriteCount() != 1 {
		t.Fatalf("expected exactly 1 write after first create, got %d", backend.WriteCount())
	}

	env2, exit2 := runCmd(t, app, stdout, "watch", "create", "--name", "西湖晚霞", "--lat", "30.25", "--lon", "120.15", "--scenes", "sunset_glow")
	if exit2 != output.ExitOK {
		t.Fatalf("second create exit = %d, want 0; envelope=%+v", exit2, env2)
	}
	data2 := env2["data"].(map[string]any)
	if data2["status"] != "unchanged" {
		t.Fatalf("second create status = %v, want unchanged (idempotent convergence)", data2["status"])
	}
	if backend.WriteCount() != 1 {
		t.Fatalf("second (converged) create must not add a write, still expect 1, got %d", backend.WriteCount())
	}
}

func TestIntegration_WatchCreate_ConflictOnFieldMismatch(t *testing.T) {
	backend := newFakeBackend()
	server := backend.server()
	defer server.Close()
	app, stdout := newTestApp(server.URL, nil)

	_, exit1 := runCmd(t, app, stdout, "watch", "create", "--name", "冲突", "--lat", "30", "--lon", "120", "--scenes", "sunset_glow")
	if exit1 != output.ExitOK {
		t.Fatalf("first create exit = %d, want 0", exit1)
	}

	env2, exit2 := runCmd(t, app, stdout, "watch", "create", "--name", "冲突", "--lat", "30", "--lon", "120", "--scenes", "starry_sky")
	if exit2 != output.ExitConflict {
		t.Fatalf("exit = %d, want %d (conflict); envelope=%+v", exit2, output.ExitConflict, env2)
	}
	errBody := env2["error"].(map[string]any)
	if errBody["code"] != output.CodeWatchConflict {
		t.Fatalf("error.code = %v, want %s", errBody["code"], output.CodeWatchConflict)
	}
}

func TestIntegration_WatchDelete_SecondCallConvergesUnchanged(t *testing.T) {
	// 回归验证：fake backend 复刻了真实后端「删除不存在的关注返回 500」的已知缺陷，
	// watch delete 的 GET-先探测 workaround 必须仍能把第二次删除收敛为 unchanged。
	backend := newFakeBackend()
	server := backend.server()
	defer server.Close()
	app, stdout := newTestApp(server.URL, nil)

	env1, exit1 := runCmd(t, app, stdout, "watch", "create", "--name", "待删除", "--lat", "10", "--lon", "20")
	if exit1 != output.ExitOK {
		t.Fatalf("create exit = %d, want 0", exit1)
	}
	id := env1["data"].(map[string]any)["watch"].(map[string]any)["id"].(string)

	env2, exit2 := runCmd(t, app, stdout, "watch", "delete", id)
	if exit2 != output.ExitOK {
		t.Fatalf("first delete exit = %d, want 0; envelope=%+v", exit2, env2)
	}
	if env2["data"].(map[string]any)["status"] != "deleted" {
		t.Fatalf("first delete status = %v, want deleted", env2["data"].(map[string]any)["status"])
	}

	env3, exit3 := runCmd(t, app, stdout, "watch", "delete", id)
	if exit3 != output.ExitOK {
		t.Fatalf("second delete exit = %d, want 0 (idempotent unchanged); envelope=%+v", exit3, env3)
	}
	if env3["data"].(map[string]any)["status"] != "unchanged" {
		t.Fatalf("second delete status = %v, want unchanged", env3["data"].(map[string]any)["status"])
	}
}

func TestIntegration_WatchDeleteDryRun_NoWriteRequests(t *testing.T) {
	backend := newFakeBackend()
	server := backend.server()
	defer server.Close()
	app, stdout := newTestApp(server.URL, nil)

	env1, _ := runCmd(t, app, stdout, "watch", "create", "--name", "预演删除", "--lat", "10", "--lon", "20")
	id := env1["data"].(map[string]any)["watch"].(map[string]any)["id"].(string)
	writesAfterCreate := backend.WriteCount()

	env2, exit2 := runCmd(t, app, stdout, "watch", "delete", id, "--dry-run")
	if exit2 != output.ExitOK {
		t.Fatalf("dry-run delete exit = %d, want 0; envelope=%+v", exit2, env2)
	}
	if env2["data"].(map[string]any)["status"] != "would_delete" {
		t.Fatalf("status = %v, want would_delete", env2["data"].(map[string]any)["status"])
	}
	if backend.WriteCount() != writesAfterCreate {
		t.Fatalf("dry-run delete must not send DELETE, writeCount changed from %d to %d", writesAfterCreate, backend.WriteCount())
	}
}

func TestIntegration_Unauthorized_Exit4(t *testing.T) {
	backend := newFakeBackend()
	server := backend.server()
	defer server.Close()
	app, stdout := newTestApp(server.URL, map[string]string{"BLUEBIRD_TOKEN": "invalid-token"})

	env, exit := runCmd(t, app, stdout, "watch", "list")
	if exit != output.ExitAuth {
		t.Fatalf("exit = %d, want %d; envelope=%+v", exit, output.ExitAuth, env)
	}
	if env["error"].(map[string]any)["code"] != output.CodeUnauthorized {
		t.Fatalf("error.code = %v, want %s", env["error"].(map[string]any)["code"], output.CodeUnauthorized)
	}
}

func TestIntegration_NotFound_Exit3(t *testing.T) {
	backend := newFakeBackend()
	server := backend.server()
	defer server.Close()
	app, stdout := newTestApp(server.URL, nil)

	env, exit := runCmd(t, app, stdout, "watch", "get", "does-not-exist")
	if exit != output.ExitNotFound {
		t.Fatalf("exit = %d, want %d; envelope=%+v", exit, output.ExitNotFound, env)
	}
}

func TestIntegration_BadRequest_Exit2(t *testing.T) {
	backend := newFakeBackend()
	server := backend.server()
	defer server.Close()
	app, stdout := newTestApp(server.URL, nil)

	// CLI 侧本地校验 --name 非空，走的是 CodeUsageError（本地即拒绝，不发请求）。
	env, exit := runCmd(t, app, stdout, "watch", "create", "--name", "", "--lat", "10", "--lon", "20")
	if exit != output.ExitUsage {
		t.Fatalf("exit = %d, want %d; envelope=%+v", exit, output.ExitUsage, env)
	}
}

func TestIntegration_ServerError_Exit1(t *testing.T) {
	backend := newFakeBackend()
	server := backend.server()
	defer server.Close()
	backend.forceStatus["GET /api/watches"] = 500
	app, stdout := newTestApp(server.URL, nil)

	env, exit := runCmd(t, app, stdout, "watch", "list")
	if exit != output.ExitGeneral {
		t.Fatalf("exit = %d, want %d; envelope=%+v", exit, output.ExitGeneral, env)
	}
	if env["error"].(map[string]any)["code"] != output.CodeServerError {
		t.Fatalf("error.code = %v, want %s", env["error"].(map[string]any)["code"], output.CodeServerError)
	}
}

func TestIntegration_Timeout_Exit6(t *testing.T) {
	backend := newFakeBackend()
	server := backend.server()
	defer server.Close()
	backend.slowPaths["GET /api/watches"] = 200 * time.Millisecond
	app, stdout := newTestApp(server.URL, nil)

	env, exit := runCmd(t, app, stdout, "watch", "list", "--timeout", "50ms")
	if exit != output.ExitTimeout {
		t.Fatalf("exit = %d, want %d; envelope=%+v", exit, output.ExitTimeout, env)
	}
}

func TestIntegration_ConditionsNow_EnvelopeValid(t *testing.T) {
	backend := newFakeBackend()
	server := backend.server()
	defer server.Close()
	app, stdout := newTestApp(server.URL, nil)

	env, exit := runCmd(t, app, stdout, "conditions", "now", "--lat", "30", "--lon", "120")
	if exit != output.ExitOK {
		t.Fatalf("exit = %d, want 0; envelope=%+v", exit, env)
	}
	if !env["ok"].(bool) {
		t.Fatalf("expected ok=true, got %+v", env)
	}
}

func TestIntegration_PlaceAmbiguous_Exit5WithCandidates(t *testing.T) {
	backend := newFakeBackend()
	server := backend.server()
	defer server.Close()
	backend.placeResults = []api.PlaceResult{
		{DisplayName: "中山市, 广东省", Latitude: 22.5, Longitude: 113.3},
		{DisplayName: "中山区, 大连市", Latitude: 38.9, Longitude: 121.6},
	}
	app, stdout := newTestApp(server.URL, nil)

	env, exit := runCmd(t, app, stdout, "conditions", "now", "--place", "中山")
	if exit != output.ExitConflict {
		t.Fatalf("exit = %d, want %d; envelope=%+v", exit, output.ExitConflict, env)
	}
	errBody := env["error"].(map[string]any)
	if errBody["code"] != output.CodePlaceAmbiguous {
		t.Fatalf("error.code = %v, want %s", errBody["code"], output.CodePlaceAmbiguous)
	}
	details := errBody["details"].(map[string]any)
	candidates := details["candidates"].([]any)
	if len(candidates) != 2 {
		t.Fatalf("expected 2 candidates in error.details, got %d", len(candidates))
	}
}

func TestIntegration_PlaceFirst_SkipsAmbiguity(t *testing.T) {
	backend := newFakeBackend()
	server := backend.server()
	defer server.Close()
	backend.placeResults = []api.PlaceResult{
		{DisplayName: "候选一", Latitude: 22.5, Longitude: 113.3},
		{DisplayName: "候选二", Latitude: 38.9, Longitude: 121.6},
	}
	app, stdout := newTestApp(server.URL, nil)

	env, exit := runCmd(t, app, stdout, "conditions", "now", "--place", "中山", "--first")
	if exit != output.ExitOK {
		t.Fatalf("exit = %d, want 0 with --first; envelope=%+v", exit, env)
	}
	resolved := env["meta"].(map[string]any)["resolved_place"].(map[string]any)
	if resolved["displayName"] != "候选一" {
		t.Fatalf("resolved_place.displayName = %v, want 候选一", resolved["displayName"])
	}
}
