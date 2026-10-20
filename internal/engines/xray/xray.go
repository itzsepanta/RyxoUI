// Package xray drives the Xray engine on a node.
//
// The panel keeps the configuration of a node in its own language and this
// adapter translates it into the language of Xray: a JSON file with the
// listeners, the outgoing connections and the routing rules.
package xray

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/ryxoui/ryxoui/internal/engines"
	"github.com/ryxoui/ryxoui/internal/ssh"
	"github.com/ryxoui/ryxoui/internal/update"
)

func init() {
	engines.Register("xray", func() engines.Engine { return New() })
}

// The names the engine uses on a node.
const (
	// DefaultService is the name of the systemd unit of the engine.
	DefaultService = "ryxoui-xray"
	// DefaultDirectory is where the engine lives on a node.
	DefaultDirectory = "/opt/ryxoui/xray"
	// BinaryPath is where the binary of the engine is installed.
	BinaryPath = "/usr/local/bin/xray"
	// Repository is the release feed of the engine.
	Repository = "XTLS/Xray-core"
	// ServiceUser is the account the engine runs as.
	ServiceUser = "ryxoui"
)

// Engine is the Xray adapter.
type Engine struct {
	// Releases reads the release feed of the engine.
	Releases *update.GitHubClient
	// HTTP downloads the archives of the releases.
	HTTP *http.Client
}

// New returns the adapter of the engine.
func New() *Engine {
	return &Engine{
		Releases: update.NewGitHubClient(Repository),
		HTTP:     &http.Client{Timeout: 5 * time.Minute},
	}
}

// Name is the name of the engine.
func (e *Engine) Name() string { return "xray" }

// Versions returns the releases the panel can install.
func (e *Engine) Versions(ctx context.Context) ([]engines.Release, error) {
	client := e.Releases
	if client == nil {
		client = update.NewGitHubClient(Repository)
	}

	releases, err := client.Releases(ctx)
	if err != nil {
		return nil, fmt.Errorf("xray: read the releases: %w", err)
	}

	out := []engines.Release{}
	for _, release := range releases {
		if release.Draft || release.Prerelease {
			continue
		}
		version := strings.TrimPrefix(release.TagName, "v")
		for _, asset := range release.Assets {
			if !strings.HasSuffix(asset.Name, ".zip") {
				continue
			}
			out = append(out, engines.Release{
				Version: version,
				Asset:   asset.Name,
				URL:     asset.DownloadURL,
				Size:    asset.Size,
			})
		}
	}
	return out, nil
}

// Install puts the engine on a node and starts it.
func (e *Engine) Install(ctx context.Context, target engines.Target, version string) error {
	release, err := e.release(ctx, target, version)
	if err != nil {
		return err
	}

	binary, err := e.download(ctx, release)
	if err != nil {
		return err
	}

	session, err := e.session(ctx, target)
	if err != nil {
		return err
	}
	defer session.Close()

	// The binary of the engine is placed and made executable.
	if err := session.UploadBytes(ctx, binary, BinaryPath); err != nil {
		return fmt.Errorf("xray: upload the binary: %w", err)
	}
	if _, stderr, code, err := session.Run(ctx, "chmod 0755 "+BinaryPath); err != nil {
		return fmt.Errorf("xray: make the binary executable: %w", err)
	} else if code != 0 {
		return fmt.Errorf("xray: make the binary executable: %s", strings.TrimSpace(stderr))
	}

	// The unit of the engine is written.
	unit := fmt.Sprintf(`[Unit]
Description=RYXOUIRyxoUI Xray engine
Documentation=https://github.com/%s
After=network.target

[Service]
Type=simple
User=%s
ExecStart=%s run -c %s
Restart=on-failure
RestartSec=5
LimitNOFILE=1048576

[Install]
WantedBy=multi-user.target
`, Repository, ServiceUser, BinaryPath, configPath(target))

	if err := session.UploadBytes(ctx, []byte(unit), unitPath(target)); err != nil {
		return fmt.Errorf("xray: write the unit: %w", err)
	}

	// The directories of the engine are prepared and the engine is started.
	prepare := fmt.Sprintf("set -e\nmkdir -p %s\nid %s >/dev/null 2>&1 || useradd --system --home %s --shell /usr/sbin/nologin %s\nchown -R %s:%s %s\nsystemctl daemon-reload\nsystemctl enable %s",
		target.Directory, ServiceUser, target.Directory, ServiceUser, ServiceUser, ServiceUser, target.Directory, serviceName(target))

	if _, stderr, code, err := session.Run(ctx, prepare); err != nil {
		return fmt.Errorf("xray: prepare the node: %w", err)
	} else if code != 0 {
		return fmt.Errorf("xray: prepare the node: %s", strings.TrimSpace(stderr))
	}

	return nil
}

