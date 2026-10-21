package nodes

import (
	"context"
	"strings"
	"testing"
	"time"

	panelssh "github.com/ryxoui/ryxoui/internal/ssh"
	"github.com/ryxoui/ryxoui/internal/ssh/sshtest"
)

func TestTheLogOfANodeIsRead(t *testing.T) {
	output := strings.Join([]string{
		"2026-10-21T10:11:12+0300 eu-1 ryxoui-xray[1234]: the engine started",
		"2026-10-21T10:11:13+0300 eu-1 ryxoui-xray[1234]: INFO the configuration was applied",
		"2026-10-21T10:11:14+0300 eu-1 ryxoui-xray[1234]: WARN the memory is almost full",
		"2026-10-21T10:11:15+0300 eu-1 ryxoui-xray[1234]: ERROR the port is taken",
		"",
	}, "\n")

	server, err := sshtest.New(output)
	if err != nil {
		t.Fatalf("start the node: %v", err)
	}
	defer server.Close()

	pair, err := panelssh.GenerateKeyPair("test")
	if err != nil {
		t.Fatalf("generate the client key: %v", err)
	}

	node := Node{SSHHost: server.Host, SSHPort: server.Port, SSHUser: "root"}

	lines, err := Log(context.Background(), node, pair.PrivateKey, LogOptions{Lines: 100})
	if err != nil {
		t.Fatalf("read the log: %v", err)
	}
	if len(lines) != 4 {
		t.Fatalf("lines = %d, want 4", len(lines))
	}

	first := lines[0]
	if first.Time != "2026-10-21T10:11:12+0300" {
		t.Fatalf("time = %q", first.Time)
	}
	if first.Message != "the engine started" {
		t.Fatalf("message = %q", first.Message)
	}
	if first.Level != "" {
		t.Fatalf("level = %q", first.Level)
	}
	if lines[1].Level != "info" {
		t.Fatalf("level = %q", lines[1].Level)
	}
	if lines[2].Level != "warning" {
		t.Fatalf("level = %q", lines[2].Level)
	}
	if lines[3].Level != "error" {
		t.Fatalf("level = %q", lines[3].Level)
	}
}

func TestTheLogNeedsAKey(t *testing.T) {
	_, err := Log(context.Background(), Node{SSHHost: "127.0.0.1"}, "", LogOptions{})
	if err != ErrNoKey {
		t.Fatalf("error = %v, want ErrNoKey", err)
	}
}

func TestTheLogIsFollowed(t *testing.T) {
	output := "2026-10-21T10:11:12+0300 eu-1 ryxoui-xray[1]: the engine started\n"

	server, err := sshtest.New(output)
	if err != nil {
		t.Fatalf("start the node: %v", err)
	}
	defer server.Close()

	pair, err := panelssh.GenerateKeyPair("test")
	if err != nil {
		t.Fatalf("generate the client key: %v", err)
	}

	node := Node{SSHHost: server.Host, SSHPort: server.Port, SSHUser: "root"}

	seen := []LogLine{}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = TailLog(ctx, node, pair.PrivateKey, LogOptions{}, func(line LogLine) {
		seen = append(seen, line)
	})
	if err != nil {
		t.Fatalf("follow the log: %v", err)
	}
	if len(seen) != 1 {
		t.Fatalf("lines = %d, want 1", len(seen))
	}
	if seen[0].Message != "the engine started" {
		t.Fatalf("message = %q", seen[0].Message)
	}
}
