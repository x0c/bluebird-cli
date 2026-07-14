// Package version 保存由 goreleaser ldflags 注入的构建信息。
package version

// Version/Commit/Date 由发布流水线通过 -ldflags 注入；本地开发构建保持 dev/none/unknown。
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)
