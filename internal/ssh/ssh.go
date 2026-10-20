// Package ssh drives the nodes over SSH.
//
// Every node gets its own key pair. The panel pins the host key of a node
// after the first successful connection, so a machine that changes its key
// is reported instead of being trusted silently.
package ssh

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

// Errors of the package.
var (
	// ErrHostKeyMismatch is returned when a node presents an unknown key.
	ErrHostKeyMismatch = errors.New("ssh: the host key of the node changed")
	// ErrNotConnected is returned when a command runs without a session.
	ErrNotConnected = errors.New("ssh: no session")
)

// KeyPair is an ed25519 key pair.
type KeyPair struct {
	PublicKey  string
	PrivateKey string
	Comment    string
}

// GenerateKeyPair creates a new ed25519 key pair. The public key is in the
// authorized_keys format and the private key in the OpenSSH format.
func GenerateKeyPair(comment string) (KeyPair, error) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return KeyPair{}, fmt.Errorf("ssh: generate key: %w", err)
	}

	sshPublic, err := ssh.NewPublicKey(public)
	if err != nil {
		return KeyPair{}, fmt.Errorf("ssh: public key: %w", err)
	}

	comment = strings.TrimSpace(comment)
	if comment != "" {
		comment = " " + comment
	}
	publicText := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(sshPublic))) + comment + "\n"

	privateText, err := marshalPrivateKey(private, comment)
	if err != nil {
		return KeyPair{}, err
	}
	return KeyPair{PublicKey: publicText, PrivateKey: privateText, Comment: comment}, nil
}

// marshalPrivateKey encodes an ed25519 key in the OpenSSH format and falls
// back to PKCS8 when the format is not available.
func marshalPrivateKey(key ed25519.PrivateKey, comment string) (string, error) {
	if block, err := ssh.MarshalPrivateKey(key, comment); err == nil {
		return string(pem.EncodeToMemory(block)), nil
	}
	raw, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return "", fmt.Errorf("ssh: marshal the private key: %w", err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: raw})), nil
}

// Config describes how to reach a node.
type Config struct {
	// Host is the address of the node.
	Host string
	// Port is the SSH port.
	Port int
	// User is the account the panel uses.
	User string
	// PrivateKey is the PEM encoded private key.
	PrivateKey string
	// Password authenticates the first connection. It is only used when no
	// private key is given and it is never stored.
	Password string
	// KnownHost is the pinned host key in the known_hosts format. It is
	// empty for the first connection.
	KnownHost string
	// Timeout bounds the handshake.
	Timeout time.Duration
}

// Client is a connection to a node.
type Client struct {
	config Config

	mu      sync.Mutex
	hostKey ssh.PublicKey
}

// NewClient returns a client for a node.
func NewClient(config Config) (*Client, error) {
	if config.Host == "" {
		return nil, errors.New("ssh: host must not be empty")
	}
	if config.Port == 0 {
		config.Port = 22
	}
	if config.User == "" {
		config.User = "root"
	}
	if config.Timeout == 0 {
		config.Timeout = 15 * time.Second
	}
	return &Client{config: config}, nil
}

// hostKeyCallback verifies the key of the node against the pinned one and
// remembers the key that was offered.
func (c *Client) hostKeyCallback() (ssh.HostKeyCallback, error) {
	if c.config.KnownHost == "" {
		// The first connection teaches the panel the key of the node.
		return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			c.mu.Lock()
			c.hostKey = key
			c.mu.Unlock()
			return nil
		}, nil
	}

	expected, err := parseKnownHost(c.config.KnownHost)
	if err != nil {
		return nil, err
	}
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		c.mu.Lock()
		c.hostKey = key
		c.mu.Unlock()
		if !bytes.Equal(key.Marshal(), expected.Marshal()) {
			return fmt.Errorf("%w: %s offered a different key", ErrHostKeyMismatch, hostname)
		}
		return nil
	}, nil
}

// Connect opens a session to the node.
func (c *Client) Connect(ctx context.Context) (*Session, error) {
	callback, err := c.hostKeyCallback()
	if err != nil {
		return nil, err
	}

	auth, err := c.authMethods()
	if err != nil {
		return nil, err
	}

	address := net.JoinHostPort(c.config.Host, fmt.Sprintf("%d", c.config.Port))
	dialer := &net.Dialer{Timeout: c.config.Timeout}
	connection, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return nil, fmt.Errorf("ssh: dial %s: %w", address, err)
	}

	clientConfig := &ssh.ClientConfig{
		User:            c.config.User,
		Auth:            auth,
		HostKeyCallback: callback,
		Timeout:         c.config.Timeout,
	}

	conn, channels, requests, err := ssh.NewClientConn(connection, address, clientConfig)
	if err != nil {
		connection.Close()
		return nil, fmt.Errorf("ssh: handshake with %s: %w", address, err)
	}
	return &Session{client: ssh.NewClient(conn, channels, requests), config: c.config}, nil
}

