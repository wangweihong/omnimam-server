package exec

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/gotoolbox/pkg/executil"

	"github.com/wangweihong/gotoolbox/pkg/log"
	"github.com/wangweihong/gotoolbox/pkg/template"
)

var (
	fileSuffixToApply = []string{".yaml", ".yml"}
)

type KustomizedTemplateFileParser struct {
	template.FileProcessor
	KubeconfigPath string
	KubectlPath    string
	KustomizePath  string
}

func (p *KustomizedTemplateFileParser) Run() error {
	if p.Error() != nil {
		return errors.Errorf("templateDirProc %v meet error before run:%v ", p.TemplateName, p.Error())
	}

	// ignore non k8s resource file
	if !isFileSuffixToApply(p.FilePath) {
		return nil
	}
	kustomizePath := filepath.Join(filepath.Dir(p.FilePath), "kustomization.yaml")
	if _, err := os.Stat(kustomizePath); err != nil {
		if !os.IsNotExist(err) {
			return errors.Errorf("stat kustomizePath:%v fail:%v", kustomizePath, err)
		}
		args := []string{"--kubeconfig", p.KubeconfigPath, "apply", "-f", p.FilePath}
		if _, _, err := executil.ExecuteCmdSplitStdoutStderr(p.KubectlPath, args, 0); err != nil {
			log.Errorf("run command [%v:%v] fail:%v", p.KubectlPath, args, errors.TrimError(err))
			return errors.TrimError(err)
		}
	} else {
		bashcmd := fmt.Sprintf("%v build %v | %v --kubeconfig %v apply -f -", p.KustomizePath, p.FilePath, p.KubectlPath, p.KubeconfigPath)
		args := []string{"-c", bashcmd}
		if _, _, err := executil.ExecuteCmdSplitStdoutStderr("/bin/bash", args, 0); err != nil {
			log.Errorf("run command [%v:%v] fail:%v", "/bin/bash", args, errors.TrimError(err))
			return errors.TrimError(err)
		}
	}

	return nil
}

func isFileSuffixToApply(fileName string) bool {
	for _, v := range fileSuffixToApply {
		if strings.HasSuffix(fileName, v) {
			return true
		}
	}
	return false
}
