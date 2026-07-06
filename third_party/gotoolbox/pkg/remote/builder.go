package remote

import (
	"io/ioutil"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/sftp"
	"github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/google/uuid"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

type PtyConfig struct {
	pty  string
	h, w int
	mode ssh.TerminalModes
}

type SSHBuilder struct {
	endpoint           string
	user               string
	authmethod         []ssh.AuthMethod
	knownHostsFilePath string
	ptyConfig          *PtyConfig
	knownHostsCallback ssh.HostKeyCallback

	timeout time.Duration
	handler func()
}

func NewSSHBuilder() *SSHBuilder {
	return &SSHBuilder{}
}

func (s *SSHBuilder) WithEndpoint(host string) *SSHBuilder {

	endpoint := host
	if !strings.HasSuffix(host, ":22") {
		endpoint = host + ":22"
	}
	s.endpoint = endpoint
	return s
}

func (s *SSHBuilder) WithUser(user string) *SSHBuilder {
	s.user = user
	return s
}

func (s *SSHBuilder) AddAuthFromPassword(password string) *SSHBuilder {
	s.authmethod = append(s.authmethod, ssh.Password(password))
	return s
}

func (s *SSHBuilder) AddAuthFromPrivateKeyFile(privateKeyPath string, privatekeyPassword string) *SSHBuilder {
	// read private key file
	pemBytes, err := ioutil.ReadFile(privateKeyPath)
	if err != nil {
		return s
	}
	// create signer
	signer, err := signerFromPem(pemBytes, []byte(privatekeyPassword))
	if err != nil {
		return s
	}
	s.authmethod = append(s.authmethod, ssh.PublicKeys(signer))
	return s
}

func (s *SSHBuilder) AddAuthFromPrivateKeyData(privateKeyData string, privatekeyPassword string) *SSHBuilder {
	pemBytes := []byte(privateKeyData)
	// create signer
	signer, err := signerFromPem(pemBytes, []byte(privatekeyPassword))
	if err != nil {
		return s
	}
	s.authmethod = append(s.authmethod, ssh.PublicKeys(signer))
	return s
}

func (s *SSHBuilder) AddHostKey(knownHostsFilePath string) *SSHBuilder {
	var knownHostsCallback ssh.HostKeyCallback
	if knownHostsFilePath != "" {
		cb, err := knownhosts.New(filepath.Join(knownHostsFilePath))
		if err != nil {
			return s
		}
		knownHostsCallback = cb
	} else {
		knownHostsCallback = ssh.InsecureIgnoreHostKey()
	}
	s.knownHostsCallback = knownHostsCallback
	return s
}

func (s *SSHBuilder) BuildSession() (*SSHSession, error) {
	if s.knownHostsCallback == nil {
		s.knownHostsCallback = ssh.InsecureIgnoreHostKey()
	}

	// SSH client configuration
	config := &ssh.ClientConfig{
		User:            s.user,
		Auth:            s.authmethod,
		HostKeyCallback: s.knownHostsCallback,
	}

	// Establish SSH connection
	conn, err := ssh.Dial("tcp", s.endpoint, config)
	if err != nil {
		return nil, err
	}

	// Create a new SSH session
	session, err := conn.NewSession()
	if err != nil {
		return nil, errors.Errorf("failed to create SSH session: %v", err)
	}

	// Setup session standard input/output
	stdinPipe, err := session.StdinPipe()
	if err != nil {
		return nil, errors.Errorf("unable to setup stdin for session: %v", err)
	}

	stdoutPipe, err := session.StdoutPipe()
	if err != nil {
		return nil, errors.Errorf("unable to setup stdout for session: %v", err)
	}

	stderrPipe, err := session.StderrPipe()
	if err != nil {
		return nil, errors.Errorf("unable to setup stderr for session: %v", err)
	}

	if s.ptyConfig == nil {
		// Request a pseudo terminal
		modes := ssh.TerminalModes{
			// 是否回显
			// 不显示输入的命令
			ssh.ECHO:          0,
			ssh.TTY_OP_ISPEED: 14400,
			ssh.TTY_OP_OSPEED: 14400,
		}

		if err := session.RequestPty("xterm", 80, 40, modes); err != nil {
			return nil, errors.Errorf("request for pseudo terminal failed: %v", err)
		}
	} else {
		if err := session.RequestPty(s.ptyConfig.pty, s.ptyConfig.h, s.ptyConfig.w, s.ptyConfig.mode); err != nil {
			return nil, errors.Errorf("request for pseudo terminal failed: %v", err)
		}
	}

	// Start the shell
	if err := session.Shell(); err != nil {
		return nil, errors.Errorf("failed to start shell: %v", err)
	}

	var timer *time.Timer
	if s.handler != nil {
		timer = time.AfterFunc(s.timeout, s.handler)
	}

	// Generate a unique ID for the session
	return &SSHSession{
		ID:           uuid.New().String(),
		Session:      session,
		StdinPipe:    stdinPipe,
		StdoutPipe:   stdoutPipe,
		StderrPipe:   stderrPipe,
		LastActivity: time.Now(),
		Timeout:      s.timeout,
		TimeoutTimer: timer,
	}, nil

}

func (s *SSHBuilder) BuildCommand() (*SSHCommand, error) {
	if s.knownHostsCallback == nil {
		s.knownHostsCallback = ssh.InsecureIgnoreHostKey()
	}

	// SSH client configuration
	config := &ssh.ClientConfig{
		User:            s.user,
		Auth:            s.authmethod,
		HostKeyCallback: s.knownHostsCallback,
	}

	// Establish SSH connection
	conn, err := ssh.Dial("tcp", s.endpoint, config)
	if err != nil {
		return nil, err
	}

	// Create a new SSH session
	session, err := conn.NewSession()
	if err != nil {
		return nil, errors.Errorf("failed to create SSH session: %v", err)
	}

	return &SSHCommand{
		Session: session,
	}, nil
}

func (s *SSHBuilder) BuildFile() (*SSHFile, error) {
	if s.knownHostsCallback == nil {
		s.knownHostsCallback = ssh.InsecureIgnoreHostKey()
	}

	// SSH client configuration
	config := &ssh.ClientConfig{
		User:            s.user,
		Auth:            s.authmethod,
		HostKeyCallback: s.knownHostsCallback,
	}

	// Establish SSH connection
	conn, err := ssh.Dial("tcp", s.endpoint, config)
	if err != nil {
		return nil, err
	}

	sftpClient, err := sftp.NewClient(conn)
	if err != nil {
		conn.Close()
		return nil, err
	}

	return &SSHFile{
		sshClient:  conn,
		sftpClient: sftpClient,
	}, nil
}
