//go:build go1.19
// +build go1.19

package exec

import (
	"errors"
	osexec "os/exec"
)

// maskErrDotCmd reverts the behavior of osexec.Cmd to what it was before go1.19
// specifically set the Err field to nil (LookPath returns a new error when the file
// is resolved to the current directory.
func maskErrDotCmd(cmd *osexec.Cmd) *osexec.Cmd {
	cmd.Err = maskErrDot(cmd.Err)
	return cmd
}

func maskErrDot(err error) error {
	if err != nil && errors.Is(err, osexec.ErrDot) {
		return nil
	}
	return err
}
