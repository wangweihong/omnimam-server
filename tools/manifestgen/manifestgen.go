package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
)

func writeString(err error, fp *os.File, data string) error {
	if err != nil {
		return err
	}
	_, err = fp.WriteString(data)
	if err != nil {
		return fmt.Errorf("error writing to output file: %v", err)
	}
	return err
}

func GenerateDataPath(packageName string, nameVar string, rootPath, outputPath string) error {

	fp, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("error creating output file: %v", err)
	}
	defer fp.Close()

	err = writeString(err, fp, fmt.Sprintf("package %s\n\n", packageName))
	//err = writeString(err, fp, "import (\n")
	//err = writeString(err, fp, "	\"api/ideploy\"\n")
	//err = writeString(err, fp, "	\"api/kubernetes/namespace\"\n")
	//err = writeString(err, fp, "	\"containercluster/manifests\"\n")
	//err = writeString(err, fp, "	\"github.com/renstrom/dedent\"\n")
	//err = writeString(err, fp, "	\"path/filepath\"\n")
	//err = writeString(err, fp, "	\"text/template\"\n")
	//err = writeString(err, fp, "	\"containercluster/manifests/generated\"\n")
	//err = writeString(err, fp, ")\n")
	err = writeString(err, fp, "\n")
	//	err = writeString(err, fp, "type GeneratedDataPath struct {\n\tData string\n\tPath string\n}\n\n")
	varsName, err := writeDataPaths(fp, rootPath, "")
	if err != nil {
		return err
	}

	err = writeString(err, fp, "type GeneratedDataPath struct {\n\tData string\n\tPath string\n}\n\n")
	err = writeString(err, fp, fmt.Sprintf("var %s = []GeneratedDataPath{\n", nameVar))

	//_, err = fmt.Fprintf(fp, "var %s = []generated.GeneratedDataPath{\n", nameVar)
	//if err != nil {
	//	return fmt.Errorf("error writing to output file: %v", err)
	//}
	for _, v := range varsName {
		err = writeString(err, fp, fmt.Sprintf("\t%v,\n", v))
	}
	err = writeString(err, fp, fmt.Sprintf("}\n"))
	//
	//_, err = fmt.Fprintf(fp, "}\n")
	//if err != nil {
	//	return fmt.Errorf("error writing to output file: %v", err)
	//}

	fmt.Println("Data paths generated successfully.")
	return nil
}

func writeDataPaths(fp *os.File, rootPath, prefix string) ([]string, error) {
	files, err := ioutil.ReadDir(rootPath)
	if err != nil {
		return nil, fmt.Errorf("error reading directory %s: %v", rootPath, err)
	}

	varsName := make([]string, 0, len(files))
	for _, file := range files {
		filePath := filepath.Join(rootPath, file.Name())

		if file.IsDir() {
			childVarsName, err := writeDataPaths(fp, filePath, filepath.Join(prefix, file.Name()))
			if err != nil {
				return nil, err
			}
			varsName = append(varsName, childVarsName...)
		} else {
			if !strings.HasSuffix(file.Name(), ".go") {
				fileContent, err := ioutil.ReadFile(filePath)
				if err != nil {
					return nil, fmt.Errorf("error reading file %s: %v", filePath, err)
				}

				dataPathVarName := strings.ReplaceAll(filepath.Join(prefix, file.Name()), "/", "_")
				dataPathVarName = strings.ReplaceAll(filepath.Join(prefix, file.Name()), "\\", "_")
				dataPathVarName = strings.ReplaceAll(dataPathVarName, ".", "_")
				dataPathVarName = strings.ReplaceAll(dataPathVarName, "-", "_")
				dataPathVarName = strings.ReplaceAll(dataPathVarName, "/", "_")
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
				//避免文件内容中也有``字符
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

func main() {
	packageName := flag.String("package", "", "The name of the package")
	nameVar := flag.String("name", "", "The variable name")
	rootPath := flag.String("root", "", "The root path")
	outputPath := flag.String("output", "", "The output path")

	flag.Parse()
	if *packageName == "" || *nameVar == "" || *rootPath == "" || *outputPath == "" {
		log.Fatal("All flags (package, name, root, output) are required.")
	}

	err := GenerateDataPath(*packageName, *nameVar, *rootPath, *outputPath)
	if err != nil {
		log.Fatalf("Error generating data path: %v", err)
	}
}
