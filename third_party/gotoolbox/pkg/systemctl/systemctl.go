package systemctl

import (
	"github.com/wangweihong/gotoolbox/pkg/errors"
	"github.com/wangweihong/gotoolbox/pkg/executil"
)

func NewCommand() Cmd {
	return Cmd{}
}

type Cmd struct {
}

func (s Cmd) Restart(svc string, reload bool) error {
	if reload {
		args := []string{"daemon-reload"}
		if _, err := executil.Execute("systemctl", args); err != nil {
			return errors.Errorf("run systemctl daemon-reload fail:%v", err)
		}
	}
	args := []string{"restart", svc}
	if _, err := executil.Execute("systemctl", args); err != nil {
		return errors.Errorf("run systemctl daemon-reload fail:%v", err)
	}
	return nil
}
