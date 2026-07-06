package exec_test

import (
	"context"
	"io"
	"io/ioutil"
	"os"
	osexec "os/exec"
	"testing"
	"time"

	"github.com/wangweihong/omnimam/third_party/k8s.io/utils/exec"
)

func TestExecutorNoArgs(t *testing.T) {
	ex := exec.New()

	cmd := ex.Command("true")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Errorf("expected success, got %v", err)
	}
	if len(out) != 0 {
		t.Errorf("expected no output, got %q", string(out))
	}

	cmd = ex.Command("false")
	out, err = cmd.CombinedOutput()
	if err == nil {
		t.Errorf("expected failure, got nil error")
	}
	if len(out) != 0 {
		t.Errorf("expected no output, got %q", string(out))
	}
	ee, ok := err.(exec.ExitError)
	if !ok {
		t.Errorf("expected an ExitError, got %+v", err)
	}
	if ee.Exited() {
		if code := ee.ExitStatus(); code != 1 {
			t.Errorf("expected exit status 1, got %d", code)
		}
	}

	cmd = ex.Command("/does/not/exist")
	_, err = cmd.CombinedOutput()
	if err == nil {
		t.Errorf("expected failure, got nil error")
	}
	if ee, ok := err.(exec.ExitError); ok {
		t.Errorf("expected non-ExitError, got %+v", ee)
	}
}

func TestExecutorWithArgs(t *testing.T) {
	ex := exec.New()

	cmd := ex.Command("echo", "stdout")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Errorf("expected success, got %+v", err)
	}
	if string(out) != "stdout\n" {
		t.Errorf("unexpected output: %q", string(out))
	}

	cmd = ex.Command("/bin/sh", "-c", "echo stderr > /dev/stderr")
	out, err = cmd.CombinedOutput()
	if err != nil {
		t.Errorf("expected success, got %+v", err)
	}
	if string(out) != "stderr\n" {
		t.Errorf("unexpected output: %q", string(out))
	}
}

func TestLookPath(t *testing.T) {
	ex := exec.New()

	shExpected, _ := osexec.LookPath("sh")
	sh, _ := ex.LookPath("sh")
	if sh != shExpected {
		t.Errorf("unexpected result for LookPath: got %s, expected %s", sh, shExpected)
	}
}

func TestExecutableNotFound(t *testing.T) {
	ex := exec.New()

	cmd := ex.Command("fake_executable_name")
	_, err := cmd.CombinedOutput()
	if err != exec.ErrExecutableNotFound {
		t.Errorf("cmd.CombinedOutput(): Expected error ErrExecutableNotFound but got %v", err)
	}

	cmd = ex.Command("fake_executable_name")
	_, err = cmd.Output()
	if err != exec.ErrExecutableNotFound {
		t.Errorf("cmd.Output(): Expected error ErrExecutableNotFound but got %v", err)
	}

	cmd = ex.Command("fake_executable_name")
	err = cmd.Run()
	if err != exec.ErrExecutableNotFound {
		t.Errorf("cmd.Run(): Expected error ErrExecutableNotFound but got %v", err)
	}
}

func TestStopBeforeStart(t *testing.T) {
	cmd := exec.New().Command("echo", "hello")

	// no panic calling Stop before calling Run
	cmd.Stop()

	cmd.Run()

	// no panic calling Stop after command is done
	cmd.Stop()
}

func TestTimeout(t *testing.T) {
	exec := exec.New()
	ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()

	err := exec.CommandContext(ctx, "sleep", "2").Run()
	if err != context.DeadlineExceeded {
		t.Errorf("expected %v but got %v", context.DeadlineExceeded, err)
	}
}

func TestSetEnv(t *testing.T) {
	ex := exec.New()

	out, err := ex.Command("/bin/sh", "-c", "echo $FOOBAR").CombinedOutput()
	if err != nil {
		t.Errorf("expected success, got %+v", err)
	}
	if string(out) != "\n" {
		t.Errorf("unexpected output: %q", string(out))
	}

	cmd := ex.Command("/bin/sh", "-c", "echo $FOOBAR")
	cmd.SetEnv([]string{"FOOBAR=baz"})
	out, err = cmd.CombinedOutput()
	if err != nil {
		t.Errorf("expected success, got %+v", err)
	}
	if string(out) != "baz\n" {
		t.Errorf("unexpected output: %q", string(out))
	}
}

func TestStdIOPipes(t *testing.T) {
	cmd := exec.New().Command("/bin/sh", "-c", "echo 'OUT'>&1; echo 'ERR'>&2")

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("expected StdoutPipe() not to error, got: %v", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		t.Fatalf("expected StderrPipe() not to error, got: %v", err)
	}

	stdout := make(chan string)
	stderr := make(chan string)

	go func() {
		stdout <- readAll(t, stdoutPipe, "StdOut")
	}()
	go func() {
		stderr <- readAll(t, stderrPipe, "StdErr")
	}()

	if err := cmd.Start(); err != nil {
		t.Errorf("expected Start() not to error, got: %v", err)
	}

	if e, a := "OUT\n", <-stdout; e != a {
		t.Errorf("expected StdOut to be '%s', got: '%v'", e, a)
	}

	if e, a := "ERR\n", <-stderr; e != a {
		t.Errorf("expected StdErr to be '%s', got: '%v'", e, a)
	}

	if err := cmd.Wait(); err != nil {
		t.Errorf("expected Wait() not to error, got: %v", err)
	}
}

func readAll(t *testing.T, r io.Reader, n string) string {
	t.Helper()

	b, err := ioutil.ReadAll(r)
	if err != nil {
		t.Fatalf("unexpected error when reading from %s: %v", n, err)
	}

	return string(b)
}

func TestExecutorGo119LookPath(t *testing.T) {
	orig := os.Getenv("PATH")
	defer func() { os.Setenv("PATH", orig) }()

	os.Setenv("PATH", "./testdata")

	ex := exec.New()
	path, err := ex.LookPath("hello")
	if err != nil {
		t.Errorf("expected success, got %v", err)
	}
	if path != "testdata/hello" {
		t.Errorf("expected relative path to hello script, got %v", path)
	}

	cmd := ex.Command("hello")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Errorf("expected success, got %v", err)
	} else {
		if len(out) != 5 {
			t.Errorf("expected 'hello' output, got %q", string(out))
		}
	}

	cmd = ex.CommandContext(context.Background(), "hello")
	out, err = cmd.CombinedOutput()
	if err != nil {
		t.Errorf("expected success, got %v", err)
	} else {
		if len(out) != 5 {
			t.Errorf("expected 'hello' output, got %q", string(out))
		}
	}
}
