package xray

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ryxoui/ryxoui/internal/engines"
	"github.com/ryxoui/ryxoui/internal/engines/enginetest"
	panelssh "github.com/ryxoui/ryxoui/internal/ssh"
	"github.com/ryxoui/ryxoui/internal/ssh/sshtest"
	"github.com/ryxoui/ryxoui/internal/update"
)

func TestTheEngineFollowsTheContract(t *testing.T) {
	enginetest.Conformance(t, New(), engines.Config{
		Entrypoints: []engines.Entrypoint{
			{
				Tag:     "inbound-443",
				Type:    "vless",
				Enabled: true,
				Config:  map[string]any{"port": 443, "clients": []any{}},
			},
		},
	})
}

func TestRenderBuildsTheConfiguration(t *testing.T) {
	engine := New()

	config := engines.Config{
		LogLevel: "debug",
		Entrypoints: []engines.Entrypoint{
			{
				Tag:     "inbound-443",
				Type:    "vless",
				Enabled: true,
				Config: map[string]any{
					"port": 443,
					"clients": []any{
						map[string]any{"id": "the-old-client"},
					},
				},
			},
			{
				Tag:     "inbound-off",
				Enabled: false,
				Config:  map[string]any{"port": 8443},
			},
		},
		Egresses: []engines.Egress{
			{Tag: "out-1", Enabled: true, Config: map[string]any{"protocol": "freedom"}},
		},
		Endpoints: []engines.Endpoint{
			{Tag: "direct", Enabled: true, Config: map[string]any{
				"domain": []any{"geosite:private"},
			}},
		},
		Users: []engines.User{
			{ID: "u1", UUID: "11111111-1111-1111-1111-111111111111", Email: "one@example", Enabled: true},
			{ID: "u2", UUID: "22222222-2222-2222-2222-222222222222", Email: "two@example", Enabled: true},
			{ID: "u3", UUID: "33333333-3333-3333-3333-333333333333", Email: "three@example", Enabled: false},
		},
	}

	content, err := engine.Render(context.Background(), engines.Target{}, config)
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	var document map[string]any
	if err := json.Unmarshal(content, &document); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if log, _ := document["log"].(map[string]any); log["loglevel"] != "debug" {
		t.Fatalf("log = %v", document["log"])
	}

	inbounds, _ := document["inbounds"].([]any)
	if len(inbounds) != 1 {
		t.Fatalf("inbounds = %d, want 1", len(inbounds))
	}
	inbound, _ := inbounds[0].(map[string]any)
	if inbound["tag"] != "inbound-443" {
		t.Fatalf("tag = %v", inbound["tag"])
	}
	if inbound["listen"] != "0.0.0.0" {
		t.Fatalf("listen = %v", inbound["listen"])
	}
	if inbound["protocol"] != "vless" {
		t.Fatalf("protocol = %v", inbound["protocol"])
	}

	clients, _ := inbound["clients"].([]any)
	if len(clients) != 2 {
		t.Fatalf("clients = %d, want 2", len(clients))
	}
	first, _ := clients[0].(map[string]any)
	if first["id"] != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("client = %v", first)
	}

	outbounds, _ := document["outbounds"].([]any)
	if len(outbounds) != 1 {
		t.Fatalf("outbounds = %d, want 1", len(outbounds))
	}

	routing, _ := document["routing"].(map[string]any)
	rules, _ := routing["rules"].([]any)
	if len(rules) != 1 {
		t.Fatalf("rules = %d, want 1", len(rules))
	}
	rule, _ := rules[0].(map[string]any)
	if rule["outboundTag"] != "direct" {
		t.Fatalf("rule = %v", rule)
	}
}

func TestRenderKeepsTheSettingsOfTheEntry(t *testing.T) {
	engine := New()

	config := engines.Config{
		Entrypoints: []engines.Entrypoint{
			{
				Tag:     "inbound-443",
				Type:    "vless",
				Enabled: true,
				Config: map[string]any{
					"port":           443,
					"security":       "reality",
					"flow":           "xtls-rprx-vision",
					"streamSettings": map[string]any{"network": "tcp"},
				},
			},
		},
	}

	content, err := engine.Render(context.Background(), engines.Target{}, config)
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	var document map[string]any
	if err := json.Unmarshal(content, &document); err != nil {
		t.Fatalf("decode: %v", err)
	}
	inbound, _ := document["inbounds"].([]any)[0].(map[string]any)
	if inbound["security"] != "reality" {
		t.Fatalf("the settings of the entry were lost: %v", inbound)
	}
	if _, ok := inbound["streamSettings"]; !ok {
		t.Fatalf("the settings of the entry were lost: %v", inbound)
	}
}

func TestRenderAddsAWayOut(t *testing.T) {
	engine := New()

	content, err := engine.Render(context.Background(), engines.Target{}, engines.Config{})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	var document map[string]any
	if err := json.Unmarshal(content, &document); err != nil {
		t.Fatalf("decode: %v", err)
	}
	outbounds, _ := document["outbounds"].([]any)
	if len(outbounds) != 1 {
		t.Fatalf("outbounds = %d, want 1", len(outbounds))
	}
	outbound, _ := outbounds[0].(map[string]any)
	if outbound["protocol"] != "freedom" {
		t.Fatalf("outbound = %v", outbound)
	}
}

