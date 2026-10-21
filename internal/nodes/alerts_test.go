package nodes

import (
	"context"
	"errors"
	"testing"
)

func TestTheCollectorWarnsAboutANodeThatDoesNotAnswer(t *testing.T) {
	store := testStore(t)
	keys := NewKeyStore(store.db)
	alerts := NewAlertStore(store.db)

	key, err := keys.Create(context.Background(), SSHKey{Name: "k", PublicKey: "pub", PrivateKey: "priv"})
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

	collector := NewCollector(store, keys, alerts, func(ctx context.Context, node Node, privateKey string) (ProbeResult, error) {
		return ProbeResult{}, errors.New("the node is gone")
	}, CollectorOptions{})

	collector.Round(context.Background())

	list, err := alerts.Alerts(context.Background(), node.ID, true)
	if err != nil {
		t.Fatalf("read the warnings: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("warnings = %d, want 1", len(list))
	}
	if list[0].Kind != AlertUnreachable {
		t.Fatalf("kind = %q", list[0].Kind)
	}
	if list[0].Severity != SeverityCritical {
		t.Fatalf("severity = %q", list[0].Severity)
	}
}

func TestTheCollectorWarnsAboutAFullDisk(t *testing.T) {
	store := testStore(t)
	keys := NewKeyStore(store.db)
	alerts := NewAlertStore(store.db)

	key, err := keys.Create(context.Background(), SSHKey{Name: "k", PublicKey: "pub", PrivateKey: "priv"})
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

	full := true
	collector := NewCollector(store, keys, alerts, func(ctx context.Context, node Node, privateKey string) (ProbeResult, error) {
		diskUsed := int64(95)
		if !full {
			diskUsed = 10
		}
		return ProbeResult{OS: "linux", Arch: "x86_64", DiskTotal: 100, DiskUsed: diskUsed}, nil
	}, CollectorOptions{})

	collector.Round(context.Background())

	list, err := alerts.Alerts(context.Background(), node.ID, true)
	if err != nil {
		t.Fatalf("read the warnings: %v", err)
	}
	if len(list) != 1 || list[0].Kind != AlertDisk {
		t.Fatalf("warnings = %+v", list)
	}

	// The disk has room again, so the warning ends.
	full = false
	collector.Round(context.Background())

	list, err = alerts.Alerts(context.Background(), node.ID, true)
	if err != nil {
		t.Fatalf("read the warnings: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("the warning must be gone: %+v", list)
	}
}
