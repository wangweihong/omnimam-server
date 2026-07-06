package remote

import (
	"io"
	"os"

	"github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

type SSHFile struct {
	sshClient  *ssh.Client
	sftpClient *sftp.Client
}

// Upload upload file to remote server
func (s *SSHFile) Upload(remote, local string) error {
	localFile, err := os.Open(local)
	if err != nil {
		return errors.WithStack(err)
	}
	defer localFile.Close()

	remoteFile, err := s.sftpClient.Create(remote)
	if err != nil {
		return errors.Errorf("failed to create remote file: %v", err)
	}
	defer remoteFile.Close()

	_, err = io.Copy(remoteFile, localFile)
	if err != nil {
		return errors.Errorf("failed to copy file: %v", err)
	}
	return nil
}

// Download download file from remote server
func (s *SSHFile) Download(remote, local string) error {
	remoteFile, err := s.sftpClient.Open(remote)
	if err != nil {
		return errors.Errorf("failed to create remote file: %v", err)
	}
	defer remoteFile.Close()

	localFile, err := os.Create(local)
	if err != nil {
		return errors.WithStack(err)
	}
	defer localFile.Close()

	_, err = io.Copy(localFile, remoteFile)
	if err != nil {
		return errors.Errorf("failed to copy file: %v", err)
	}
	return nil
}

// ListDirectory list directory files from remote server
func (s *SSHFile) ListDirectory(remoteDir string) ([]os.FileInfo, error) {
	files, err := s.sftpClient.ReadDir(remoteDir)
	if err != nil {
		return nil, err
	}
	return files, nil
}

func (s *SSHFile) ReadFile(remoteFilePath string) (string, error) {
	remoteFile, err := s.sftpClient.Open(remoteFilePath)
	if err != nil {
		return "", errors.Errorf("failed to open remote file: %w", err)
	}
	defer remoteFile.Close()

	content := make([]byte, 1024)
	n, err := remoteFile.Read(content)
	if err != nil && err.Error() != "EOF" {
		return "", errors.Errorf("failed to read remote file: %w", err)
	}

	return string(content[:n]), nil
}

func (s *SSHFile) Close() error {
	_ = s.sftpClient.Close()
	if err := s.sshClient.Close(); err != nil {
		return errors.Errorf("failed to close SSH session: %v", err)
	}
	return nil
}
