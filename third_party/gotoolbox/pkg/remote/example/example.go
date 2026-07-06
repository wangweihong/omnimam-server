package main

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/google/uuid"
)

/*
1. 一个服务用于实现远程shell
2. /ssh生成一个新的ssh会话
3、/exec接收指令，ssh到远程环境执行命令
4. /close 关闭指定的ssh会话
5. 5分钟没有输入新的指令，则关闭删除会话
*/
// SSHSession holds the SSH session and related information
type SSHSession struct {
	Session      *ssh.Session
	StdinPipe    io.WriteCloser
	StdoutPipe   io.Reader
	LastActivity time.Time
	TimeoutTimer *time.Timer
}

var (
	mu       sync.Mutex
	sessions = make(map[string]*SSHSession)
)

// handleSSH creates a new SSH session and returns its ID
func handleSSH(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	defer mu.Unlock()

	server := "example.com:22"
	username := "your_username"
	privateKeyPath := "/path/to/your/private/key"

	// Read private key
	key, err := ioutil.ReadFile(privateKeyPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Unable to read private key: %v", err), http.StatusInternalServerError)
		return
	}

	// Parse private key
	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		http.Error(w, fmt.Sprintf("Unable to parse private key: %v", err), http.StatusInternalServerError)
		return
	}

	// SSH client configuration
	config := &ssh.ClientConfig{
		User: username,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // Should use knownhosts.New in production
	}

	// Establish SSH connection
	conn, err := ssh.Dial("tcp", server, config)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to connect to SSH server: %v", err), http.StatusInternalServerError)
		return
	}

	// Create a new SSH session
	session, err := conn.NewSession()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create SSH session: %v", err), http.StatusInternalServerError)
		return
	}

	// Setup session standard input/output
	stdinPipe, err := session.StdinPipe()
	if err != nil {
		http.Error(w, fmt.Sprintf("Unable to setup stdin for session: %v", err), http.StatusInternalServerError)
		return
	}
	stdoutPipe, err := session.StdoutPipe()
	if err != nil {
		http.Error(w, fmt.Sprintf("Unable to setup stdout for session: %v", err), http.StatusInternalServerError)
		return
	}

	// Request a pseudo terminal
	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	if err := session.RequestPty("xterm", 80, 40, modes); err != nil {
		http.Error(w, fmt.Sprintf("Request for pseudo terminal failed: %v", err), http.StatusInternalServerError)
		return
	}

	// Start the shell
	if err := session.Shell(); err != nil {
		http.Error(w, fmt.Sprintf("Failed to start shell: %v", err), http.StatusInternalServerError)
		return
	}

	// Generate a unique ID for the session
	sessionID := uuid.New().String()
	sessions[sessionID] = &SSHSession{
		Session:      session,
		StdinPipe:    stdinPipe,
		StdoutPipe:   stdoutPipe,
		LastActivity: time.Now(),
		TimeoutTimer: time.AfterFunc(5*time.Minute, func() {
			handleSessionTimeout(sessionID)
		}),
	}

	// Return the session ID
	fmt.Fprintln(w, sessionID)
}

// handleExec executes a command on a specific SSH session and returns the output
func handleExec(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	defer mu.Unlock()

	// Get the session ID and command from the request
	sessionID := r.URL.Query().Get("id")
	command := r.URL.Query().Get("cmd")

	if sessionID == "" || command == "" {
		http.Error(w, "Missing session ID or command", http.StatusBadRequest)
		return
	}

	// Get the SSH session
	session, ok := sessions[sessionID]
	if !ok {
		http.Error(w, "Invalid session ID", http.StatusNotFound)
		return
	}

	// Write command to SSH session
	_, err := fmt.Fprintln(session.StdinPipe, command)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to write to stdin: %v", err), http.StatusInternalServerError)
		return
	}

	// Reset the timeout timer
	session.TimeoutTimer.Reset(5 * time.Minute)
	session.LastActivity = time.Now()

	// Read command output
	reader := bufio.NewReader(session.StdoutPipe)
	line, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		http.Error(w, fmt.Sprintf("Failed to read from stdout: %v", err), http.StatusInternalServerError)
		return
	}

	// Write output to response
	fmt.Fprintln(w, line)
}

// handleClose closes a specific SSH session
func handleClose(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	defer mu.Unlock()

	// Get the session ID from the request
	sessionID := r.URL.Query().Get("id")
	if sessionID == "" {
		http.Error(w, "Missing session ID", http.StatusBadRequest)
		return
	}

	// Get the SSH session
	session, ok := sessions[sessionID]
	if !ok {
		http.Error(w, "Invalid session ID", http.StatusNotFound)
		return
	}

	// Close the SSH session
	if err := session.Session.Close(); err != nil {
		http.Error(w, fmt.Sprintf("Failed to close SSH session: %v", err), http.StatusInternalServerError)
		return
	}

	// Stop the timeout timer
	session.TimeoutTimer.Stop()

	// Remove the session from the map
	delete(sessions, sessionID)

	fmt.Fprintln(w, "SSH session closed")
}

// handleSessionTimeout handles the session timeout
func handleSessionTimeout(sessionID string) {
	mu.Lock()
	defer mu.Unlock()

	session, ok := sessions[sessionID]
	if !ok {
		return
	}

	// Close the SSH session
	if err := session.Session.Close(); err != nil {
		log.Printf("Failed to close SSH session %v: %v", sessionID, err)
	}

	// Remove the session from the map
	delete(sessions, sessionID)

	log.Printf("SSH session %v timed out and closed", sessionID)
}

func main() {
	http.HandleFunc("/ssh", handleSSH)
	http.HandleFunc("/exec", handleExec)
	http.HandleFunc("/close", handleClose)

	// Handle termination signals to gracefully shut down
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		log.Printf("Received signal: %v, shutting down...", sig)
		mu.Lock()
		for id, session := range sessions {
			if err := session.Session.Close(); err != nil {
				log.Printf("Failed to close SSH session %v: %v", id, err)
			}
		}
		mu.Unlock()
		os.Exit(0)
	}()

	log.Println("Starting HTTP server on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Failed to start HTTP server: %v", err)
	}
}
