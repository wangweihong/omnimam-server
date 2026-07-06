// generator.go
package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
)

type Generator struct {
	buf      bytes.Buffer
	pkg      *Package
	maxDepth int
	marker   string
}

type Package struct {
	name  string
	files []*File
	path  string
	types map[string]*TypeInfo // 类型名 -> 类型信息
}

type File struct {
	pkg  *Package
	file *ast.File
}

type TypeInfo struct {
	StructType *ast.StructType
	Comments   []*ast.Comment // 相关的注释
	File       *ast.File      // 所属文件
}

func NewGenerator(maxDepth int, marker string) *Generator {
	return &Generator{
		maxDepth: maxDepth,
		marker:   marker,
	}
}

func (g *Generator) parsePackage(dir string) error {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, nil, parser.ParseComments)
	if err != nil {
		return err
	}

	// if len(pkgs) != 1 {
	// 	return fmt.Errorf("找到 %d 个包，期望 1 个", len(pkgs))
	// }

	var astPkg *ast.Package
	for _, p := range pkgs {
		if !strings.HasSuffix(p.Name, "_test") {
			astPkg = p
			break
		}
	}

	return g.buildPackage(astPkg, dir)
}

func (g *Generator) buildPackage(astPkg *ast.Package, path string) error {
	pkg := &Package{
		name:  astPkg.Name,
		path:  path,
		types: make(map[string]*TypeInfo),
	}

	// 收集所有类型定义
	for filename, astFile := range astPkg.Files {
		file := &File{
			file: astFile,
			pkg:  pkg,
		}
		pkg.files = append(pkg.files, file)
		g.collectTypes(astFile, pkg, filename)
	}

	g.pkg = pkg
	return nil
}

// func (g *Generator) collectTypes(file *ast.File, pkg *Package, filename string) {
// 	// 构建注释组映射
// 	commentMap := ast.NewCommentMap(token.NewFileSet(), file, file.Comments)
// 	fmt.Println("commentMap Len", len(commentMap))

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
// 				// 获取与此声明相关的注释
// 				var comments []*ast.Comment
// 				if commentGroups, exists := commentMap[decl]; exists {
// 					for _, cg := range commentGroups {
// 						for _, c := range cg.List {
// 							comments = append(comments, c)
// 						}
// 					}
// 				}

// 				pkg.types[typeSpec.Name.Name] = &TypeInfo{
// 					StructType: structType,
// 					Comments:   comments,
// 					File:       file,
// 				}
// 			}
// 		}
// 	}
// }

func (g *Generator) collectTypes(file *ast.File, pkg *Package, filename string) {
	// 使用更可靠的方法构建注释映射
	// 直接遍历注释组，找到它们关联的节点
	commentMap := make(map[ast.Node][]*ast.CommentGroup)

	// 构建节点到注释组的映射
	for _, commentGroup := range file.Comments {
		// 找到这个注释组关联的节点
		node := g.findAssociatedNode(file, commentGroup)
		if node != nil {
			commentMap[node] = append(commentMap[node], commentGroup)
		}
	}

	for _, decl := range file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.TYPE {
			continue
		}

		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}

			structType, ok := typeSpec.Type.(*ast.StructType)
			if ok {
				// 获取与此类型声明相关的注释
				var comments []*ast.Comment

				// 方法1: 从 GenDecl 的 Doc 获取
				if genDecl.Doc != nil {
					for _, comment := range genDecl.Doc.List {
						comments = append(comments, comment)
					}
				}

				// 方法2: 从注释映射获取
				if commentGroups, exists := commentMap[genDecl]; exists {
					for _, cg := range commentGroups {
						for _, c := range cg.List {
							comments = append(comments, c)
						}
					}
				}

				// 方法3: 从 TypeSpec 的 Doc 获取
				if typeSpec.Doc != nil {
					for _, comment := range typeSpec.Doc.List {
						comments = append(comments, comment)
					}
				}

				pkg.types[typeSpec.Name.Name] = &TypeInfo{
					StructType: structType,
					Comments:   comments,
					File:       file,
				}

				// 调试输出
				fmt.Printf("类型 %s 找到 %d 条注释\n", typeSpec.Name.Name, len(comments))
				for i, comment := range comments {
					fmt.Printf("  注释 %d: %s\n", i+1, comment.Text)
				}
			}
		}
	}
}

// findAssociatedNode 找到注释组关联的 AST 节点
func (g *Generator) findAssociatedNode(file *ast.File, cg *ast.CommentGroup) ast.Node {
	cgPos := cg.Pos()

	// 遍历所有声明
	for _, decl := range file.Decls {
		declPos := decl.Pos()
		if declPos > cgPos {
			// 如果声明在注释之后，则前一个声明可能是关联的
			continue
		}

		// 检查注释是否紧接在声明之前
		// 这里使用简单的位置比较，实际可能需要更精确的算法
		if decl.End() < cgPos && (cgPos-decl.End()) < 1000 { // 1000 是一个经验值
			return decl
		}
	}

	return nil
}

// 或者使用更简单但更可靠的方法：直接解析文件内容
func (g *Generator) collectTypesWithContent(file *ast.File, pkg *Package, filename string) {
	// 读取文件内容
	content, err := os.ReadFile(filename)
	if err != nil {
		fmt.Printf("无法读取文件 %s: %v\n", filename, err)
		return
	}

	for _, decl := range file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.TYPE {
			continue
		}

		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}

			structType, ok := typeSpec.Type.(*ast.StructType)
			if ok {
				// 获取类型声明前的注释
				comments := g.extractCommentsBefore(content, genDecl.Pos())

				pkg.types[typeSpec.Name.Name] = &TypeInfo{
					StructType: structType,
					Comments:   comments,
					File:       file,
				}

				// 调试输出
				fmt.Printf("类型 %s 找到 %d 条注释\n", typeSpec.Name.Name, len(comments))
				for i, comment := range comments {
					fmt.Printf("  注释 %d: %s\n", i+1, comment.Text)
				}
			}
		}
	}
}

// extractCommentsBefore 从文件内容中提取指定位置前的注释
func (g *Generator) extractCommentsBefore(content []byte, pos token.Pos) []*ast.Comment {
	var comments []*ast.Comment

	// 将 token.Pos 转换为字节偏移量
	offset := int(pos) - 1
	if offset <= 0 || offset > len(content) {
		return comments
	}

	// 向前查找注释
	// 这里简化处理，实际可能需要完整的词法分析
	lines := strings.Split(string(content[:offset]), "\n")

	// 从后向前遍历行，找到连续的注释行
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "//") {
			// 单行注释
			comment := &ast.Comment{
				Text: line,
			}
			comments = append([]*ast.Comment{comment}, comments...)
		} else if strings.HasPrefix(line, "/*") && strings.HasSuffix(line, "*/") {
			// 块注释
			comment := &ast.Comment{
				Text: line,
			}
			comments = append([]*ast.Comment{comment}, comments...)
		} else {
			// 遇到非注释行，停止
			break
		}
	}

	return comments
}
