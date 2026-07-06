//go:build !go1.19
// +build !go1.19

package exec

import (
	osexec "os/exec"
)

func maskErrDotCmd(cmd *osexec.Cmd) *osexec.Cmd {
	return cmd
}

func maskErrDot(err error) error {
	return err
}
