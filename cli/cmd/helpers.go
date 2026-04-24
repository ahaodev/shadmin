package cmd

import "time"

// sec 返回 1 秒的 time.Duration，供命令里拼 timeout。
// 独立函数方便后续统一调整或 mock。
func sec() time.Duration { return time.Second }
