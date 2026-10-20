package update

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/ryxoui/ryxoui/internal/db"
)

// Options configures the manager.
type Options struct {
	// CurrentVersion is the version of the running panel.
	CurrentVersion string
	// Repository is the "owner/name" pair the releases are read from.
	Repository string
	// ReleaseFeed overrides the repository, for example a self hosted feed.
	ReleaseFeed string
	// CheckInterval is the delay between two automatic checks.
	CheckInterval time.Duration
	// WorkDir is where downloads are stored. It defaults to the system
	// temporary directory.
	WorkDir string
	// Installer replaces the running binary.
	Installer *Installer
	// Restart is called after a successful install.
	Restart func(ctx context.Context) error
	// Logger receives the progress messages.
	Logger *slog.Logger
}

// Manager drives the update process.
type Manager struct {
	opts    Options
	store   *Store
	client  *GitHubClient
	http    *http.Client
	log     *slog.Logger
	mu      sync.RWMutex
	state   State
	started time.Time
}

// NewManager returns an update manager.
func NewManager(opts Options, store *Store) *Manager {
	if opts.CheckInterval <= 0 {
		opts.CheckInterval = 6 * time.Hour
	}
	if opts.WorkDir == "" {
		opts.WorkDir = os.TempDir()
	}
	if opts.Installer == nil {
		opts.Installer = NewInstaller()
	}
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}

	client := NewGitHubClient(opts.Repository)
	manager := &Manager{
		opts:   opts,
		store:  store,
		client: client,
		http:   &http.Client{Timeout: 10 * time.Minute},
		log:    logger,
	}
	manager.state = State{
		CurrentVersion: opts.CurrentVersion,
		Status:         StatusIdle,
	}
	return manager
}

// State returns the last known state.
func (m *Manager) State(ctx context.Context) State {
	m.mu.RLock()
	state := m.state
	m.mu.RUnlock()
	return state
}

func (m *Manager) setState(state State) {
	m.mu.Lock()
	m.state = state
	m.mu.Unlock()
}

// Check asks the release feed for the newest release and compares it with
// the running version.
func (m *Manager) Check(ctx context.Context) (State, error) {
	m.setState(State{
		CurrentVersion: m.opts.CurrentVersion,
		Status:         StatusChecking,
		Message:        "checking for updates",
		CheckedAt:      db.Now(),
	})

	release, err := m.client.Latest(ctx)
	if err != nil {
		state := m.State(ctx)
		state.Status = StatusFailed
		state.Message = err.Error()
		m.setState(state)
		_ = m.store.Save(ctx, state)
		return state, err
	}

	latest := release.TagName
	state := State{
		CurrentVersion: m.opts.CurrentVersion,
		LatestVersion:  latest,
		ReleaseURL:     release.HTMLURL,
		ReleaseNotes:   release.Body,
		CheckedAt:      db.Now(),
	}

	current, currentErr := Parse(m.opts.CurrentVersion)
	latestVersion, latestErr := Parse(latest)

	switch {
	case currentErr != nil || latestErr != nil:
		state.Status = StatusCurrent
		state.Message = "the running version is a development build"
	case latestVersion.IsNewer(current):
		assetName := AssetName(latest, Current().OS, Current().Arch)
		state.AssetName = assetName
		state.Status = StatusAvailable
		state.Message = fmt.Sprintf("version %s is available", latest)
	default:
		state.Status = StatusCurrent
		state.Message = fmt.Sprintf("version %s is the newest release", latest)
	}

	m.setState(state)
	if err := m.store.Save(ctx, state); err != nil {
		m.log.Error("update: save state", "error", err)
	}
	return state, nil
}

