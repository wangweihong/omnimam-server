// main.go
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

// ./sortgen -input ../../apis/iapiserver/  -output ./example/generated_sortfields.go
var (
	inputDir = flag.String("input", ".", "输入目录路径")
	output   = flag.String("output", "", "输出文件名；默认 srcdir/sortfields_generated.go")
	marker   = flag.String("marker", "gen:sortfields", "用于标记需要生成代码的结构体的注释标记")
	maxDepth = flag.Int("maxdepth", 5, "最大递归深度")
)

func Usage() {
	fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "\tsort-gen [flags] -input [directory]\n")
	fmt.Fprintf(os.Stderr, "Flags:\n")
	flag.PrintDefaults()
}

func main() {
	flag.Usage = Usage
	flag.Parse()

	g := NewGenerator(*maxDepth, *marker)

	if err := g.parsePackage(*inputDir); err != nil {
		fmt.Fprintf(os.Stderr, "解析包失败: %v\n", err)
		os.Exit(2)
	}

	// 查找标记的结构体
	markedTypes := g.findMarkedTypes()
	if len(markedTypes) == 0 {
		fmt.Fprintf(os.Stderr, "未找到带有标记 %q 的结构体\n", *marker)
		os.Exit(2)
	}

	fmt.Printf("找到 %d 个需要生成代码的结构体: %v\n", len(markedTypes), markedTypes)

	if err := g.generate(markedTypes); err != nil {
		fmt.Fprintf(os.Stderr, "生成代码失败: %v\n", err)
		os.Exit(2)
	}

	outputName := *output
	if outputName == "" {
		outputName = filepath.Join(g.pkg.path, "sortfields_generated.go")
	}

	if err := g.writeOutput(outputName); err != nil {
		fmt.Fprintf(os.Stderr, "写入输出失败: %v\n", err)
		os.Exit(2)
	}

	fmt.Printf("成功生成代码文件: %s\n", outputName)
}
