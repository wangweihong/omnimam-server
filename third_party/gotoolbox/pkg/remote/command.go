package remote

import (
	"fmt"
	"strings"

	"github.com/wangweihong/gotoolbox/pkg/errors"

	"golang.org/x/crypto/ssh"
)

// SSHCommand execute command on remote ssh
type SSHCommand struct {
	Session *ssh.Session
}

// Exec executes a command on a specific SSH session and returns the output
func (s *SSHCommand) Output(command string) (string, error) {
	output, err := s.Session.Output(command)
	if err != nil {
		return "", errors.WithStack(err)
	}

	return string(output), nil
}

func (s *SSHCommand) Close() error {
	if err := s.Session.Close(); err != nil {
		return errors.Wrapf(err, "failed to close SSH session")
	}
	return nil
}

// Tree list  trees
func (s *SSHCommand) Tree(condition, dir string) ([]string, error) {
	command := fmt.Sprintf("find %s", dir)
	// condition: find %s \( -name 'updatepackage*.tar.gz' -o -name '*.sign' -o -path '*/usr/bin/*' -o -path '*/usr/sbin/*' \) -not -path '*/tpmdev/*'
	if condition != "" {
		command += " " + condition
	}
	output, err := s.Session.Output(command)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	lines := strings.Split(string(output), "\n")
	fps := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			fps = append(fps, line)
		}
	}
	return fps, nil
}
