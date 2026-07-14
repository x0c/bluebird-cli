// Package output 定义 bluebird CLI 面向机器与面向人的统一输出契约。
package output

// Envelope 是所有 --json 输出的统一信封，成功与失败共用同一结构。
type Envelope struct {
	OK    bool       `json:"ok"`
	Data  any        `json:"data"`
	Error *ErrorBody `json:"error"`
	Meta  Meta       `json:"meta"`
}

// ErrorBody 描述一次失败：code 供程序判断，message 面向人（中文）。
type ErrorBody struct {
	Code         string   `json:"code"`
	Message      string   `json:"message"`
	Hint         string   `json:"hint,omitempty"`
	NextCommands []string `json:"next_commands,omitempty"`
	Retryable    bool     `json:"retryable"`
	Details      any      `json:"details,omitempty"`
}

// Meta 携带非主体信息：耗时、dry-run 标记、地名解析结果、CLI 版本等。
type Meta struct {
	DurationMS    int64  `json:"duration_ms"`
	DryRun        bool   `json:"dry_run,omitempty"`
	ResolvedPlace any    `json:"resolved_place,omitempty"`
	Version       string `json:"version"`
}

// Success 构造一个成功信封。
func Success(data any, meta Meta) Envelope {
	return Envelope{OK: true, Data: data, Error: nil, Meta: meta}
}

// Failure 构造一个失败信封；data 恒为 nil。
func Failure(err *ErrorBody, meta Meta) Envelope {
	return Envelope{OK: false, Data: nil, Error: err, Meta: meta}
}
