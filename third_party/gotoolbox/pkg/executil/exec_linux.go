package executil

import (
	"fmt"
	"os/exec"
	"os/user"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func ExecuteByUser(binary string, args []string, executeUser string) (string, error) {
	return ExecuteTimeoutByUser(binary, args, 600, executeUser)
}

func ExecuteTimeoutByUser(binary string, args []string, timeout time.Duration, targetUser string) (string, error) {
	for _, arg := range args {
		if strings.Contains(arg, "`") {
			return "", fmt.Errorf("timeout executing: %v,error: %s contain special symbols", binary, arg)
		}
	}
	return ExecuteTimeoutEnvByUser(binary, args, timeout, nil, targetUser)
}

func ExecuteTimeoutEnvByUser(binary string, args []string, timeout time.Duration, env []string, executeUser string) (string, error) {
	var output []byte
	var err error
	cmd := exec.Command(binary, args...)
	if executeUser != "" {
		targetUser, err := user.Lookup(executeUser)
		if err != nil {
			return "", err
		}
		uid, _ := strconv.Atoi(targetUser.Uid)
		gid, _ := strconv.Atoi(targetUser.Gid)
		cmd.SysProcAttr = &syscall.SysProcAttr{}
		cmd.SysProcAttr.Credential = &syscall.Credential{Uid: uint32(uid), Gid: uint32(gid)}
	}
	cmd.Env = env
	done := make(chan struct{})

	go func() {
		output, err = cmd.CombinedOutput()
		done <- struct{}{}
	}()

	select {
	case <-done:
	case <-time.After(timeout):
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
		return "", fmt.Errorf("timeout executing: %v %v, output %v, error %v", binary, args, string(output), err)
	}

	if err != nil {
		if !strings.Contains(err.Error(), "no child processes") {
			return "", fmt.Errorf("failed to execute: %v %v, output %v, error %v", binary, args, string(output), err)
		} else {
			fmt.Printf("execute: %v %v, output %v, error %v", binary, args, string(output), err)
		}
	}
	return string(output), nil
}
