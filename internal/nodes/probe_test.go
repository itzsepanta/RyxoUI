package nodes

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	panelssh "github.com/ryxoui/ryxoui/internal/ssh"
	"github.com/ryxoui/ryxoui/internal/ssh/sshtest"
)

func TestProbeReadsTheNode(t *testing.T) {
	output := strings.Join([]string{
		"os=linux",
		"arch=x86_64",
		"kernel=6.1.0",
		"mem_total=16777216000",
		"mem_available=8388608000",
		"disk_total=214748364800",
		"disk_used=10737418240",
		"load1=0.42",
		"xray_version=Xray 1.8.4",
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

	node := Node{
		ID:           "node-1",
		Name:         "eu-1",
		SSHHost:      server.Host,
		SSHPort:      server.Port,
		SSHUser:      "root",
		SSHKeyID:     "key-1",
		KeyInstalled: true,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := Probe(ctx, node, pair.PrivateKey)
	if err != nil {
		t.Fatalf("probe: %v", err)
	}

	if result.OS != "linux" {
		t.Fatalf("os = %q", result.OS)
	}
	if result.Arch != "x86_64" {
		t.Fatalf("arch = %q", result.Arch)
	}
	if result.CoreVersion != "Xray 1.8.4" {
		t.Fatalf("core version = %q", result.CoreVersion)
	}
	if result.MemTotal != 16777216000 {
		t.Fatalf("mem total = %d", result.MemTotal)
	}
	if result.MemUsed != 16777216000-8388608000 {
		t.Fatalf("mem used = %d", result.MemUsed)
	}
	if result.DiskTotal != 214748364800 || result.DiskUsed != 10737418240 {
		t.Fatalf("disk = %d/%d", result.DiskUsed, result.DiskTotal)
	}
	if result.Load1 != 0.42 {
		t.Fatalf("load = %v", result.Load1)
	}
	if result.HostKey == "" {
		t.Fatal("the probe must bring the host key of the node")
	}

	sample := result.HealthSample(node.ID, 100)
	if sample.NodeID != "node-1" || sample.MemUsed != result.MemUsed {
		t.Fatalf("sample = %+v", sample)
	}
}

func TestProbeNeedsAKey(t *testing.T) {
	_, err := Probe(context.Background(), Node{SSHHost: "127.0.0.1"}, "")
	if err != ErrNoKey {
		t.Fatalf("error = %v, want ErrNoKey", err)
	}
}

func TestProbePinsTheHostKey(t *testing.T) {
	server, err := sshtest.New("os=linux\n")
	if err != nil {
		t.Fatalf("start the node: %v", err)
	}
	defer server.Close()
	pair, err := panelssh.GenerateKeyPair("test")
	if err != nil {
		t.Fatalf("generate the client key: %v", err)
	}

	node := Node{SSHHost: server.Host, SSHPort: server.Port, SSHUser: "root"}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	first, err := Probe(ctx, node, pair.PrivateKey)
	if err != nil {
		t.Fatalf("first probe: %v", err)
	}
	if first.HostKey == "" {
		t.Fatal("the first probe must learn the host key")
	}

	// The pinned key is accepted, a different one is refused.
	node.HostKey = first.HostKey
	if _, err := Probe(ctx, node, pair.PrivateKey); err != nil {
		t.Fatalf("probe with the pinned key: %v", err)
	}

	node.HostKey = strings.Replace(first.HostKey, "ssh-ed25519", "ssh-rsa", 1)
	if _, err := Probe(ctx, node, pair.PrivateKey); err == nil {
		t.Fatal("a changed host key must be refused")
	}
}

func TestParseProbeOutput(t *testing.T) {
	result := parseProbeOutput("os=linux\narch=arm64\nload1=1.5\nbroken line\n")
	if result.OS != "linux" || result.Arch != "arm64" || result.Load1 != 1.5 {
		t.Fatalf("result = %+v", result)
	}
}

func TestTheCollectorRecordsTheAnswer(t *testing.T) {
	store := testStore(t)
	keys := NewKeyStore(store.db)

	pair, err := panelssh.GenerateKeyPair("test")
	if err != nil {
		t.Fatalf("generate the client key: %v", err)
	}
	key, err := keys.Create(context.Background(), SSHKey{Name: "k", PublicKey: pair.PublicKey, PrivateKey: pair.PrivateKey})
	if err != nil {
		t.Fatalf("create the key: %v", err)
	}

	node, err := store.Create(context.Background(), Node{
		Name:         "eu-1",
		Address:      "10.0.0.5",
		SSHHost:      "10.0.0.5",
		SSHUser:      "root",
		SSHKeyID:     key.ID,
		KeyInstalled: true,
		Enabled:      true,
		Status:       StatusConnecting,
	})
	if err != nil {
		t.Fatalf("create the node: %v", err)
	}

	collector := NewCollector(store, keys, func(ctx context.Context, node Node, privateKey string) (ProbeResult, error) {
		return ProbeResult{OS: "linux", Arch: "x86_64", MemTotal: 100, MemUsed: 40, HostKey: "pinned"}, nil
	}, CollectorOptions{Logger: nil})

	collector.Round(context.Background())

	updated, err := store.Get(context.Background(), node.ID)
	if err != nil {
		t.Fatalf("read the node: %v", err)
	}
	if updated.Status != StatusConnected {
		t.Fatalf("status = %q", updated.Status)
	}
	if updated.OS != "linux" || updated.Arch != "x86_64" {
		t.Fatalf("node = %+v", updated)
	}
	if updated.HostKey != "pinned" {
		t.Fatalf("host key = %q", updated.HostKey)
	}
	if updated.LastSeenAt == 0 {
		t.Fatal("the node must be marked as seen")
	}

	samples, err := store.Health(context.Background(), node.ID, 10)
	if err != nil {
		t.Fatalf("read the health: %v", err)
	}
	if len(samples) != 1 {
		t.Fatalf("samples = %d, want 1", len(samples))
	}
}

func TestTheCollectorReportsAFailure(t *testing.T) {
	store := testStore(t)
	keys := NewKeyStore(store.db)

	pair, err := panelssh.GenerateKeyPair("test")
	if err != nil {
		t.Fatalf("generate the client key: %v", err)
	}
	key, err := keys.Create(context.Background(), SSHKey{Name: "k", PublicKey: pair.PublicKey, PrivateKey: pair.PrivateKey})
	if err != nil {
		t.Fatalf("create the key: %v", err)
	}

	node, err := store.Create(context.Background(), Node{
		Name: "eu-1", Address: "10.0.0.5", SSHHost: "10.0.0.5", SSHUser: "root",
		SSHKeyID: key.ID, KeyInstalled: true, Enabled: true, Status: StatusConnecting,
	})
	if err != nil {
		t.Fatalf("create the node: %v", err)
	}

	collector := NewCollector(store, keys, func(ctx context.Context, node Node, privateKey string) (ProbeResult, error) {
		return ProbeResult{}, fmt.Errorf("the node is gone")
	}, CollectorOptions{})

	collector.Round(context.Background())

	updated, err := store.Get(context.Background(), node.ID)
	if err != nil {
		t.Fatalf("read the node: %v", err)
	}
	if updated.Status != StatusError {
		t.Fatalf("status = %q, want %q", updated.Status, StatusError)
	}
}
