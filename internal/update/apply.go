package update

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Installer replaces the running executable with a new one.
//
// The running binary is renamed instead of overwritten, which works on
// Windows as well because a running image can be renamed but not written.
type Installer struct {
	// ExecutablePath is the binary to replace. It defaults to the running
	// executable.
	ExecutablePath string
	// KeepBackup keeps the previous binary next to the new one.
	KeepBackup bool
}

// NewInstaller returns an installer for the running executable.
func NewInstaller() *Installer {
	path, err := os.Executable()
	if err != nil {
		path = ""
	}
	return &Installer{ExecutablePath: path, KeepBackup: true}
}

// Install copies the new binary over the running one.
func (i *Installer) Install(ctx context.Context, newBinary string) error {
	target := i.ExecutablePath
	if target == "" {
		return errors.New("update: the path of the running executable is unknown")
	}

	source, err := os.Open(newBinary)
	if err != nil {
		return fmt.Errorf("update: open new binary: %w", err)
	}
	defer source.Close()

	info, err := os.Stat(target)
	if err != nil {
		return fmt.Errorf("update: stat %s: %w", target, err)
	}
	if info.IsDir() {
		return fmt.Errorf("update: %s is a directory", target)
	}

	directory := filepath.Dir(target)
	staging := filepath.Join(directory, "."+filepath.Base(target)+".new")
	if err := copyTo(source, staging, 0o755); err != nil {
		return err
	}

	backup := target + ".old"
	_ = os.Remove(backup)
	if err := os.Rename(target, backup); err != nil {
		// Windows refuses to rename a running image in some setups; the
		// staging file is left behind on purpose so the operator can retry.
		return fmt.Errorf("update: move the current binary aside: %w", err)
	}
	if err := os.Rename(staging, target); err != nil {
		// Put the old binary back so the panel keeps working.
		_ = os.Rename(backup, target)
		return fmt.Errorf("update: install the new binary: %w", err)
	}
	if !i.KeepBackup {
		_ = os.Remove(backup)
	}
	return nil
}

func copyTo(source io.Reader, path string, mode os.FileMode) error {
	handle, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return fmt.Errorf("update: create %s: %w", path, err)
	}
	defer handle.Close()
	if _, err := io.Copy(handle, source); err != nil {
		_ = os.Remove(path)
		return fmt.Errorf("update: write %s: %w", path, err)
	}
	return nil
}

// Download fetches a file into a directory and returns its path.
func Download(ctx context.Context, client *http.Client, url, dir, name string) (string, error) {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Minute}
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("update: build download request: %w", err)
	}
	response, err := client.Do(request)
	if err != nil {
		return "", fmt.Errorf("update: download %s: %w", url, err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("update: download %s failed with status %d", url, response.StatusCode)
	}

	path := filepath.Join(dir, name)
	handle, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return "", fmt.Errorf("update: create %s: %w", path, err)
	}
	defer handle.Close()

	if _, err := io.Copy(handle, response.Body); err != nil {
		return "", fmt.Errorf("update: write %s: %w", path, err)
	}
	return path, nil
}

// DownloadText fetches a small text file.
func DownloadText(ctx context.Context, client *http.Client, url string) (string, error) {
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("update: build request: %w", err)
	}
	response, err := client.Do(request)
	if err != nil {
		return "", fmt.Errorf("update: fetch %s: %w", url, err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("update: fetch %s failed with status %d", url, response.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("update: read %s: %w", url, err)
	}
	return string(raw), nil
}

// sanitizeName keeps a release asset name safe to use as a file name.
func sanitizeName(name string) string {
	name = filepath.Base(strings.TrimSpace(name))
	name = strings.ReplaceAll(name, "..", "")
	if name == "" || name == "." || name == string(filepath.Separator) {
		return "artifact"
	}
	return name
}
