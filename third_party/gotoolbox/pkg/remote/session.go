package remote

import (
	"bufio"
	"fmt"
	"io"
	"time"

	"golang.org/x/crypto/ssh"
)

// SSHSession holds the SSH session and related information
type SSHSession struct {
	ID           string
	Session      *ssh.Session
	StdinPipe    io.WriteCloser
	StdoutPipe   io.Reader
	StderrPipe   io.Reader
	LastActivity time.Time
	Timeout      time.Duration
	TimeoutTimer *time.Timer
}

// Exec executes a command on a specific SSH session and returns the output
func (s *SSHSession) Exec(command string) (string, error) {
	// Write command to SSH session
	_, err := fmt.Fprintln(s.StdinPipe, command)
	if err != nil {
		return "", err
	}

	// Reset the timeout timer
	if s.TimeoutTimer != nil {
		s.TimeoutTimer.Reset(s.Timeout)
	}
	s.LastActivity = time.Now()

	// Read command output
	reader := bufio.NewReader(s.StdoutPipe)
	line, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", fmt.Errorf("failed to read from stdout: %v", err)
	}

	return line, nil
}

func (s *SSHSession) Close() error {
	// Close the SSH session
	if err := s.Session.Close(); err != nil {
		return fmt.Errorf("failed to close SSH session: %v", err)
	}

	if s.TimeoutTimer != nil {
		// Stop the timeout timer
		s.TimeoutTimer.Stop()
	}
	return nil
}
