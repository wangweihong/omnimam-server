package main

import "github.com/wangweihong/gotoolbox/pkg/log"

func main() {
	// case 1: no output
	outputToDiscard()
	// case 2: output to multiple file
	outputToFile()
	// case 3: output to stdout
	outputToStd()
}

func outputToDiscard() {
	opts := log.NewOptions()
	opts.OutputPaths = nil
	opts.ErrorOutputPaths = nil
	// 初始化全局logger
	log.Init(opts)
	defer log.Flush()

	log.Info("you can't see me")
}

func outputToFile() {
	opts := log.NewOptions()
	opts.OutputPaths = []string{"./my.log", "./my2.log"}
	// 初始化全局logger
	log.Init(opts)
	defer log.Flush()

	log.Info("i will be my.log and my2.log")
	log.Error("i will be my.log and my2.log")
}

func outputToStd() {
	opts := log.NewOptions()
	opts.OutputPaths = []string{"stdout"}
	// 初始化全局logger
	log.Init(opts)
	defer log.Flush()

	log.Info("i will be stdout")
}
