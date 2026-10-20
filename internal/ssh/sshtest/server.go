// Package sshtest serves a small SSH server that stands in for a node in
// the tests.
//
// The server accepts every key and every password and answers every command
// with the same text, so a test can drive the whole path from the panel to a
// node without a real machine.
package sshtest

import (
	"crypto/ed25519"
	"crypto/rand"
	"io"
	"net"
	"sync"

	"golang.org/x/crypto/ssh"
)

// Server is a fake node.
type Server struct {
	// Host is the address of the server.
	Host string
	// Port is the port of the server.
	Port int
	// Output is the answer of every command.
	Output string
	// Commands holds the commands the server received, in order.
	Commands []string

	listener net.Listener
	mu       sync.Mutex
}

// New starts a fake node that answers every command with the given text.
func New(output string) (*Server, error) {
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	signer, err := ssh.NewSignerFromKey(private)
	if err != nil {
		return nil, err
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}

	server := &Server{
		Host:     "127.0.0.1",
		Port:     listener.Addr().(*net.TCPAddr).Port,
		Output:   output,
		listener: listener,
	}

	go server.accept(signer)
	return server, nil
}

// Close stops the server.
func (s *Server) Close() {
	if s.listener != nil {
		s.listener.Close()
	}
}

// CommandCount returns the number of commands the server received.
func (s *Server) CommandCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.Commands)
}

// accept waits for the connections of the clients.
func (s *Server) accept(signer ssh.Signer) {
	for {
		connection, err := s.listener.Accept()
		if err != nil {
			return
		}
		go s.serve(connection, signer)
	}
}

// serve answers one connection.
func (s *Server) serve(connection net.Conn, signer ssh.Signer) {
	config := &ssh.ServerConfig{
		PublicKeyCallback: func(ssh.ConnMetadata, ssh.PublicKey) (*ssh.Permissions, error) {
			return nil, nil
		},
		PasswordCallback: func(ssh.ConnMetadata, []byte) (*ssh.Permissions, error) {
			return nil, nil
		},
	}
	config.AddHostKey(signer)

	serverConn, channels, requests, err := ssh.NewServerConn(connection, config)
	if err != nil {
		connection.Close()
		return
	}
	defer serverConn.Close()
	go ssh.DiscardRequests(requests)

	for newChannel := range channels {
		if newChannel.ChannelType() != "session" {
			newChannel.Reject(ssh.UnknownChannelType, "only sessions are served")
			continue
		}
		channel, channelRequests, err := newChannel.Accept()
		if err != nil {
			return
		}
		go s.serveSession(channel, channelRequests)
	}
}

// serveSession answers one session.
func (s *Server) serveSession(channel ssh.Channel, requests <-chan *ssh.Request) {
	defer channel.Close()

	for request := range requests {
		if request.Type != "exec" {
			request.Reply(false, nil)
			continue
		}
		request.Reply(true, nil)

		var payload struct{ Command string }
		_ = ssh.Unmarshal(request.Payload, &payload)

		s.mu.Lock()
		s.Commands = append(s.Commands, payload.Command)
		s.mu.Unlock()

		// A command may carry content, for example a file that is uploaded.
		// The server reads it before it answers.
		_, _ = io.Copy(io.Discard, channel)

		_, _ = channel.Write([]byte(s.Output))
		_, _ = channel.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{Status: 0}))
		return
	}
}
