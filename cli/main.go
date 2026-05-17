package main

import (
	"shadmin-cli/cmd"
)

// 构建时注入
var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	cmd.Execute(version, commit, date)
}
