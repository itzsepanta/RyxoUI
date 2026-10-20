package update

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// BinaryName is the name of the panel executable inside an archive.
const BinaryName = "ryxoui"

// AssetName returns the name of the release artefact for a platform.
//
// The convention is ryxoui_<version>_<os>_<arch>.tar.gz, with a matching
// .sha256 file published next to it.
func AssetName(version, goos, goarch string) string {
	return fmt.Sprintf("%s_%s_%s_%s.tar.gz", BinaryName, Normalise(version), goos, goarch)
}

// ChecksumAssetName returns the name of the checksum file of an artefact.
func ChecksumAssetName(asset string) string { return asset + ".sha256" }

// Platform describes the machine the panel runs on.
type Platform struct {
	OS   string `json:"os"`
	Arch string `json:"arch"`
}

// Current returns the platform of the running binary.
func Current() Platform {
	return Platform{OS: runtime.GOOS, Arch: runtime.GOARCH}
}

// String renders the platform.
func (p Platform) String() string { return p.OS + "/" + p.Arch }

// FileChecksum returns the SHA-256 of a file in lower case hex.
func FileChecksum(path string) (string, error) {
	handle, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("update: open %s: %w", path, err)
	}
	defer handle.Close()

	digest := sha256.New()
	if _, err := io.Copy(digest, handle); err != nil {
		return "", fmt.Errorf("update: hash %s: %w", path, err)
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}

// ParseChecksum reads a checksum file. Both the "<hash>  <name>" format
// used by sha256sum and a bare hash are accepted.
func ParseChecksum(raw string) (string, error) {
	fields := strings.Fields(strings.TrimSpace(raw))
	if len(fields) == 0 {
		return "", fmt.Errorf("update: checksum file is empty")
	}
	hash := strings.ToLower(fields[0])
	if len(hash) != 64 {
		return "", fmt.Errorf("update: checksum %q has the wrong length", hash)
	}
	if _, err := hex.DecodeString(hash); err != nil {
		return "", fmt.Errorf("update: checksum %q is not hexadecimal", hash)
	}
	return hash, nil
}

// ExtractArchive extracts a .tar.gz or .zip archive into a directory and
// returns the path of the panel binary inside it.
func ExtractArchive(archivePath, targetDir string) (string, error) {
	switch {
	case strings.HasSuffix(archivePath, ".zip"):
		return extractZip(archivePath, targetDir)
	case strings.HasSuffix(archivePath, ".tar.gz"), strings.HasSuffix(archivePath, ".tgz"):
		return extractTarGz(archivePath, targetDir)
	default:
		return "", fmt.Errorf("update: unsupported archive %q", archivePath)
	}
}

func extractTarGz(archivePath, targetDir string) (string, error) {
	handle, err := os.Open(archivePath)
	if err != nil {
		return "", fmt.Errorf("update: open archive: %w", err)
	}
	defer handle.Close()

	gzipReader, err := gzip.NewReader(handle)
	if err != nil {
		return "", fmt.Errorf("update: read gzip: %w", err)
	}
	defer gzipReader.Close()

	reader := tar.NewReader(gzipReader)
	found := ""
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("update: read archive: %w", err)
		}
		if header.Typeflag != tar.TypeReg {
			continue
		}
		name := filepath.Base(header.Name)
		if name == "" || name == "." || name == ".." {
			continue
		}
		path := filepath.Join(targetDir, name)
		if err := writeFile(path, reader, header.FileInfo().Mode()); err != nil {
			return "", err
		}
		if name == BinaryName || strings.HasPrefix(name, BinaryName+".") {
			found = path
		}
	}
	if found == "" {
		return "", fmt.Errorf("update: archive does not contain %q", BinaryName)
	}
	return found, nil
}

func extractZip(archivePath, targetDir string) (string, error) {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return "", fmt.Errorf("update: open zip: %w", err)
	}
	defer reader.Close()

	found := ""
	for _, entry := range reader.File {
		if entry.FileInfo().IsDir() {
			continue
		}
		name := filepath.Base(entry.Name)
		if name == "" || name == "." || name == ".." {
			continue
		}
		source, err := entry.Open()
		if err != nil {
			return "", fmt.Errorf("update: read zip entry: %w", err)
		}
		path := filepath.Join(targetDir, name)
		err = writeFile(path, source, entry.Mode())
		source.Close()
		if err != nil {
			return "", err
		}
		if name == BinaryName || strings.HasPrefix(name, BinaryName+".") {
			found = path
		}
	}
	if found == "" {
		return "", fmt.Errorf("update: archive does not contain %q", BinaryName)
	}
	return found, nil
}

func writeFile(path string, source io.Reader, mode os.FileMode) error {
	if mode == 0 {
		mode = 0o755
	}
	handle, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode.Perm())
	if err != nil {
		return fmt.Errorf("update: create %s: %w", path, err)
	}
	defer handle.Close()
	if _, err := io.Copy(handle, source); err != nil {
		return fmt.Errorf("update: write %s: %w", path, err)
	}
	return nil
}