// Apply writes the configuration on a node and restarts the engine.
func (e *Engine) Apply(ctx context.Context, target engines.Target, config engines.Config) error {
	content, err := e.Render(ctx, target, config)
	if err != nil {
		return err
	}

	session, err := e.session(ctx, target)
	if err != nil {
		return err
	}
	defer session.Close()

	if err := session.UploadBytes(ctx, content, configPath(target)); err != nil {
		return fmt.Errorf("xray: write the configuration: %w", err)
	}

	restart := fmt.Sprintf("chown -R %s:%s %s && systemctl restart %s",
		ServiceUser, ServiceUser, target.Directory, serviceName(target))
	if _, stderr, code, err := session.Run(ctx, restart); err != nil {
		return fmt.Errorf("xray: restart the engine: %w", err)
	} else if code != 0 {
		return fmt.Errorf("xray: restart the engine: %s", strings.TrimSpace(stderr))
	}
	return nil
}

// Health reports the state of the engine on a node.
func (e *Engine) Health(ctx context.Context, target engines.Target) (engines.Health, error) {
	session, err := e.session(ctx, target)
	if err != nil {
		return engines.Health{}, err
	}
	defer session.Close()

	health := engines.Health{}

	stdout, _, code, err := session.Run(ctx, "systemctl is-active "+serviceName(target))
	if err != nil {
		return engines.Health{}, fmt.Errorf("xray: read the state of the engine: %w", err)
	}
	health.Running = code == 0 && strings.TrimSpace(stdout) == "active"

	stdout, _, _, err = session.Run(ctx, BinaryPath+" version 2>/dev/null | head -n 1")
	if err == nil {
		health.Version = strings.TrimSpace(stdout)
	}
	return health, nil
}

// Users returns the clients the engine knows on a node.
func (e *Engine) Users(ctx context.Context, target engines.Target) ([]engines.User, error) {
	session, err := e.session(ctx, target)
	if err != nil {
		return nil, err
	}
	defer session.Close()

	stdout, stderr, code, err := session.Run(ctx, "cat "+configPath(target))
	if err != nil {
		return nil, fmt.Errorf("xray: read the configuration: %w", err)
	}
	if code != 0 {
		return nil, fmt.Errorf("xray: read the configuration: %s", strings.TrimSpace(stderr))
	}
	return clientsOf([]byte(stdout))
}

// UpsertUser adds a client to a node or changes it.
func (e *Engine) UpsertUser(ctx context.Context, target engines.Target, user engines.User) error {
	return e.writeUsers(ctx, target, func(users []engines.User) []engines.User {
		for i, existing := range users {
			if existing.ID == user.ID || (user.UUID != "" && existing.UUID == user.UUID) {
				users[i] = user
				return users
			}
		}
		return append(users, user)
	})
}

// SuspendUser removes a client from a node without losing its record.
func (e *Engine) SuspendUser(ctx context.Context, target engines.Target, userID string) error {
	return e.writeUsers(ctx, target, func(users []engines.User) []engines.User {
		out := make([]engines.User, 0, len(users))
		for _, user := range users {
			if user.ID == userID || user.UUID == userID {
				continue
			}
			out = append(out, user)
		}
		return out
	})
}

// RemoveUser removes a client from a node.
func (e *Engine) RemoveUser(ctx context.Context, target engines.Target, userID string) error {
	return e.SuspendUser(ctx, target, userID)
}

