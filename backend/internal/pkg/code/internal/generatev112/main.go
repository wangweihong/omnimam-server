package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"gopkg.in/yaml.v3"
)

type errorDefinition struct {
	Code  string `yaml:"code"`
	Value int    `yaml:"value"`
	HTTP  int    `yaml:"http"`
	CN    string `yaml:"cn"`
	EN    string `yaml:"en"`
}

func main() {
	root := flag.String("root", "", "repository root")
	flag.Parse()
	if *root == "" {
		fatalf("-root is required")
	}
	absRoot, err := filepath.Abs(*root)
	if err != nil {
		fatalf("resolve root: %v", err)
	}
	var definitions []errorDefinition
	for _, domain := range []string{"agent", "appstudio", "infrastructure"} {
		path := filepath.Join(absRoot, "ssot/01_contracts/domains", domain, "errors.yaml")
		raw, err := os.ReadFile(path)
		if err != nil {
			fatalf("read %s: %v", path, err)
		}
		var items []errorDefinition
		if err := yaml.Unmarshal(raw, &items); err != nil {
			fatalf("decode %s: %v", path, err)
		}
		definitions = append(definitions, items...)
	}

	var source bytes.Buffer
	source.WriteString("// Code generated from released spec-v1.12.0 error catalogs; DO NOT EDIT.\n\npackage code\n\nconst (\n")
	for _, definition := range definitions {
		fmt.Fprintf(&source, "\t// @HTTP %d\n\t// @CN %s\n\t// @EN %s\n\t%s int = %d\n\n",
			definition.HTTP, definition.CN, definition.EN, goName(definition.Code), definition.Value)
	}
	source.WriteString(")\n")
	formatted, err := format.Source(source.Bytes())
	if err != nil {
		fatalf("format generated source: %v\n%s", err, source.String())
	}
	destination := filepath.Join(absRoot, "backend/internal/pkg/code/release_v112.go")
	if err := os.WriteFile(destination, formatted, 0o644); err != nil {
		fatalf("write %s: %v", destination, err)
	}
}

func goName(code string) string {
	var result strings.Builder
	result.WriteString("Err")
	for _, part := range strings.Split(strings.TrimPrefix(code, "ERR_"), "_") {
		if part == "APPSTUDIO" {
			result.WriteString("AppStudio")
			continue
		}
		for index, r := range strings.ToLower(part) {
			if index == 0 {
				r = unicode.ToUpper(r)
			}
			result.WriteRune(r)
		}
	}
	return result.String()
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
