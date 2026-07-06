package main

import "github.com/wangweihong/gotoolbox/pkg/log"

func main() {
	log.Debug("in default options, I don't print")
	// flush all buffer before reinit std logger instance
	log.Flush()

	// change log default logger level to debug
	opts := log.NewOptions()
	opts.Level = log.DebugLevel.String()
	// 初始化全局logger
	log.Init(opts)
	defer log.Flush()

	log.Debug("I will print after changed")
}