// Apply downloads the given release, verifies it and replaces the running
// binary.
//
// The version must be one that a previous check reported as available. When
// it is empty the newest available release is used.
func (m *Manager) Apply(ctx context.Context, version string) (State, error) {
	state, err := m.Check(ctx)
	if err != nil {
		return state, err
	}
	if version != "" && version != state.LatestVersion {
		state.Status = StatusFailed
		state.Message = fmt.Sprintf("version %s was not offered by the release feed", version)
		m.setState(state)
		_ = m.store.Save(ctx, state)
		return state, errors.New("update: unknown version")
	}
	if !state.UpdateAvailable() {
		state.Status = StatusCurrent
		state.Message = "the panel is already up to date"
		m.setState(state)
		_ = m.store.Save(ctx, state)
		return state, errors.New("update: no newer release is available")
	}

	release, err := m.client.Latest(ctx)
	if err != nil {
		return state, err
	}

	assetName := AssetName(state.LatestVersion, Current().OS, Current().Arch)
	asset, ok := release.Asset(assetName)
	if !ok {
		state.Status = StatusFailed
		state.Message = fmt.Sprintf("the release does not contain %s for %s", assetName, Current())
		m.setState(state)
		_ = m.store.Save(ctx, state)
		return state, fmt.Errorf("update: release has no artefact %s", assetName)
	}

	checksumAsset, ok := release.Asset(ChecksumAssetName(assetName))
	if !ok {
		state.Status = StatusFailed
		state.Message = "the release has no checksum file, refusing to install it"
		m.setState(state)
		_ = m.store.Save(ctx, state)
		return state, fmt.Errorf("update: release has no checksum for %s", assetName)
	}

	workDir, err := os.MkdirTemp(m.opts.WorkDir, "ryxoui-update-")
	if err != nil {
		return state, fmt.Errorf("update: create work directory: %w", err)
	}
	defer os.RemoveAll(workDir)

	checksumRaw, err := DownloadText(ctx, m.http, checksumAsset.DownloadURL)
	if err != nil {
		return state, err
	}
	expected, err := ParseChecksum(checksumRaw)
	if err != nil {
		state.Status = StatusFailed
		state.Message = err.Error()
		m.setState(state)
		_ = m.store.Save(ctx, state)
		return state, err
	}

	state.Status = StatusDownload
	state.Message = "downloading " + assetName
	state.AssetName = assetName
	state.Checksum = expected
	m.setState(state)
	_ = m.store.Save(ctx, state)

	archivePath, err := Download(ctx, m.http, asset.DownloadURL, workDir, sanitizeName(assetName))
	if err != nil {
		return state, err
	}

	actual, err := FileChecksum(archivePath)
	if err != nil {
		return state, err
	}
	if actual != expected {
		state.Status = StatusFailed
		state.Message = "the downloaded artefact does not match its checksum"
		m.setState(state)
		_ = m.store.Save(ctx, state)
		return state, fmt.Errorf("update: checksum mismatch for %s", assetName)
	}

	extractDir := filepath.Join(workDir, "extracted")
	if err := os.MkdirAll(extractDir, 0o755); err != nil {
		return state, fmt.Errorf("update: create extract directory: %w", err)
	}
	binaryPath, err := ExtractArchive(archivePath, extractDir)
	if err != nil {
		return state, err
	}

	if err := m.opts.Installer.Install(ctx, binaryPath); err != nil {
		state.Status = StatusFailed
		state.Message = err.Error()
		m.setState(state)
		_ = m.store.Save(ctx, state)
		return state, err
	}

	state.Status = StatusInstalled
	state.Message = fmt.Sprintf("version %s was installed, restart the panel to run it", state.LatestVersion)
	state.AppliedAt = AppliedAtNow()
	m.setState(state)
	_ = m.store.Save(ctx, state)

	if m.opts.Restart != nil {
		go func() {
			// Give the HTTP response time to reach the operator before the
			// service goes away.
			time.Sleep(2 * time.Second)
			if err := m.opts.Restart(context.Background()); err != nil {
				m.log.Error("update: restart failed", "error", err)
			}
		}()
	}
	return state, nil
}

// Run checks for updates on a fixed interval until the context is done.
func (m *Manager) Run(ctx context.Context) error {
	ticker := time.NewTicker(m.opts.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if _, err := m.Check(ctx); err != nil {
				m.log.Warn("update: check failed", "error", err)
			}
		}
	}
}
