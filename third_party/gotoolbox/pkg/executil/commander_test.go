package executil_test

import (
	"strings"
	"testing"

	"github.com/wangweihong/gotoolbox/pkg/executil"
)

func TestCommander(t *testing.T) {
	commander := executil.NewCommander()

	cleanTask := strings.Split("ctr -n k8s.io tasks ls -q | xargs -r -I{} ctr -n k8s.io task kill --signal SIGKILL {} || true", " ")
	cleanContainer := strings.Split("ctr -n k8s.io containers ls -q | xargs -r ctr -n k8s.io containers delete", " ")
	_, commander.Err = commander.Execute(cleanTask[0], cleanTask[1:]...)
	_, commander.Err = commander.Execute(cleanContainer[0], cleanContainer[1:]...)
	if commander.Err != nil {
		t.Fatal(commander.Err)
	}

}
