package main

import "github.com/wangweihong/gotoolbox/pkg/log"

func main() {
	opt1 := log.NewOptions()
	opt1.Format = "console"
	consoleLog := log.New(opt1)
	defer consoleLog.Flush()

	// 2023-06-15 16:58:50.669 INFO    format/format.go:11     i am console log        {"key": "value"}
	consoleLog.Infow("i am console log", "key", "value")

	opt2 := log.NewOptions()
	opt2.Format = "json"
	jsonLog := log.New(opt2)
	defer jsonLog.Flush()

	// {"level":"INFO","timestamp":"2023-06-15 16:58:50.688","caller":"format/format.go:18","message":"i am json
	// log","key":"value"}
	jsonLog.Infow("i am json log", "key", "value")
}
