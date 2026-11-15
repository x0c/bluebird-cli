# Bluebird CLI（`bluebird`）Agent 规范

通用工程规范：[Go 规范](../../_standards/go.md)

`bluebird` 是 Bluebird（「等好天气」）天气发现服务的命令行客户端，面向人与大模型 Agent 双受众设计，遵循 skill `agent-friendly-cli` 的 P0/P1 契约。默认连接线上生产 `https://bluebird.caozc.top`。未来会接入 OpenConductor 管家（见文末）。

设计方案见 `/home/vibecoder/.claude/plans/bluebird-ios-go-glowing-sphinx.md`（2026-07-14，用户已批准）。

## 技术栈

- Go（`go.mod` 的 `go` 指令为下限，`go build`/CI 用当前可用工具链）。
- 依赖仅三个：`spf13/cobra`（命令树，`describe` 与参数解析同源）、`golang-jwt/jwt/v5`（与后端同库同版，HS256 行为一致）、`golang.org/x/term`（TTY 检测）。不引入 viper——配置只有几个环境变量，`os.Getenv` 收口进 `internal/auth.Config`。
- **不 import backend Go module**（backend 在内网 Forgejo，本仓库开源）；API 请求/响应类型手工镜像在 `internal/api/`，以 `backend/docs/API_CONTRACTS_KNOWLEDGE_BASE.md` 为准，变更需人工同步。

## 目录结构

```
cmd/bluebird/main.go        # 壳：构建 App、Execute、把 output.ExitError 转成 os.Exit 码
internal/cmdtree/           # cobra 命令定义，一资源一文件；App 持有注入的 IO/Getenv，全树可离线测试
internal/api/                # HTTP 客户端 + HTTP 状态→CmdError 映射（client.go / errors.go）
internal/auth/                # 鉴权配置加载 + token 解析/自签（token.go）
internal/output/              # envelope / 退出码 / TTY 检测 / --fields 投影 / 人机双路径渲染
internal/place/                # 地名解析策略，供 conditions/astro/air/watch create 复用
internal/scenes/               # 12 场景枚举 + 中文名（校验/渲染/describe 三处同源）
internal/watch/                # watch create 幂等收敛比较逻辑（match.go），独立成包便于离线单测
internal/version/              # ldflags 注入的构建信息
scripts/acceptance.sh          # 对生产真实跑一遍验收清单，逐项打印证据（见下方「验证约定」）
```

## 输出契约（对外契约，发布后只加不改不删）

- `--json`（或 stdout 非 TTY 自动开启）统一信封 `{ok, data, error, meta}`；**失败也向 stdout 写合法信封 + 非零退出码**，不是只在 stderr 打文本。`error.message` 中文，`error.code` 稳定英文短码，`error.hint`/`error.next_commands`/`error.retryable`/`error.details` 均可选。
- 退出码：0 成功(含幂等 unchanged) / 1 一般失败(5xx/网络/429/501) / 2 用法错误(参数/400) / 3 不存在(404/地名零结果/无位置) / 4 鉴权(401/403/未配置) / 5 冲突(地名歧义/watch 冲突/409) / 6 超时。单一映射见 `internal/output/exitcode.go`，改动需同步表驱动测试。
- 人类模式（无 `--json` 且 stdout 是 TTY）：成功走 stdout 可读文本，失败走 stderr 中文文本；**两条路径物理分离**（`output.Emit` 内部），不共用同一份格式化逻辑，避免机器信封被人类文案污染或反之。
- 人类渲染禁止打印裸分数（后端 `backend/AGENTS.md` 硬约束：分数只用于排序/阈值，不作产品化展示）；JSON `data` 保留后端原始字段供机器使用。
- `describe [command] --json` 遍历实际 cobra 命令树生成，与参数解析同源，不需要手动维护文档；只列各命令**自身**定义的 flag，全局 flag 只在根节点出现一次（避免体积膨胀）。

## 鉴权（环境变量，禁止落盘、禁止进日志/输出/错误信息）

优先级：`BLUEBIRD_TOKEN` 原样使用 → `BLUEBIRD_JWT_SECRET`（未设回退 `OPENCONDUCTOR_SUPABASE_JWT_SECRET`，与 TempoTask 约定一致，两产品共用同一 GoTrue 密钥）+ `BLUEBIRD_USER_ID` 自签 5 分钟 HS256（`iss` 默认 `https://auth.caozc.top`，后端配了 `JWT_ISSUER` 会强校验，见 `backend/internal/auth/auth.go`）→ 都没有则鉴权命令返回码 4。`BLUEBIRD_BASE_URL` 覆盖默认生产地址；`BLUEBIRD_JWT_ISSUER` 覆盖默认 issuer。`doctor` 只报「已配置/未配置」，不打印密钥值。

## 已知取舍（不要当作 bug 重新讨论）