// writeUsers changes the clients of a node and restarts the engine.
func (e *Engine) writeUsers(ctx context.Context, target engines.Target, change func([]engines.User) []engines.User) error {
	session, err := e.session(ctx, target)
	if err != nil {
		return err
	}
	defer session.Close()

	stdout, stderr, code, err := session.Run(ctx, "cat "+configPath(target))
	if err != nil {
		return fmt.Errorf("xray: read the configuration: %w", err)
	}
	if code != 0 {
		return fmt.Errorf("xray: read the configuration: %s", strings.TrimSpace(stderr))
	}

	users, err := clientsOf([]byte(stdout))
	if err != nil {
		return err
	}

	users = change(users)
	content, err := renderUsers(users)
	if err != nil {
		return err
	}

	if err := session.UploadBytes(ctx, content, configPath(target)); err != nil {
		return fmt.Errorf("xray: write the configuration: %w", err)
	}
	if _, stderr, code, err := session.Run(ctx, "systemctl restart "+serviceName(target)); err != nil {
		return fmt.Errorf("xray: restart the engine: %w", err)
	} else if code != 0 {
		return fmt.Errorf("xray: restart the engine: %s", strings.TrimSpace(stderr))
	}
	return nil
}

// release finds the release the panel should install.
func (e *Engine) release(ctx context.Context, target engines.Target, version string) (engines.Release, error) {
	releases, err := e.Versions(ctx)
	if err != nil {
		return engines.Release{}, err
	}

	asset := assetName(target)
	if asset == "" {
		return engines.Release{}, fmt.Errorf("xray: %s/%s is not supported", target.OS, target.Arch)
	}

	for _, release := range releases {
		if version != "" && release.Version != version {
			continue
		}
		if release.Asset == asset {
			return release, nil
		}
	}
	return engines.Release{}, fmt.Errorf("xray: no release for %s was found", asset)
}

// download fetches the binary of a release.
func (e *Engine) download(ctx context.Context, release engines.Release) ([]byte, error) {
	client := e.HTTP
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Minute}
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, release.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("xray: build the download request: %w", err)
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("xray: download %s: %w", release.Asset, err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("xray: download %s: the server answered with %d", release.Asset, response.StatusCode)
	}

	archive, err := io.ReadAll(io.LimitReader(response.Body, 128<<20))
	if err != nil {
		return nil, fmt.Errorf("xray: read %s: %w", release.Asset, err)
	}
	return binaryOf(archive)
}

// binaryOf takes the binary of the engine out of its archive.
func binaryOf(archive []byte) ([]byte, error) {
	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return nil, fmt.Errorf("xray: read the archive: %w", err)
	}

	for _, file := range reader.File {
		if path.Base(file.Name) != "xray" {
			continue
		}
		content, err := file.Open()
		if err != nil {
			return nil, fmt.Errorf("xray: read the binary: %w", err)
		}
		defer content.Close()

		binary, err := io.ReadAll(io.LimitReader(content, 128<<20))
		if err != nil {
			return nil, fmt.Errorf("xray: read the binary: %w", err)
		}
		return binary, nil
	}
	return nil, fmt.Errorf("xray: the archive holds no binary")
}

// session opens a session to the node of a target.
func (e *Engine) session(ctx context.Context, target engines.Target) (*ssh.Session, error) {
	client, err := ssh.NewClient(ssh.Config{
		Host:       target.Host,
		Port:       target.Port,
		User:       target.User,
		PrivateKey: target.PrivateKey,
		KnownHost:  target.KnownHost,
	})
	if err != nil {
		return nil, err
	}
	return client.Connect(ctx)
}

// assetName is the name of the archive of the engine for a platform.
func assetName(target engines.Target) string {
	if target.OS != "" && target.OS != "linux" {
		return ""
	}
	switch target.Arch {
	case "amd64", "x86_64":
		return "Xray-linux-64.zip"
	case "arm64", "aarch64":
		return "Xray-linux-arm64-v8a.zip"
	default:
		return ""
	}
}

// serviceName is the name of the systemd unit of the engine.
func serviceName(target engines.Target) string {
	if target.Service != "" {
		return target.Service
	}
	return DefaultService
}

// configPath is where the configuration of the engine lives.
func configPath(target engines.Target) string {
	return path.Join(directory(target), "config.json")
}

// unitPath is where the systemd unit of the engine lives.
func unitPath(target engines.Target) string {
	return "/etc/systemd/system/" + serviceName(target) + ".service"
}

// directory is where the engine lives on the node.
func directory(target engines.Target) string {
	if target.Directory != "" {
		return target.Directory
	}
	return DefaultDirectory
}

// decodeConfig reads a configuration of the engine.
func decodeConfig(raw []byte) (map[string]any, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return map[string]any{}, nil
	}
	var config map[string]any
	if err := json.Unmarshal(raw, &config); err != nil {
		return nil, fmt.Errorf("xray: decode the configuration: %w", err)
	}
	return config, nil
}