// authMethods builds the ways the panel may authenticate itself.
//
// The private key is used when there is one. The password is the way in for
// the first contact with a node, before the key of the panel is installed.
func (c *Client) authMethods() ([]ssh.AuthMethod, error) {
	var methods []ssh.AuthMethod

	if strings.TrimSpace(c.config.PrivateKey) != "" {
		signer, err := ssh.ParsePrivateKey([]byte(c.config.PrivateKey))
		if err != nil {
			return nil, fmt.Errorf("ssh: parse the private key: %w", err)
		}
		methods = append(methods, ssh.PublicKeys(signer))
	}

	if c.config.Password != "" {
		methods = append(methods, ssh.Password(c.config.Password))
	}

	if len(methods) == 0 {
		return nil, errors.New("ssh: neither a key nor a password was given")
	}
	return methods, nil
}

// KnownHost returns the host key of the node in the known_hosts format. It
// is meant to be stored after the first successful connection so the key can
// be pinned.
func (c *Client) KnownHost() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.hostKey == nil {
		return ""
	}
	return knownHostsLine(c.config.Host, c.config.Port, c.hostKey)
}

// parseKnownHost reads the pinned host key out of a known_hosts line.
func parseKnownHost(line string) (ssh.PublicKey, error) {
	_, _, key, _, _, err := ssh.ParseKnownHosts([]byte(strings.TrimSpace(line)))
	if err != nil {
		return nil, fmt.Errorf("ssh: parse the pinned host key: %w", err)
	}
	if key == nil {
		return nil, errors.New("ssh: the pinned host key is empty")
	}
	return key, nil
}

// knownHostsLine builds a known_hosts entry for a host key.
func knownHostsLine(host string, port int, key ssh.PublicKey) string {
	name := host
	if port != 22 {
		name = fmt.Sprintf("[%s]:%d", host, port)
	}
	return fmt.Sprintf("%s %s\n", name, strings.TrimSpace(string(ssh.MarshalAuthorizedKey(key))))
}

// Session is an open session to a node.
type Session struct {
	client *ssh.Client
	config Config
}

// Close ends the session.
func (s *Session) Close() error {
	if s == nil || s.client == nil {
		return nil
	}
	return s.client.Close()
}

// Run executes a command and returns its standard output, its standard error
// and the exit status.
func (s *Session) Run(ctx context.Context, command string) (string, string, int, error) {
	if s == nil || s.client == nil {
		return "", "", 0, ErrNotConnected
	}
	session, err := s.client.NewSession()
	if err != nil {
		return "", "", 0, fmt.Errorf("ssh: open session: %w", err)
	}
	defer session.Close()

	var stdout, stderr strings.Builder
	session.Stdout = &stdout
	session.Stderr = &stderr

	if err := session.Start(command); err != nil {
		return stdout.String(), stderr.String(), 0, fmt.Errorf("ssh: run %q: %w", command, err)
	}

	done := make(chan error, 1)
	go func() { done <- session.Wait() }()

	select {
	case <-ctx.Done():
		_ = session.Signal(ssh.SIGKILL)
		return stdout.String(), stderr.String(), 0, ctx.Err()
	case err := <-done:
		code := 0
		var exitErr *ssh.ExitError
		if errors.As(err, &exitErr) {
			code = exitErr.ExitStatus()
		} else if err != nil {
			return stdout.String(), stderr.String(), code, fmt.Errorf("ssh: wait: %w", err)
		}
		return stdout.String(), stderr.String(), code, nil
	}
}

// Upload copies a local file to the node.
func (s *Session) Upload(ctx context.Context, localPath, remotePath string) error {
	if s == nil || s.client == nil {
		return ErrNotConnected
	}
	handle, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("ssh: open %s: %w", localPath, err)
	}
	defer handle.Close()
	return s.upload(ctx, handle, remotePath, fmt.Sprintf("cat > %s && chmod 0755 %s", shellQuote(remotePath), shellQuote(remotePath)))
}

// UploadBytes writes bytes to a file on the node.
func (s *Session) UploadBytes(ctx context.Context, content []byte, remotePath string) error {
	if s == nil || s.client == nil {
		return ErrNotConnected
	}
	return s.upload(ctx, strings.NewReader(string(content)), remotePath, fmt.Sprintf("cat > %s", shellQuote(remotePath)))
}

func (s *Session) upload(ctx context.Context, source interface{ Read([]byte) (int, error) }, remotePath, command string) error {
	session, err := s.client.NewSession()
	if err != nil {
		return fmt.Errorf("ssh: open session: %w", err)
	}
	defer session.Close()

	stdin, err := session.StdinPipe()
	if err != nil {
		return fmt.Errorf("ssh: stdin pipe: %w", err)
	}
	if err := session.Start(command); err != nil {
		return fmt.Errorf("ssh: start upload: %w", err)
	}

	copyErr := make(chan error, 1)
	go func() {
		_, err := copyAll(stdin, source)
		stdin.Close()
		copyErr <- err
	}()

	waitErr := session.Wait()
	if err := <-copyErr; err != nil {
		return fmt.Errorf("ssh: upload %s: %w", remotePath, err)
	}
	if waitErr != nil {
		return fmt.Errorf("ssh: upload %s: %w", remotePath, waitErr)
	}
	return nil
}

// shellQuote makes a value safe for a POSIX shell.
func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}
