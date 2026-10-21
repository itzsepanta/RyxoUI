package nodes

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/ryxoui/ryxoui/internal/db"
	"github.com/ryxoui/ryxoui/internal/engines"
	"github.com/ryxoui/ryxoui/internal/id"
	panelssh "github.com/ryxoui/ryxoui/internal/ssh"
)

// The status of an install job.
const (
	JobQueued    = "queued"
	JobRunning   = "running"
	JobSucceeded = "succeeded"
	JobFailed    = "failed"
)

// ErrJobNotFound is returned when a job does not exist.
var ErrJobNotFound = errors.New("nodes: install job not found")

// InstallJob is the work of setting a node up.
type InstallJob struct {
	ID         string `json:"id"`
	NodeID     string `json:"nodeId"`
	Status     string `json:"status"`
	Step       string `json:"step"`
	Progress   int    `json:"progress"`
	Log        string `json:"log"`
	Error      string `json:"error"`
	StartedAt  int64  `json:"startedAt"`
	FinishedAt int64  `json:"finishedAt"`
}

// JobStore reads and writes the install jobs.
type JobStore struct {
	db db.Querier
}

// NewJobStore returns a job store.
func NewJobStore(q db.Querier) *JobStore { return &JobStore{db: q} }

// Create stores a new job for a node.
func (s *JobStore) Create(ctx context.Context, nodeID string) (InstallJob, error) {
	job := InstallJob{
		ID:        id.New(),
		NodeID:    nodeID,
		Status:    JobQueued,
		StartedAt: db.Now(),
	}
	_, err := s.db.ExecContext(ctx,
		"INSERT INTO install_jobs (id, node_id, status, step, progress, log, error, started_at, finished_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
		job.ID, job.NodeID, job.Status, job.Step, job.Progress, job.Log, job.Error, job.StartedAt, job.FinishedAt)
	if err != nil {
		return InstallJob{}, fmt.Errorf("nodes: create the install job: %w", err)
	}
	return job, nil
}

// Update writes the state of a job.
func (s *JobStore) Update(ctx context.Context, job InstallJob) error {
	_, err := s.db.ExecContext(ctx,
		"UPDATE install_jobs SET status = ?, step = ?, progress = ?, log = ?, error = ?, finished_at = ? WHERE id = ?",
		job.Status, job.Step, job.Progress, job.Log, job.Error, job.FinishedAt, job.ID)
	if err != nil {
		return fmt.Errorf("nodes: update the install job: %w", err)
	}
	return nil
}

// Get reads one job.
func (s *JobStore) Get(ctx context.Context, jobID string) (InstallJob, error) {
	var job InstallJob
	err := s.db.QueryRowContext(ctx,
		"SELECT id, node_id, status, step, progress, log, error, started_at, finished_at FROM install_jobs WHERE id = ?", jobID).
		Scan(&job.ID, &job.NodeID, &job.Status, &job.Step, &job.Progress, &job.Log, &job.Error, &job.StartedAt, &job.FinishedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return InstallJob{}, ErrJobNotFound
	}
	if err != nil {
		return InstallJob{}, fmt.Errorf("nodes: read the install job: %w", err)
	}
	return job, nil
}

// Latest returns the newest job of a node.
func (s *JobStore) Latest(ctx context.Context, nodeID string) (InstallJob, error) {
	var job InstallJob
	err := s.db.QueryRowContext(ctx,
		`SELECT id, node_id, status, step, progress, log, error, started_at, finished_at FROM install_jobs
		 WHERE node_id = ? ORDER BY started_at DESC LIMIT 1`, nodeID).
		Scan(&job.ID, &job.NodeID, &job.Status, &job.Step, &job.Progress, &job.Log, &job.Error, &job.StartedAt, &job.FinishedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return InstallJob{}, ErrJobNotFound
	}
	if err != nil {
		return InstallJob{}, fmt.Errorf("nodes: read the install job: %w", err)
	}
	return job, nil
}

