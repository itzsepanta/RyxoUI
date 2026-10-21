package nodes

import (
	"context"
	"fmt"
	"strings"

	"github.com/ryxoui/ryxoui/internal/ssh"
)

// LogOptions narrows the log of a node.
type LogOptions struct {
	// Lines is the number of lines to read. Zero means two hundred.
	Lines int
	// Since limits the log to the lines of the last minutes. Zero means no
	// limit.
	Since int
	// Service is the systemd unit the log belongs to. Empty means the unit
	// of the engine.
	Service string
}

// LogLine is one line of the log of a node.
type LogLine struct {
	Time    string `json:"time"`
	Level   string `json:"level"`
	Message string `json:"message"`
}

// Log reads the last lines of the log of a node.
func Log(ctx context.Context, node Node, privateKey string, opts LogOptions) ([]LogLine, error) {
	session, err := logSession(ctx, node, privateKey)
	if err != nil {
		return nil, err
	}
	defer session.Close()

	lines := opts.Lines
	if lines <= 0 {
		lines = 200
	}

	command := fmt.Sprintf("journalctl -u %s -n %d --no-pager -o short-iso",
		shellQuote(serviceOf(node, opts)), lines)
	if opts.Since > 0 {
		command = fmt.Sprintf("journalctl -u %s --since '-%d minutes' --no-pager -o short-iso",
			shellQuote(serviceOf(node, opts)), opts.Since)
	}

	stdout, stderr, code, err := session.Run(ctx, command)
	if err != nil {
		return nil, fmt.Errorf("nodes: read the log: %w", err)
	}
	if code != 0 {
		return nil, fmt.Errorf("nodes: read the log: %s", strings.TrimSpace(stderr))
	}
	return parseLog(stdout), nil
}

// TailLog follows the log of a node and hands every line to the callback.
//
// The tail ends when the context is cancelled, so the caller decides how
// long it follows.
func TailLog(ctx context.Context, node Node, privateKey string, opts LogOptions, handler func(LogLine)) error {
	session, err := logSession(ctx, node, privateKey)
	if err != nil {
		return err
	}
	defer session.Close()

	command := fmt.Sprintf("journalctl -u %s -f -n 0 --no-pager -o short-iso",
		shellQuote(serviceOf(node, opts)))

	return session.Stream(ctx, command, func(line string) {
		handler(parseLogLine(line))
	})
}

// logSession opens a session to the node of a log.
func logSession(ctx context.Context, node Node, privateKey string) (*ssh.Session, error) {
	if strings.TrimSpace(privateKey) == "" {
		return nil, ErrNoKey
	}

	client, err := ssh.NewClient(ssh.Config{
		Host:       node.SSHHostName(),
		Port:       node.SSHPort,
		User:       node.SSHUser,
		PrivateKey: privateKey,
		KnownHost:  node.HostKey,
	})
	if err != nil {
		return nil, err
	}
	return client.Connect(ctx)
}

// serviceOf is the systemd unit the log belongs to.
func serviceOf(node Node, opts LogOptions) string {
	if opts.Service != "" {
		return opts.Service
	}
	return "ryxoui-xray"
}

// parseLog reads the lines of a log.
func parseLog(raw string) []LogLine {
	lines := []LogLine{}
	for _, line := range strings.Split(raw, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		lines = append(lines, parseLogLine(line))
	}
	return lines
}

// parseLogLine reads one line of a log.
//
// The lines of the journal are written as
// "2026-10-21T10:11:12+0300 hostname unit[1234]: the message".
func parseLogLine(line string) LogLine {
	out := LogLine{Message: strings.TrimSpace(line)}

	rest := line
	if index := strings.Index(rest, " "); index >= 0 {
		out.Time = rest[:index]
		rest = rest[index+1:]
	}
	if index := strings.Index(rest, " "); index >= 0 {
		rest = rest[index+1:]
	}
	if index := strings.Index(rest, ": "); index >= 0 {
		out.Message = strings.TrimSpace(rest[index+2:])
		rest = rest[:index]
	}
	if index := strings.Index(rest, "["); index >= 0 {
		if end := strings.Index(rest, "]"); end > index {
			rest = rest[:index] + rest[end+1:]
		}
	}
	out.Level = levelOf(out.Message)
	return out
}

// levelOf guesses the level of a line of a log.
func levelOf(message string) string {
	upper := strings.ToUpper(message)
	switch {
	case strings.Contains(upper, "ERROR"), strings.Contains(upper, "FATAL"):
		return "error"
	case strings.Contains(upper, "WARN"):
		return "warning"
	case strings.Contains(upper, "DEBUG"):
		return "debug"
	case strings.Contains(upper, "INFO"):
		return "info"
	default:
		return ""
	}
}
