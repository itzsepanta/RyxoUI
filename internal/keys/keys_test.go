package keys

import (
	"encoding/base64"
	"encoding/binary"
	"strings"
	"testing"
)

func TestTheX25519KeyIsMade(t *testing.T) {
	pair, err := GenerateX25519()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	public, err := base64.RawURLEncoding.DecodeString(pair.PublicKey)
	if err != nil {
		t.Fatalf("decode the public key: %v", err)
	}
	if len(public) != 32 {
		t.Fatalf("the public key is %d bytes, want 32", len(public))
	}

	private, err := base64.RawURLEncoding.DecodeString(pair.PrivateKey)
	if err != nil {
		t.Fatalf("decode the private key: %v", err)
	}
	if len(private) != 32 {
		t.Fatalf("the private key is %d bytes, want 32", len(private))
	}

	other, err := GenerateX25519()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if other.PublicKey == pair.PublicKey {
		t.Fatal("two keys must not be the same")
	}
}

func TestTheECHKeyIsMade(t *testing.T) {
	key, err := GenerateECH("example.com")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	public, err := base64.StdEncoding.DecodeString(key.PublicKey)
	if err != nil {
		t.Fatalf("decode the public key: %v", err)
	}
	if len(public) != 65 {
		t.Fatalf("the public key is %d bytes, want 65", len(public))
	}
	if public[0] != 4 {
		t.Fatalf("the public key is not a point: %d", public[0])
	}

	list, err := base64.StdEncoding.DecodeString(key.Config)
	if err != nil {
		t.Fatalf("decode the configuration list: %v", err)
	}
	if len(list) < 4 {
		t.Fatalf("the configuration list is too short: %d", len(list))
	}
	if list[0] != 0xfe || list[1] != 0x0d {
		t.Fatalf("the version of the list is %x", list[:2])
	}

	length := binary.BigEndian.Uint16(list[2:4])
	if int(length) != len(list)-4 {
		t.Fatalf("the length of the list is %d, want %d", length, len(list)-4)
	}

	contents := list[4:]
	if contents[0] == 0 {
		t.Fatal("the id of the configuration must not be zero")
	}
	if kem := binary.BigEndian.Uint16(contents[1:3]); kem != 0x0020 {
		t.Fatalf("the suite of the key exchange is %x, want 0x0020", kem)
	}

	size := binary.BigEndian.Uint16(contents[3:5])
	publicKey := contents[5 : 5+int(size)]
	if len(publicKey) != 65 {
		t.Fatalf("the public key of the list is %d bytes, want 65", len(publicKey))
	}
	if !strings.Contains(key.Config, "") {
		t.Fatal("the configuration must be text")
	}

	rest := contents[5+int(size):]
	nameLength := int(rest[0])
	if nameLength != len("example.com") {
		t.Fatalf("the name is %d bytes, want %d", nameLength, len("example.com"))
	}
	if string(rest[1:1+nameLength]) != "example.com" {
		t.Fatalf("the name is %q", string(rest[1:1+nameLength]))
	}
}

func TestTheECHKeyUsesAName(t *testing.T) {
	key, err := GenerateECH("")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if key.PublicKey == "" || key.Config == "" {
		t.Fatal("the key must hold a public key and a configuration")
	}
}

func TestTheVLESSKeyIsMade(t *testing.T) {
	key, err := GenerateVLESSEncryption()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	client, err := base64.StdEncoding.DecodeString(key.ClientKey)
	if err != nil {
		t.Fatalf("decode the client key: %v", err)
	}
	if len(client) != 1184 {
		t.Fatalf("the key of the client is %d bytes, want 1184", len(client))
	}

	server, err := base64.StdEncoding.DecodeString(key.ServerKey)
	if err != nil {
		t.Fatalf("decode the server key: %v", err)
	}
	if len(server) != 64 {
		t.Fatalf("the key of the server is %d bytes, want 64", len(server))
	}
}
