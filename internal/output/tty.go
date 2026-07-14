package output

import (
	"os"

	"golang.org/x/term"
)

// StdoutIsTTY 报告 stdout 是否连接终端；stdout 被管道接走时为 false。
func StdoutIsTTY() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}

// StdinIsTTY 报告 stdin 是否连接终端。stdout 被管道接走但 stdin 仍是终端的情况真实存在，
// 因此两者必须分开检测，不能用同一个判断代替。
func StdinIsTTY() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}

// ColorEnabled 报告是否应该输出颜色：非 TTY、NO_COLOR、TERM=dumb 或显式 --no-color 都关闭。
func ColorEnabled(noColorFlag bool) bool {
	if noColorFlag {
		return false
	}
	if _, ok := os.LookupEnv("NO_COLOR"); ok {
		return false
	}
	if os.Getenv("TERM") == "dumb" {
		return false
	}
	return StdoutIsTTY()
}

// DecideJSON 决定是否使用 JSON envelope 模式：显式 --json，或 stdout 非 TTY 时自动开启，
// 调用方无需主动传参就能被 Agent 正确降级消费。
func DecideJSON(explicit bool) bool {
	if explicit {
		return true
	}
	return !StdoutIsTTY()
}