// List returns the jobs of a node, newest first.
func (s *JobStore) List(ctx context.Context, nodeID string, limit int) ([]InstallJob, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, node_id, status, step, progress, log, error, started_at, finished_at FROM install_jobs
		 WHERE node_id = ? ORDER BY started_at DESC LIMIT ?`, nodeID, limit)
	if err != nil {
		return nil, fmt.Errorf("nodes: list the install jobs: %w", err)
	}
	defer rows.Close()

	out := []InstallJob{}
	for rows.Next() {
		var job InstallJob
		if err := rows.Scan(&job.ID, &job.NodeID, &job.Status, &job.Step, &job.Progress, &job.Log, &job.Error, &job.StartedAt, &job.FinishedAt); err != nil {
			return nil, fmt.Errorf("nodes: scan the install job: %w", err)
		}
		out = append(out, job)
	}
	return out, rows.Err()
}

// Installer sets a node up over SSH.
//
// The job keeps a log of every step, so an operator can follow the work and
// read what went wrong. The password is only used for the first contact and
// is never written to the log or to the database.
type Installer struct {
	store  *Store
	keys   *KeyStore
	jobs   *JobStore
	config *ConfigStore
	probe  func(ctx context.Context, node Node, privateKey string) (ProbeResult, error)
	logger *slog.Logger

	mu   sync.Mutex
	logs map[string]*strings.Builder
	opts InstallerOptions
}

// InstallerOptions configures the installer.
type InstallerOptions struct {
	// Logger receives the reports.
	Logger *slog.Logger
	// Timeout bounds one step. Zero means two minutes.
	Timeout time.Duration
	// Engine is the protocol engine the job puts on the node. It may be
	// nil, then the job only prepares the node.
	Engine engines.Engine
	// EngineVersion is the version of the engine the job installs. Empty
	// means the newest one the panel knows.
	EngineVersion string
}

// NewInstaller returns an installer.
func NewInstaller(store *Store, keys *KeyStore, jobs *JobStore, config *ConfigStore, probe func(ctx context.Context, node Node, privateKey string) (ProbeResult, error), opts InstallerOptions) *Installer {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	if probe == nil {
		probe = Probe
	}
	return &Installer{
		store:  store,
		keys:   keys,
		jobs:   jobs,
		config: config,
		probe:  probe,
		logger: opts.Logger,
		logs:   map[string]*strings.Builder{},
		opts:   opts,
	}
}

// Start creates a job and runs it in the background. The job is returned
// right away, so the caller can follow it while it works.
func (i *Installer) Start(ctx context.Context, node Node, password string) (InstallJob, error) {
	job, err := i.jobs.Create(ctx, node.ID)
	if err != nil {
		return InstallJob{}, err
	}
	i.reset(job.ID)

	go func() {
		if _, err := i.run(context.WithoutCancel(ctx), job, node, password); err != nil {
			i.logger.Error("the install job failed", "job", job.ID, "error", err)
		}
	}()
	return job, nil
}

// Run sets a node up and waits for the end of the work.
func (i *Installer) Run(ctx context.Context, node Node, password string) (InstallJob, error) {
	job, err := i.jobs.Create(ctx, node.ID)
	if err != nil {
		return InstallJob{}, err
	}
	i.reset(job.ID)
	return i.run(ctx, job, node, password)
}

// run does the work of a job.
func (i *Installer) run(ctx context.Context, job InstallJob, node Node, password string) (InstallJob, error) {
	job.Status = JobRunning
	job.Step = "connect"
	job.Progress = 5
	if err := i.jobs.Update(ctx, job); err != nil {
		return InstallJob{}, err
	}
	i.write(job.ID, "connecting to %s as %s\n", node.SSHHostName(), node.SSHUser)

	timeout := i.opts.Timeout
	if timeout <= 0 {
		timeout = 2 * time.Minute
	}
	stepCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	fail := func(step string, err error) (InstallJob, error) {
		job.Status = JobFailed
		job.Step = step
		job.Error = err.Error()
		job.FinishedAt = db.Now()
		i.write(job.ID, "failed: %s\n", err.Error())
		if updateErr := i.jobs.Update(ctx, job); updateErr != nil {
			i.logger.Error("update the install job", "job", job.ID, "error", updateErr)
		}
		return job, err
	}

	// The first contact uses the password, so the key of the panel can be
	// installed on the node.
	client, session, err := i.connect(stepCtx, node, password, "")
	if err != nil {
		return fail("connect", err)
	}
	defer session.Close()
	i.write(job.ID, "connected\n")

	// The key of the panel is installed on the node.
	job.Step = "keys"
	job.Progress = 35
	if err := i.jobs.Update(ctx, job); err != nil {
		return InstallJob{}, err
	}

	key, err := i.keyFor(ctx, node)
	if err != nil {
		return fail("keys", err)
	}
	node.SSHKeyID = key.ID
	if err := i.installKey(stepCtx, session, key.PublicKey); err != nil {
		return fail("keys", err)
	}
	i.write(job.ID, "the key of the panel is installed\n")

	// The key of the panel works from now on, and the host key of the node is
	// pinned, so a machine that changes it is reported later on.
	node.KeyInstalled = true
	if pinned := client.KnownHost(); pinned != "" && node.HostKey == "" {
		node.HostKey = pinned
	}
	if err := i.store.Update(stepCtx, node); err != nil {
		return fail("keys", err)
	}

	// The directories of the panel are prepared on the node.
	job.Step = "prepare"
	job.Progress = 60
	if err := i.jobs.Update(ctx, job); err != nil {
		return InstallJob{}, err
	}
	if err := i.prepare(stepCtx, session); err != nil {
		return fail("prepare", err)
	}
	i.write(job.ID, "the directories of the panel are ready\n")

	// The engine of the node is put on it and started with the
	// configuration of the node.
	if i.opts.Engine != nil {
		job.Step = "engine"
		job.Progress = 70
		if err := i.jobs.Update(ctx, job); err != nil {
			return InstallJob{}, err
		}
		if err := i.installEngine(stepCtx, node, key.PrivateKey); err != nil {
			return fail("engine", err)
		}
		i.write(job.ID, "the engine %s is installed\n", i.opts.Engine.Name())
	}

	// The node is asked whether it is alive, this time with the key.
	job.Step = "verify"
	job.Progress = 85
	if err := i.jobs.Update(ctx, job); err != nil {
		return InstallJob{}, err
	}

	result, err := i.probeWithKey(stepCtx, node, key.PrivateKey)
	if err != nil {
		return fail("verify", err)
	}
	if err := i.store.ApplyProbe(stepCtx, node.ID, result, StatusConnected, ""); err != nil {
		return fail("verify", err)
	}
	if err := i.store.RecordHealth(stepCtx, result.HealthSample(node.ID, db.Now())); err != nil {
		i.logger.Error("record the health of the node", "node", node.Name, "error", err)
	}
	i.write(job.ID, "the node answered: %s %s\n", result.OS, result.Arch)

	job.Status = JobSucceeded
	job.Step = "done"
	job.Progress = 100
	job.FinishedAt = db.Now()
	if err := i.jobs.Update(ctx, job); err != nil {
		return InstallJob{}, err
	}
	i.write(job.ID, "the node is ready\n")
	return job, nil
}

// installEngine puts the protocol engine on the node and applies the
// configuration of the node.
func (i *Installer) installEngine(ctx context.Context, node Node, privateKey string) error {
	// The engine has to know the platform of the node, so the node is asked
	// when the panel does not know it yet.
	if node.OS == "" || node.Arch == "" {
		result, err := i.probe(ctx, node, privateKey)
		if err != nil {
			return fmt.Errorf("nodes: read the platform of the node: %w", err)
		}
		node.OS = result.OS
		node.Arch = result.Arch
		node.CoreVersion = result.CoreVersion
		node.NodeVersion = result.NodeVersion
		if err := i.store.ApplyProbe(ctx, node.ID, result, node.Status, node.Message); err != nil {
			return err
		}
	}

	config, err := i.ConfigOf(ctx, node.ID)
	if err != nil {
		return err
	}

	target := engines.Target{
		NodeID:     node.ID,
		Host:       node.SSHHostName(),
		Port:       node.SSHPort,
		User:       node.SSHUser,
		PrivateKey: privateKey,
		KnownHost:  node.HostKey,
		OS:         node.OS,
		Arch:       node.Arch,
	}

	if err := i.opts.Engine.Install(ctx, target, i.opts.EngineVersion); err != nil {
		return err
	}
	return i.opts.Engine.Apply(ctx, target, config)
}

// ConfigOf builds the configuration of a node from the store.
func (i *Installer) ConfigOf(ctx context.Context, nodeID string) (engines.Config, error) {
	config := engines.Config{}

	if i.config == nil {
		return config, nil
	}

	entrypoints, err := i.config.EntryPoints(ctx, nodeID)
	if err != nil {
		return config, err
	}
	for _, item := range entrypoints {
		config.Entrypoints = append(config.Entrypoints, engines.Entrypoint{
			ID:      item.ID,
			Name:    item.Name,
			Tag:     item.Tag,
			Type:    item.Type,
			Config:  item.Config,
			Enabled: item.Enabled,
		})
	}

	egresses, err := i.config.Egresses(ctx, nodeID)
	if err != nil {
		return config, err
	}
	for _, item := range egresses {
		config.Egresses = append(config.Egresses, engines.Egress{
			ID:      item.ID,
			Name:    item.Name,
			Tag:     item.Tag,
			Type:    item.Type,
			Config:  item.Config,
			Enabled: item.Enabled,
		})
	}

	endpoints, err := i.config.Endpoints(ctx, nodeID)
	if err != nil {
		return config, err
	}
	for _, item := range endpoints {
		// A destination carries its tag and its state in its settings, the
		// table itself only knows the name and the type.
		tag, _ := item.Config["tag"].(string)
		enabled := true
		if disabled, ok := item.Config["disabled"].(bool); ok {
			enabled = !disabled
		}
		config.Endpoints = append(config.Endpoints, engines.Endpoint{
			ID:      item.ID,
			Name:    item.Name,
			Tag:     tag,
			Type:    item.Type,
			Config:  item.Config,
			Enabled: enabled,
		})
	}

	return config, nil
}

// Log returns the log of a job while it runs.
func (i *Installer) Log(jobID string) string {
	i.mu.Lock()
	defer i.mu.Unlock()
	builder, ok := i.logs[jobID]
	if !ok {
		return ""
	}
	return builder.String()
}

// reset starts a new log for a job.
func (i *Installer) reset(jobID string) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.logs[jobID] = &strings.Builder{}
}

// write adds a line to the log of a job.
func (i *Installer) write(jobID, format string, args ...any) {
	i.mu.Lock()
	defer i.mu.Unlock()
	builder, ok := i.logs[jobID]
	if !ok {
		builder = &strings.Builder{}
		i.logs[jobID] = builder
	}
	fmt.Fprintf(builder, format, args...)
}

// connect opens a session to the node.
func (i *Installer) connect(ctx context.Context, node Node, password, privateKey string) (*panelssh.Client, *panelssh.Session, error) {
	client, err := panelssh.NewClient(panelssh.Config{
		Host:       node.SSHHostName(),
		Port:       node.SSHPort,
		User:       node.SSHUser,
		PrivateKey: privateKey,
		Password:   password,
		KnownHost:  node.HostKey,
	})
	if err != nil {
		return nil, nil, err
	}
	session, err := client.Connect(ctx)
	if err != nil {
		return nil, nil, err
	}
	return client, session, nil
}

// keyFor reads the key of a node or creates one.
func (i *Installer) keyFor(ctx context.Context, node Node) (SSHKey, error) {
	if node.SSHKeyID != "" {
		key, err := i.keys.Get(ctx, node.SSHKeyID)
		if err == nil {
			return key, nil
		}
		if !errors.Is(err, ErrKeyNotFound) {
			return SSHKey{}, err
		}
	}

	pair, err := panelssh.GenerateKeyPair(node.Name)
	if err != nil {
		return SSHKey{}, err
	}
	key, err := i.keys.Create(ctx, SSHKey{
		Name:       node.Name + "-key",
		PublicKey:  pair.PublicKey,
		PrivateKey: pair.PrivateKey,
		Comment:    "ryxoui",
	})
	if err != nil {
		return SSHKey{}, err
	}

	node.SSHKeyID = key.ID
	if err := i.store.Update(ctx, node); err != nil {
		return SSHKey{}, err
	}
	return key, nil
}

// installKey puts the public key of the panel into the authorized keys of
// the node.
func (i *Installer) installKey(ctx context.Context, session *panelssh.Session, publicKey string) error {
	publicKey = strings.TrimSpace(publicKey)
	if publicKey == "" {
		return errors.New("nodes: the key of the panel is empty")
	}

	script := strings.Join([]string{
		"set -e",
		"mkdir -p ~/.ssh",
		"chmod 700 ~/.ssh",
		"touch ~/.ssh/authorized_keys",
		"chmod 600 ~/.ssh/authorized_keys",
		fmt.Sprintf("grep -qxF %s ~/.ssh/authorized_keys || echo %s >> ~/.ssh/authorized_keys",
			shellQuote(publicKey), shellQuote(publicKey)),
	}, "\n")

	_, stderr, code, err := session.Run(ctx, script)
	if err != nil {
		return fmt.Errorf("nodes: install the key: %w", err)
	}
	if code != 0 {
		return fmt.Errorf("nodes: install the key: the node answered with code %d: %s", code, strings.TrimSpace(stderr))
	}
	return nil
}

// prepare creates the directories the panel uses on the node.
func (i *Installer) prepare(ctx context.Context, session *panelssh.Session) error {
	script := "set -e\nmkdir -p /opt/ryxoui /var/log/ryxoui\nid ryxoui >/dev/null 2>&1 || useradd --system --home /opt/ryxoui --shell /usr/sbin/nologin ryxoui || true\nchown -R ryxoui:ryxoui /opt/ryxoui /var/log/ryxoui 2>/dev/null || true"

	_, stderr, code, err := session.Run(ctx, script)
	if err != nil {
		return fmt.Errorf("nodes: prepare the node: %w", err)
	}
	if code != 0 {
		return fmt.Errorf("nodes: prepare the node: the node answered with code %d: %s", code, strings.TrimSpace(stderr))
	}
	return nil
}

// probeWithKey asks the node about itself with the key of the panel.
func (i *Installer) probeWithKey(ctx context.Context, node Node, privateKey string) (ProbeResult, error) {
	return i.probe(ctx, node, privateKey)
}

// shellQuote puts a value between quotes so a shell keeps it as one word.
func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}
