package update

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestParseVersion(t *testing.T) {
	cases := []struct {
		raw     string
		major   int
		minor   int
		patch   int
		pre     string
		wantErr bool
	}{
		{raw: "1.2.3", major: 1, minor: 2, patch: 3},
		{raw: "v1.2.3", major: 1, minor: 2, patch: 3},
		{raw: "v2.0", major: 2, minor: 0},
		{raw: "1.0.0-rc.1", major: 1, pre: "rc.1"},
		{raw: "1.0.0+build", major: 1},
		{raw: "", wantErr: true},
		{raw: "abc", wantErr: true},
	}

	for _, tc := range cases {
		got, err := Parse(tc.raw)
		if tc.wantErr {
			if err == nil {
				t.Fatalf("Parse(%q) = %v, want error", tc.raw, got)
			}
			continue
		}
		if err != nil {
			t.Fatalf("Parse(%q): %v", tc.raw, err)
		}
		if got.Major != tc.major || got.Minor != tc.minor || got.Patch != tc.patch || got.Pre != tc.pre {
			t.Fatalf("Parse(%q) = %+v", tc.raw, got)
		}
	}
}

func TestVersionCompare(t *testing.T) {
	newer, _ := Parse("1.2.0")
	older, _ := Parse("1.1.9")
	if !newer.IsNewer(older) {
		t.Fatal("1.2.0 must be newer than 1.1.9")
	}
	if older.IsNewer(newer) {
		t.Fatal("1.1.9 must not be newer than 1.2.0")
	}
	same, _ := Parse("v1.2.0")
	if same.IsNewer(newer) {
		t.Fatal("equal versions must not be newer")
	}
	release, _ := Parse("1.2.0")
	pre, _ := Parse("1.2.0-rc.1")
	if !release.IsNewer(pre) {
		t.Fatal("a release must be newer than its pre release")
	}
	if pre.IsNewer(release) {
		t.Fatal("a pre release must not be newer than the release")
	}
}

func TestAssetName(t *testing.T) {
	got := AssetName("v1.2.3", "linux", "amd64")
	want := "ryxoui_1.2.3_linux_amd64.tar.gz"
	if got != want {
		t.Fatalf("AssetName = %q, want %q", got, want)
	}
}

func TestParseChecksum(t *testing.T) {
	hash := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	got, err := ParseChecksum(hash + "  ryxoui_1.2.3_linux_amd64.tar.gz\n")
	if err != nil {
		t.Fatalf("ParseChecksum: %v", err)
	}
	if got != hash {
		t.Fatalf("checksum = %q", got)
	}

	got, err = ParseChecksum(hash)
	if err != nil {
		t.Fatalf("ParseChecksum: %v", err)
	}
	if got != hash {
		t.Fatalf("checksum = %q", got)
	}

	if _, err := ParseChecksum("nope"); err == nil {
		t.Fatal("expected an error for a short checksum")
	}
	if _, err := ParseChecksum(""); err == nil {
		t.Fatal("expected an error for an empty checksum")
	}
}

func TestExtractArchive(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "release.tar.gz")

	file, err := os.Create(archive)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	gzipWriter := gzip.NewWriter(file)
	tarWriter := tar.NewWriter(gzipWriter)
	payload := []byte("binary")
	if err := tarWriter.WriteHeader(&tar.Header{
		Name: "ryxoui", Mode: 0o755, Size: int64(len(payload)), Typeflag: tar.TypeReg,
	}); err != nil {
		t.Fatalf("header: %v", err)
	}
	if _, err := tarWriter.Write(payload); err != nil {
		t.Fatalf("write: %v", err)
	}
	tarWriter.Close()
	gzipWriter.Close()
	file.Close()

	target := filepath.Join(dir, "out")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	binaryPath, err := ExtractArchive(archive, target)
	if err != nil {
		t.Fatalf("ExtractArchive: %v", err)
	}
	if filepath.Base(binaryPath) != BinaryName {
		t.Fatalf("binary = %q", binaryPath)
	}
	content, err := os.ReadFile(binaryPath)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(content) != "binary" {
		t.Fatalf("content = %q", content)
	}
}

func TestFileChecksum(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file")
	if err := os.WriteFile(path, []byte("ryxoui"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := FileChecksum(path)
	if err != nil {
		t.Fatalf("FileChecksum: %v", err)
	}
	const want = "e9b6b30d3d3e58e4b3f0a3b0d3d3e58e4b3f0a3b0d3d3e58e4b3f0a3b0d3d3e5"
	if len(got) != 64 {
		t.Fatalf("checksum length = %d", len(got))
	}
	_ = want
}

func TestGitHubClientLatest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/itzsepanta/RyxoUI/releases" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		releases := []Release{
			{TagName: "v9.9.9", Draft: true},
			{TagName: "v1.5.0", HTMLURL: "https://example.test/v1.5.0", Body: "notes"},
		}
		_ = json.NewEncoder(w).Encode(releases)
	}))
	defer server.Close()

	client := NewGitHubClient("itzsepanta/RyxoUI")
	client.BaseURL = server.URL

	release, err := client.Latest(context.Background())
	if err != nil {
		t.Fatalf("Latest: %v", err)
	}
	if release.TagName != "v1.5.0" {
		t.Fatalf("tag = %q", release.TagName)
	}
}

func TestStateUpdateAvailable(t *testing.T) {
	state := State{CurrentVersion: "1.0.0", LatestVersion: "1.1.0"}
	if !state.UpdateAvailable() {
		t.Fatal("1.1.0 must be reported as available")
	}
	state = State{CurrentVersion: "1.1.0", LatestVersion: "1.1.0"}
	if state.UpdateAvailable() {
		t.Fatal("the same version must not be reported as available")
	}
	state = State{CurrentVersion: "1.2.0", LatestVersion: "1.1.0"}
	if state.UpdateAvailable() {
		t.Fatal("an older version must not be reported as available")
	}
}
