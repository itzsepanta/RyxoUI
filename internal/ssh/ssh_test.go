package ssh

import (
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
)

func TestGenerateKeyPair(t *testing.T) {
	pair, err := GenerateKeyPair("ryxoui")
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}
	if !strings.HasPrefix(pair.PublicKey, "ssh-ed25519 ") {
		t.Fatalf("public key = %q", pair.PublicKey)
	}
	if !strings.Contains(pair.PublicKey, "ryxoui") {
		t.Fatalf("public key does not carry the comment: %q", pair.PublicKey)
	}

	signer, err := ssh.ParsePrivateKey([]byte(pair.PrivateKey))
	if err != nil {
		t.Fatalf("ParsePrivateKey: %v", err)
	}
	public := signer.PublicKey()
	marshalled := strings.Fields(string(ssh.MarshalAuthorizedKey(public)))
	if got := strings.Fields(pair.PublicKey)[1]; got != marshalled[1] {
		t.Fatal("the public key does not match the private key")
	}
}

func TestKnownHostRoundTrip(t *testing.T) {
	pair, err := GenerateKeyPair("node")
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}
	signer, _ := ssh.ParsePrivateKey([]byte(pair.PrivateKey))
	public := signer.PublicKey()

	line := knownHostsLine("10.0.0.5", 22, public)
	if !strings.HasPrefix(line, "10.0.0.5 ") {
		t.Fatalf("known hosts line = %q", line)
	}

	nonStandard := knownHostsLine("10.0.0.5", 2222, public)
	if !strings.HasPrefix(nonStandard, "[10.0.0.5]:2222 ") {
		t.Fatalf("known hosts line = %q", nonStandard)
	}
}

func TestParseKnownHost(t *testing.T) {
	pair, _ := GenerateKeyPair("node")
	signer, _ := ssh.ParsePrivateKey([]byte(pair.PrivateKey))
	public := signer.PublicKey()

	key, err := parseKnownHost(knownHostsLine("10.0.0.5", 22, public))
	if err != nil {
		t.Fatalf("parseKnownHost: %v", err)
	}
	if string(key.Marshal()) != string(public.Marshal()) {
		t.Fatal("the parsed key does not match")
	}

	if _, err := parseKnownHost("not a known hosts line"); err == nil {
		t.Fatal("expected an error for a malformed line")
	}
}

func TestNewClientValidatesTheConfig(t *testing.T) {
	if _, err := NewClient(Config{}); err == nil {
		t.Fatal("expected an error for an empty host")
	}
	client, err := NewClient(Config{Host: "10.0.0.5"})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if client.config.Port != 22 || client.config.User != "root" {
		t.Fatalf("unexpected defaults: %+v", client.config)
	}
}
