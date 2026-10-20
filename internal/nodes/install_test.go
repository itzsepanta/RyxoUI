package nodes

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestTheInstallerSetsTheNodeUp(t *testing.T) {
	store := testStore(t)
	keys := NewKeyStore(store.db)
	jobs := NewJobStore(store.db)

	server := startSSHServer(t, "os=linux\narch=x86_64\nmem_total=1024\nmem_available=512\n")

	node, err := store.Create(context.Background(), Node{
		Name:         "eu-1",
		Address:      server.host,
		SSHHost:      server.host,
		SSHPort:      server.port,
		SSHUser:      "root",
		KeyInstalled: true,
		Enabled:      true,
		Status:       StatusConnecting,
	})
	if err != nil {
		t.Fatalf("create the node: %v", err)
	}

	installer := NewInstaller(store, keys, jobs, func(ctx context.Context, node Node, privateKey string) (ProbeResult, error) {
		return ProbeResult{OS: "linux", Arch: "x86_64", HostKey: "pinned"}, nil
	}, InstallerOptions{})

	job, err := installer.Run(context.Background(), node, "the password of the node")
	if err != nil {
		t.Fatalf("run the installer: %v", err)
	}

	if job.Status != JobSucceeded {
		t.Fatalf("status = %q, error = %q", job.Status, job.Error)
	}
	if job.Progress != 100 {
		t.Fatalf("progress = %d", job.Progress)
	}
	if !strings.Contains(installer.Log(job.ID), "the key of the panel is installed") {
		t.Fatalf("log = %q", installer.Log(job.ID))
	}

	updated, err := store.Get(context.Background(), node.ID)
	if err != nil {
		t.Fatalf("read the node: %v", err)
	}
	if updated.Status != StatusConnected {
		t.Fatalf("status = %q", updated.Status)
	}
	if updated.SSHKeyID == "" {
		t.Fatal("the node must have a key")
	}
	if updated.HostKey != "pinned" {
		t.Fatalf("host key = %q", updated.HostKey)
	}

	// The job is stored, so the operator can read it later.
	stored, err := jobs.Get(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("read the job: %v", err)
	}
	if stored.Status != JobSucceeded {
		t.Fatalf("stored status = %q", stored.Status)
	}

	// The password is nowhere to be found.
	all, err := jobs.List(context.Background(), node.ID, 10)
	if err != nil {
		t.Fatalf("list the jobs: %v", err)
	}
	for _, item := range all {
		if strings.Contains(item.Log, "the password of the node") {
			t.Fatal("the password must never reach the log")
		}
	}
}

func TestTheInstallerReportsAFailure(t *testing.T) {
	store := testStore(t)
	keys := NewKeyStore(store.db)
	jobs := NewJobStore(store.db)

	node, err := store.Create(context.Background(), Node{
		Name: "eu-1", Address: "127.0.0.1", SSHHost: "127.0.0.1", SSHPort: 1, SSHUser: "root",
	})
	if err != nil {
		t.Fatalf("create the node: %v", err)
	}

	installer := NewInstaller(store, keys, jobs, nil, InstallerOptions{Timeout: 5 * time.Second})

	job, err := installer.Run(context.Background(), node, "wrong")
	if err == nil {
		t.Fatal("the installer must report the failure")
	}
	if job.Status != JobFailed {
		t.Fatalf("status = %q", job.Status)
	}
	if job.Error == "" {
		t.Fatal("the job must carry the reason")
	}
	if !strings.Contains(installer.Log(job.ID), "failed") {
		t.Fatalf("log = %q", installer.Log(job.ID))
	}
}

func TestTheJobsAreListedNewestFirst(t *testing.T) {
	store := testStore(t)
	jobs := NewJobStore(store.db)

	node, err := store.Create(context.Background(), Node{Name: "eu-1", Address: "10.0.0.5"})
	if err != nil {
		t.Fatalf("create the node: %v", err)
	}

	first, err := jobs.Create(context.Background(), node.ID)
	if err != nil {
		t.Fatalf("create the job: %v", err)
	}
	second, err := jobs.Create(context.Background(), node.ID)
	if err != nil {
		t.Fatalf("create the job: %v", err)
	}

	list, err := jobs.List(context.Background(), node.ID, 10)
	if err != nil {
		t.Fatalf("list the jobs: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("jobs = %d, want 2", len(list))
	}
	if list[0].ID != second.ID || list[1].ID != first.ID {
		t.Fatalf("the newest job must come first: %v", list)
	}

	latest, err := jobs.Latest(context.Background(), node.ID)
	if err != nil {
		t.Fatalf("read the latest job: %v", err)
	}
	if latest.ID != second.ID {
		t.Fatalf("latest = %q", latest.ID)
	}
}
