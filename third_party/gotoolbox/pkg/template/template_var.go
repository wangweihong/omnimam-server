package template

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/wangweihong/gotoolbox/pkg/stringutil"
)

//当前有目录树，该目录树下的文件非go文件，实现一个生成代码功能，遍历当前目录下所有文件，读取文件的内容，每个文件生成一个结构 GeneratedDataPath的变量
//type GeneratedDataPath struct {
//	Path string
//	Data string
//}
//如./a/b.yaml文件, 内容为bbb, 则在生成go文件代码如 var a_b_yaml=DataPath {
//	 PATH: "./a/b.yaml",
//	 Data: `
//bbb
//`,
//}

var ignoreFileExtensions = []string{".go"}

func GenerateDataPathToFile(packageName string, nameVar string, rootPath, outputPath string) error {
	buf, err := GenerateDataPath(packageName, nameVar, rootPath)
	if err != nil {
		return err
	}
	if err := ioutil.WriteFile(outputPath, buf.Bytes(), 0o644); err != nil {
		return err
	}

	return nil
}

func GenerateDataPath(packageName string, nameVar string, rootPath string) (*bytes.Buffer, error) {
	fp := bytes.NewBuffer([]byte{})
	_, err := fp.WriteString(fmt.Sprintf("package %s\n\n", packageName))
	if err != nil {
		return nil, fmt.Errorf("error writing to output file: %v", err)
	}

	_, err = fp.WriteString("type GeneratedDataPath struct {\n\tData string\n\tPath string\n}\n\n")
	if err != nil {
		return nil, fmt.Errorf("error writing to output file: %v", err)
	}

	varsName, err := WalkRecordFileDataPath(fp, rootPath, "")
	if err != nil {
		return nil, err
	}

	_, err = fmt.Fprintf(fp, "var %s = []GeneratedDataPath{\n", nameVar)
	if err != nil {
		return nil, fmt.Errorf("error writing to output file: %v", err)
	}
	for _, v := range varsName {
		if _, err = fmt.Fprintf(fp, "\t%v,\n", v); err != nil {
			return nil, err
		}
	}
	_, err = fmt.Fprintf(fp, "}\n")
	if err != nil {
		return nil, fmt.Errorf("error writing to output file: %v", err)
	}

	return fp, nil
}

func WalkRecordFileDataPath(fp *bytes.Buffer, rootPath, prefix string) ([]string, error) {
	files, err := ioutil.ReadDir(rootPath)
	if err != nil {
		return nil, fmt.Errorf("error reading directory %s: %v", rootPath, err)
	}

	varsName := make([]string, 0, len(files))
	for _, file := range files {
		filePath := filepath.Join(rootPath, file.Name())

		if file.IsDir() {
			childVarsName, err := WalkRecordFileDataPath(fp, filePath, filepath.Join(prefix, file.Name()))
			if err != nil {
				return nil, err
			}
			varsName = append(varsName, childVarsName...)
		} else {
			if !stringutil.HasAnySuffix(file.Name(), ignoreFileExtensions...) {
				fileContent, err := ioutil.ReadFile(filePath)
				if err != nil {
					return nil, fmt.Errorf("error reading file %s: %v", filePath, err)
				}

				dataPathVarName := strings.ReplaceAll(filepath.Join(prefix, file.Name()), "/", "_")
				dataPathVarName = strings.ReplaceAll(filepath.Join(prefix, file.Name()), "\\", "_")
				dataPathVarName = strings.ReplaceAll(dataPathVarName, ".", "_")
				dataPathVarName = strings.ReplaceAll(dataPathVarName, "-", "_")
				varsName = append(varsName, dataPathVarName)
				_, err = fmt.Fprintf(fp, "var %s = GeneratedDataPath{\n", dataPathVarName)
				if err != nil {
					return nil, fmt.Errorf("error writing to output file: %v", err)
				}

				filePath = strings.ReplaceAll(filepath.Join(prefix, file.Name()), `\`, "/")
				_, err = fmt.Fprintf(fp, "\tPath: \"%s\",\n", filePath)
				if err != nil {
					return nil, fmt.Errorf("error writing to output file: %v", err)
				}
				// 避免文件内容中也有``字符
				_, err = fmt.Fprintf(fp, "\tData: `%s`,\n", escapeBackticks(string(fileContent)))
				if err != nil {
					return nil, fmt.Errorf("error writing to output file: %v", err)
				}
				_, err = fmt.Fprintln(fp, "}")
				if err != nil {
					return nil, fmt.Errorf("error writing to output file: %v", err)
				}
				_, err = fmt.Fprintln(fp, "")
				if err != nil {
					return nil, fmt.Errorf("error writing to output file: %v", err)
				}
			}
		}
	}

	return varsName, nil
}

func escapeBackticks(s string) string {
	return strings.ReplaceAll(s, "`", "`+\"`\"+`")
}