func TestClientsAreReadBack(t *testing.T) {
	content := []byte(`{"inbounds":[{"tag":"in","clients":[
		{"id":"11111111-1111-1111-1111-111111111111","email":"one@example","flow":"xtls-rprx-vision"},
		{"id":"22222222-2222-2222-2222-222222222222","email":"two@example","password":"secret"}
	]}]}`)

	users, err := clientsOf(content)
	if err != nil {
		t.Fatalf("read the clients: %v", err)
	}
	if len(users) != 2 {
		t.Fatalf("users = %d, want 2", len(users))
	}
	if users[0].UUID != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("user = %+v", users[0])
	}
	if users[0].Flow != "xtls-rprx-vision" {
		t.Fatalf("user = %+v", users[0])
	}
	if users[1].Password != "secret" {
		t.Fatalf("user = %+v", users[1])
	}
}

func TestAssetNameFollowsThePlatform(t *testing.T) {
	cases := map[string]struct {
		os   string
		arch string
		want string
	}{
		"linux amd64": {os: "linux", arch: "amd64", want: "Xray-linux-64.zip"},
		"linux arm64": {os: "linux", arch: "arm64", want: "Xray-linux-arm64-v8a.zip"},
		"windows":     {os: "windows", arch: "amd64", want: ""},
		"unknown":     {os: "linux", arch: "riscv", want: ""},
	}

	for name, item := range cases {
		got := assetName(engines.Target{OS: item.os, Arch: item.arch})
		if got != item.want {
			t.Fatalf("%s: asset = %q, want %q", name, got, item.want)
		}
	}
}

func TestVersionsAreRead(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"tag_name":"v1.8.4","draft":false,"prerelease":false,"assets":[
			{"name":"Xray-linux-64.zip","browser_download_url":"https://example.invalid/xray.zip","size":10},
			{"name":"Xray-linux-64.zip.dgst","browser_download_url":"https://example.invalid/dgst","size":10}
		]}]`))
	}))
	defer server.Close()

	engine := New()
	engine.Releases = &update.GitHubClient{Repo: Repository, BaseURL: server.URL}

	releases, err := engine.Versions(context.Background())
	if err != nil {
		t.Fatalf("versions: %v", err)
	}
	if len(releases) != 1 {
		t.Fatalf("releases = %d, want 1", len(releases))
	}
	if releases[0].Version != "1.8.4" {
		t.Fatalf("version = %q", releases[0].Version)
	}
	if releases[0].Asset != "Xray-linux-64.zip" {
		t.Fatalf("asset = %q", releases[0].Asset)
	}
}

func TestInstallPutsTheEngineOnTheNode(t *testing.T) {
	archive := zipArchive(t, "xray", []byte("the binary of the engine"))

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/downloads/xray.zip") {
			w.Header().Set("Content-Type", "application/zip")
			_, _ = w.Write(archive)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `[{"tag_name":"v1.8.4","draft":false,"prerelease":false,"assets":[
			{"name":"Xray-linux-64.zip","browser_download_url":"%s/downloads/xray.zip","size":%d}
		]}]`, server.URL, len(archive))
	}))
	defer server.Close()

	node, err := sshtest.New("the answer of the node")
	if err != nil {
		t.Fatalf("start the node: %v", err)
	}
	defer node.Close()

	engine := New()
	engine.Releases = &update.GitHubClient{Repo: Repository, BaseURL: server.URL}
	engine.HTTP = server.Client()

	target := engines.Target{
		NodeID:     "node-1",
		Host:       node.Host,
		Port:       node.Port,
		User:       "root",
		OS:         "linux",
		Arch:       "amd64",
		PrivateKey: testKey(t),
	}

	if err := engine.Install(context.Background(), target, ""); err != nil {
		t.Fatalf("install: %v", err)
	}
	if node.CommandCount() == 0 {
		t.Fatal("the engine did not talk to the node")
	}
}

func TestHealthReadsTheStateOfTheEngine(t *testing.T) {
	node, err := sshtest.New("active\n")
	if err != nil {
		t.Fatalf("start the node: %v", err)
	}
	defer node.Close()

	engine := New()
	health, err := engine.Health(context.Background(), engines.Target{
		Host: node.Host, Port: node.Port, User: "root", PrivateKey: testKey(t),
	})
	if err != nil {
		t.Fatalf("health: %v", err)
	}
	if !health.Running {
		t.Fatal("the engine must be reported as running")
	}
	if health.Version != "active" {
		t.Fatalf("version = %q", health.Version)
	}
}

// testKey returns a key pair the fake node accepts.
func testKey(t *testing.T) string {
	t.Helper()
	pair, err := panelssh.GenerateKeyPair("test")
	if err != nil {
		t.Fatalf("generate the key: %v", err)
	}
	return pair.PrivateKey
}

// zipArchive builds an archive with one file in it.
func zipArchive(t *testing.T, name string, content []byte) []byte {
	t.Helper()

	buffer := &bytes.Buffer{}
	writer := zip.NewWriter(buffer)
	file, err := writer.Create(name)
	if err != nil {
		t.Fatalf("create the archive: %v", err)
	}
	if _, err := file.Write(content); err != nil {
		t.Fatalf("write the archive: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close the archive: %v", err)
	}
	return buffer.Bytes()
}
