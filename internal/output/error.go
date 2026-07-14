package output

// CmdError 是命令内部统一使用的错误类型，携带渲染 ErrorBody 所需的全部信息。
type CmdError struct {
	Code         string
	Message      string
	Hint         string
	NextCommands []string
	Retryable    bool
	Details      any
}

func (e *CmdError) Error() string {
	return e.Message
}

// Body 把 CmdError 转换为对外的 ErrorBody；退出码由 error.code 单一映射得出。
func (e *CmdError) Body() *ErrorBody {
	return &ErrorBody{
		Code:         e.Code,
		Message:      e.Message,
		Hint:         e.Hint,
		NextCommands: e.NextCommands,
		Retryable:    e.Retryable,
		Details:      e.Details,
	}
}

// NewError 是构造 CmdError 的便捷方法。
func NewError(code, message string) *CmdError {
	return &CmdError{Code: code, Message: message}
}

// WithHint 链式设置 hint。
func (e *CmdError) WithHint(hint string) *CmdError {
	e.Hint = hint
	return e
}

// WithNextCommands 链式设置 next_commands。
func (e *CmdError) WithNextCommands(cmds ...string) *CmdError {
	e.NextCommands = cmds
	return e
}

// WithDetails 链式设置 details。
func (e *CmdError) WithDetails(details any) *CmdError {
	e.Details = details
	return e
}

// WithRetryable 链式设置 retryable。
func (e *CmdError) WithRetryable(retryable bool) *CmdError {
	e.Retryable = retryable
	return e
}
