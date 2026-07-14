package output

import (
	"fmt"
	"io"
)

// ExitError 把 Emit 计算出的进程退出码从 cobra RunE 传回 main，
// 因为 cobra.Command.Execute() 本身只返回 error，不保留自定义退出码。
type ExitError struct {
	Code int
}

func (e *ExitError) Error() string {
	return fmt.Sprintf("exit code %d", e.Code)
}

// AsExitError 构造 ExitError；code 为 0（成功）时返回 nil，方便直接 return。
func AsExitError(code int) error {
	if code == ExitOK {
		return nil
	}
	return &ExitError{Code: code}
}

// EmitParseError 处理 cobra 在 RunE 之前发生的参数解析失败（未知 flag、类型错误等），
// 因为此时 globalOptions 可能尚未成功绑定，只能按 stdout 是否为 TTY 判断输出模式。
func EmitParseError(stdout, stderr io.Writer, appVersion string, err error) int {
	jsonMode := DecideJSON(false)
	cmdErr := NewError(CodeUsageError, "参数解析失败："+err.Error())
	meta := Meta{Version: appVersion}
	if jsonMode {
		return writeEnvelope(stdout, Failure(cmdErr.Body(), meta), cmdErr.Code)
	}
	return writeHumanError(stderr, cmdErr)
}