- **watch 创建的幂等收敛依赖 `GET /api/watches` 全量拉取再本地比对**（后端无幂等键）。收敛键 = name 相同 + 坐标各舍入 3 位小数相同。`list→POST` 之间存在 TOCTOU 竞态窗口；单用户 CLI 场景概率可忽略，接受该风险，不做分布式锁之类的过度设计。后端未来若给 `/api/watches` 加分页，这里要同步改。
- **`--timeout` 默认 30s，不要调小**：生产 `conditions now/windows` 冷缓存首查可达 20s+（多数据源实时富化），比常见 CLI 超时默认值大得多，是刻意选择。
- **`watch delete` 先 GET 再 DELETE，不是直接调用后端 DELETE 端点**：后端 `DELETE /api/watches/{id}` 对「目标不存在」和「真实内部错误」都返回同一个 500 + 固定文案「删除失败」（`backend/internal/store/store.go` 的 `DeleteWatch`：`RowsAffected==0` 与真实数据库错误被包成同一种 error，handler 又统一映射成 500），响应体上无法区分两者。CLI 侧改用有明确 404 语义的 `GET /api/watches/{id}` 先探测存在性，只有存在时才真正发 DELETE，藉此实现设计要求的幂等收敛（2026-07-14 对生产实测发现：直接信任 DELETE 的响应会导致「删除已删除的关注」被错误地报成 `server_error` 而不是 `unchanged`）。
- **`air stations` 后端查询参数是 `lat`+`lng`（不是 `lon`）**：CLI 对外统一 `--lon`，仅在拼装该端点请求时转换成 `lng`。**`air current` 完全不接受坐标参数**（与 `stations` 不对称，2026-07-14 对生产实测发现——原计划设计把两者当同一组坐标端点处理是错的）：它只读用户最近一次 `POST /api/location` 上报的位置，没有上报过位置的账号会稳定 404，不能靠传 `--place`/`--lat/--lon` 绕过。

## 验证约定

- `go build ./...`、`go vet ./...`、`go test -race ./...` 交付前必过（`internal/cmdtree` 下的 `fakebackend_test.go`/`integration_test.go` 用内存假后端驱动真实命令树，覆盖写请求计数、幂等收敛、退出码映射；改命令面或 API 契约后优先补这里的用例，不要只加纯函数单测）。
- `golangci-lint run ./...`、`govulncheck ./...`（依赖/网络/鉴权链路变更后）。
- 涉及真实网络调用的命令（`doctor` 在线段、`conditions`/`watch`/`astro`/`air` 等）改动后，对生产 `https://bluebird.caozc.top` 跑一次真实冒烟，不能只凭单测/集成测试断言完成；写操作（`watch create/update/delete`）冒烟要跑真实闭环并清理测试数据（建了就删）。
- `scripts/acceptance.sh`：按 skill `agent-friendly-cli` 的 `~/.claude/skills/agent-friendly-cli/references/verification.md` 逐条粘证据，对生产真实跑一遍。用法 `BLUEBIRD_TOKEN=... ./scripts/acceptance.sh`（或 `BLUEBIRD_JWT_SECRET`+`BLUEBIRD_USER_ID`），`BLUEBIRD_BIN` 可覆盖要测的二进制路径。脚本本身不含任何取密钥逻辑，也不硬编码内部基础设施地址——这点在改脚本时必须保持，公开仓库不能泄漏内网信息。

## 发布

- SemVer git tag `v*` 触发 `.goreleaser.yaml`：darwin/linux × amd64/arm64，GitHub Release，同时维护 `x0c/homebrew-tap` 的 `Casks/bluebird.rb`（`homebrew_casks`，goreleaser v2.16 起 `brews`/`Formula` 已移除）。`install.sh` 提供 curl\|bash 兜底安装。三条安装路径（brew / install.sh / go install）同步维护，改一条要回头检查其余描述是否还准确。
- Remote：GitHub `github.com/x0c/bluebird-cli`（origin，CI/Release/Homebrew tap 都在这）+ 内网 Forgejo 备份镜像 `Max/bluebird-cli`（只 push，不跑 CI；具体地址见个人基础设施知识库，**本仓库公开，不要写内网地址**）。

## OpenConductor 接入（P6，可选阶段）

本仓 `docs/` 目前为空；接入 OpenConductor 时再补 `docs/SKILL.md` 作为源文件，登记到 `~/.openconductor/tools/bluebird/SKILL.md`，frontmatter `name:` 必须等于可执行文件名 `bluebird`（历史教训见 `~/.config/agentsync/docs/AGENT_TOOL_CLI_INTEGRATION_GUIDE.md`）。接入后需要在 OpenConductor `server/cmd/conductor/main.go` 的 codex 沙箱 passthrough allowlist 里加 `BLUEBIRD_*` 四个变量，否则 codex 运行时管家调用会因环境变量被剥离而 `auth_not_configured`；claude 与 codex 两种管家运行时都要真机冒烟验证。

## 文档导航

暂无独立 `docs/` 知识库文档；随实现推进（P4 起）在此登记验收脚本、SKILL.md 维护要点等长期文档。
