// // robust_generator.go
package main

// import (
// 	"bufio"
// 	"fmt"
// 	"go/ast"
// 	"go/parser"
// 	"go/token"
// 	"os"
// 	"path/filepath"
// 	"strings"
// )

// // RobustGenerator 使用更可靠的方法处理注释
// type RobustGenerator struct {
// 	Generator
// }

// // parsePackageWithRobustComments 使用更可靠的注释解析
// func (g *RobustGenerator) parsePackageWithRobustComments(dir string) error {
// 	fset := token.NewFileSet()
// 	pkgs, err := parser.ParseDir(fset, dir, nil, parser.ParseComments)
// 	if err != nil {
// 		return err
// 	}

// 	if len(pkgs) != 1 {
// 		return fmt.Errorf("找到 %d 个包，期望 1 个", len(pkgs))
// 	}

// 	var astPkg *ast.Package
// 	for _, p := range pkgs {
// 		astPkg = p
// 		break
// 	}

// 	return g.buildPackageWithRobustComments(astPkg, dir)
// }

// func (g *RobustGenerator) buildPackageWithRobustComments(astPkg *ast.Package, path string) error {
// 	pkg := &Package{
// 		name:  astPkg.Name,
// 		path:  path,
// 		types: make(map[string]*TypeInfo),
// 	}

// 	// 对每个文件分别处理
// 	for filename, astFile := range astPkg.Files {
// 		file := &File{
// 			file: astFile,
// 			pkg:  pkg,
// 		}
// 		pkg.files = append(pkg.files, file)

// 		// 使用可靠的方法收集类型和注释
// 		g.collectTypesReliably(astFile, pkg, filename)
// 	}

// 	g.pkg = pkg
// 	return nil
// }

// func (g *RobustGenerator) collectTypesReliably(file *ast.File, pkg *Package, filename string) {
// 	// 首先读取原始文件内容
// 	content, err := os.ReadFile(filename)
// 	if err != nil {
// 		fmt.Printf("警告: 无法读取文件 %s: %v\n", filename, err)
// 		return
// 	}

// 	// 按行解析文件
// 	lines, typePositions := g.parseFileLines(content, file)

// 	for _, decl := range file.Decls {
// 		genDecl, ok := decl.(*ast.GenDecl)
// 		if !ok || genDecl.Tok != token.TYPE {
// 			continue
// 		}

// 		for _, spec := range genDecl.Specs {
// 			typeSpec, ok := spec.(*ast.TypeSpec)
// 			if !ok {
// 				continue
// 			}

// 			structType, ok := typeSpec.Type.(*ast.StructType)
// 			if ok {
// 				typeName := typeSpec.Name.Name

// 				// 获取该类型声明前的注释
// 				comments := g.findCommentsBeforeType(lines, typePositions[typeName])

// 				pkg.types[typeName] = &TypeInfo{
// 					StructType: structType,
// 					Comments:   comments,
// 					File:       file,
// 				}

// 				fmt.Printf("类型 %s 找到 %d 条注释:\n", typeName, len(comments))
// 				for i, comment := range comments {
// 					fmt.Printf("  [%d] %s\n", i+1, comment.Text)
// 				}
// 			}
// 		}
// 	}
// }

// // parseFileLines 解析文件行并记录类型位置
// func (g *RobustGenerator) parseFileLines(content []byte, file *ast.File) ([]string, map[string]int) {
// 	lines := strings.Split(string(content), "\n")
// 	typePositions := make(map[string]int)

// 	// 记录每个类型声明的行号
// 	for _, decl := range file.Decls {
// 		genDecl, ok := decl.(*ast.GenDecl)
// 		if !ok || genDecl.Tok != token.TYPE {
// 			continue
// 		}

// 		for _, spec := range genDecl.Specs {
// 			typeSpec, ok := spec.(*ast.TypeSpec)
// 			if ok {
// 				// 获取类型声明的行号
// 				line := file.Fset.Position(typeSpec.Pos()).Line
// 				typePositions[typeSpec.Name.Name] = line
// 			}
// 		}
// 	}

// 	return lines, typePositions
// }

// // findCommentsBeforeType 查找类型声明前的注释
// func (g *RobustGenerator) findCommentsBeforeType(lines []string, typeLine int) []*ast.Comment {
// 	var comments []*ast.Comment

// 	if typeLine <= 1 || typeLine > len(lines) {
// 		return comments
// 	}

// 	// 从类型声明行向上查找连续的注释
// 	for i := typeLine - 2; i >= 0; i-- {
// 		line := strings.TrimSpace(lines[i])

// 		if line == "" {
// 			// 空行，继续向上查找
// 			continue
// 		}

// 		if strings.HasPrefix(line, "//") {
// 			// 单行注释
// 			comment := &ast.Comment{
// 				Text: line,
// 			}
// 			comments = append([]*ast.Comment{comment}, comments...)
// 		} else if strings.HasPrefix(line, "/*") && strings.HasSuffix(line, "*/") {
// 			// 块注释
// 			comment := &ast.Comment{
// 				Text: line,
// 			}
// 			comments = append([]*ast.Comment{comment}, comments...)
// 		} else {
// 			// 遇到非注释行，停止查找
// 			break
// 		}
// 	}

// 	return comments
// }
