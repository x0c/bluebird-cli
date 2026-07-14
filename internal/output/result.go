package output

import (
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// Result 是命令执行的中间产物，渲染前的通用载体；人类与机器两条渲染路径都消费它。
type Result struct {
	Data          any
	Err           *CmdError
	DryRun        bool
	ResolvedPlace any
}

// RenderOptions 控制一次渲染的行为，来自全局 flag。
type RenderOptions struct {
	JSONMode      bool
	Fields        []string
	Compact       bool
	CompactFields []string
	Quiet         bool
}

// HumanRenderFunc 只在人类模式下被调用，负责把 data 渲染成可读文本。
type HumanRenderFunc func(w io.Writer, data any, quiet bool) error

// Emit 是渲染的唯一入口：决定信封/文本两条物理分离的路径，返回进程退出码。
// 失败也会向 stdout 写出合法 JSON 信封（JSON 模式下），不会只在 stderr 打一行文本。
func Emit(stdout, stderr io.Writer, version string, start time.Time, opts RenderOptions, res Result, human HumanRenderFunc) int {
	meta := Meta{
		DurationMS:    time.Since(start).Milliseconds(),
		DryRun:        res.DryRun,
		ResolvedPlace: res.ResolvedPlace,
		Version:       version,
	}

	if res.Err != nil {
		if opts.JSONMode {
			return writeEnvelope(stdout, Failure(res.Err.Body(), meta), res.Err.Code)
		}
		return writeHumanError(stderr, res.Err)
	}

	data := res.Data
	if opts.Compact && len(opts.CompactFields) > 0 {
		if projected, err := ProjectFields(data, opts.CompactFields); err == nil {
			data = projected
		}
	}
	if len(opts.Fields) > 0 {
		if projected, err := ProjectFields(data, opts.Fields); err == nil {
			data = projected
		}
	}

	if opts.JSONMode {
		return writeEnvelope(stdout, Success(data, meta), "")
	}

	if human != nil {
		if err := human(stdout, data, opts.Quiet); err != nil {
			_, _ = fmt.Fprintf(stderr, "错误：渲染输出失败：%v\n", err)
			return ExitGeneral
		}
	}
	return ExitOK
}

func writeEnvelope(w io.Writer, env Envelope, errCode string) int {
	enc := json.NewEncoder(w)
	if err := enc.Encode(env); err != nil {
		// 序列化本身失败极罕见；仍需向 stdout 输出合法信封而不是静默退出。
		fallback := Failure(&ErrorBody{Code: CodeServerError, Message: "内部错误：结果序列化失败", Retryable: false}, env.Meta)
		_ = json.NewEncoder(w).Encode(fallback)
		return ExitGeneral
	}
	if errCode == "" {
		return ExitOK
	}
	return ExitCodeFor(errCode)
}

func writeHumanError(w io.Writer, err *CmdError) int {
	_, _ = fmt.Fprintf(w, "错误：%s\n", err.Message)
	if err.Hint != "" {
		_, _ = fmt.Fprintf(w, "提示：%s\n", err.Hint)
	}
	for _, cmd := range err.NextCommands {
		_, _ = fmt.Fprintf(w, "可尝试：%s\n", cmd)
	}
	return ExitCodeFor(err.Code)
}
