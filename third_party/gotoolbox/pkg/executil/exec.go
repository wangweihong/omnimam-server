package executil

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const (
	ExecuteTime = 60
)

func Execute(binary string, args []string) (string, error) {
	return ExecuteTimeout(binary, args, ExecuteTime)
}

func ExecuteTimeout(binary string, args []string, timeout time.Duration) (string, error) {
	for _, arg := range args {
		if strings.Contains(arg, "`") {
			return "", fmt.Errorf("timeout executing: %v,error: %s contain special symbols", binary, arg)
		}
	}
	return ExecuteTimeoutEnv(binary, args, timeout, nil)
}

func CheckIfCmdlineArgvIsValid(args []string) bool {
	for _, arg := range args {
		if strings.Contains(arg, "`") {
			return false
		}
	}
	return true
}

func ExecuteTimeoutEnv(binary string, args []string, timeout time.Duration, env []string) (string, error) {
	var output []byte
	var err error
	cmd := exec.Command(binary, args...)
	cmd.Env = env

	done := make(chan struct{}, 1)

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
			return string(output), fmt.Errorf("failed to execute: %v %v, output %v, error %v", binary, args, string(output), err)
		} else {
			fmt.Printf("execute: %v %v, output %v, error %v", binary, args, string(output), err)
		}
	}
	return string(output), nil
}

func ExecuteCmdSplitStdoutStderr(binary string, arg []string, timeout time.Duration) (string, string, error) {
	if timeout == 0 {
		timeout = time.Minute
	}
	var output []byte
	var err error
	var stderr bytes.Buffer
	cmd := exec.Command(binary, arg...)
	cmd.Stderr = &stderr
	done := make(chan struct{})

	go func() {
		output, err = cmd.Output()
		done <- struct{}{}
	}()

	select {
	case <-done:
	case <-time.After(timeout):
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
		return "", "", fmt.Errorf("timeout executing: %v, arg %v,error %v", binary, arg, err)
	}

	if err != nil {
		if !strings.Contains(err.Error(), "no child processes") {
			return string(output), stderr.String(), fmt.Errorf("failed to execute: %v, arg %v, error %v", binary, arg, err)
		}
	}

	return string(output), stderr.String(), nil
}
