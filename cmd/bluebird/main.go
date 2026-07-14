// Command bluebird 是 Bluebird 天气发现服务的命令行客户端，面向人与大模型 Agent 双受众。
package main

import (
	"errors"
	"os"

	"github.com/x0c/bluebird-cli/internal/cmdtree"
	"github.com/x0c/bluebird-cli/internal/output"
	"github.com/x0c/bluebird-cli/internal/version"
)

func main() {
	app := &cmdtree.App{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Stdin:  os.Stdin,
		Getenv: os.Getenv,
	}

	root := app.Build()
	err := root.Execute()
	if err == nil {
		os.Exit(output.ExitOK)
	}

	var exitErr *output.ExitError
	if errors.As(err, &exitErr) {
		os.Exit(exitErr.Code)
	}

	// cobra 自身在 RunE 之前发生的参数解析错误：仍需向 stdout/stderr 输出合法信封或中文文本。
	os.Exit(output.EmitParseError(os.Stdout, os.Stderr, version.Version, err))
}
