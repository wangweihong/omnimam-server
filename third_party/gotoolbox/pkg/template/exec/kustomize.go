package exec

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/gotoolbox/pkg/executil"
	"github.com/wangweihong/gotoolbox/pkg/log"
	"github.com/wangweihong/gotoolbox/pkg/template"
)

type KustomizedTemplateDirParser struct {
	template.DirectoryProcessor
	KubeconfigPath string
	KubectlPath    string
	KustomizePath  string
}

// 如果有kustomization, 先用kustomize解析后再部署到k8s中
func (p *KustomizedTemplateDirParser) RunCreate() error {
	if p.Error() != nil {
		return errors.Errorf("templateDirProc %v meet error before run:%v ", p.TemplateName, p.Error())
	}
	kustomizePath := filepath.Join(p.LocateParsedDir, "kustomization.yaml")
	if _, err := os.Stat(kustomizePath); err != nil {
		if !os.IsNotExist(err) {
			return errors.Errorf("stat kustomizePath:%v fail:%v", kustomizePath, err)
		}
		args := []string{"--kubeconfig", p.KubeconfigPath, "apply", "-f", p.LocateParsedDir}
		if _, stderr, err := executil.ExecuteCmdSplitStdoutStderr(p.KubectlPath, args, 120); err != nil {
			log.Errorf("run command [%v:%v] fail:%v,stderr,%v", p.KubectlPath, args, errors.TrimError(err), stderr)
			return errors.TrimError(err)
		}

	} else {
		bashcmd := fmt.Sprintf("%v build %v | %v --kubeconfig %v create -f -", "kustomize", p.LocateParsedDir, p.KubectlPath, p.KubeconfigPath)
		args := []string{"-c", bashcmd}
		if _, stderr, err := executil.ExecuteCmdSplitStdoutStderr("/bin/bash", args, 0); err != nil {
			log.Errorf("run command [%v:%v] fail:%v,stderr:%v", "/bin/bash", args, errors.TrimError(err), stderr)
			return errors.TrimError(err)
		}
	}
	return nil
}
