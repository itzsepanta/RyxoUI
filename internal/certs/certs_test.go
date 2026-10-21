package certs

import (
	"context"
	"crypto/x509"
	"database/sql"
	"encoding/pem"
	"strings"
	"testing"

	"github.com/ryxoui/ryxoui/internal/db"
	_ "modernc.org/sqlite"
)

func testStore(t *testing.T) *Store {
	t.Helper()

	handle, err := sql.Open("sqlite", "file:"+t.Name()+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	handle.SetMaxOpenConns(4)
	t.Cleanup(func() { handle.Close() })

	q := db.New(handle, db.SQLite)
	if _, err := db.NewMigrator(q).Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return NewStore(q)
}

func TestTheAuthorityIsCreatedOnce(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()

	first, err := store.EnsureCA(ctx)
	if err != nil {
		t.Fatalf("ensure the authority: %v", err)
	}
	if first.Kind != KindCA {
		t.Fatalf("kind = %q", first.Kind)
	}
	if !strings.Contains(first.Certificate, "BEGIN CERTIFICATE") {
		t.Fatalf("the authority is not a certificate: %s", first.Certificate)
	}
	if first.PrivateKey == "" {
		t.Fatal("the authority has no key")
	}
	if first.Fingerprint == "" {
		t.Fatal("the authority has no fingerprint")
	}

	second, err := store.EnsureCA(ctx)
	if err != nil {
		t.Fatalf("ensure the authority again: %v", err)
	}
	if second.Certificate != first.Certificate {
		t.Fatal("the authority of the panel must not change")
	}
	if second.Fingerprint != first.Fingerprint {
		t.Fatal("the fingerprint of the authority must not change")
	}
}

func TestTheAuthoritySigns(t *testing.T) {
	store := testStore(t)

	cert, err := store.EnsureCA(context.Background())
	if err != nil {
		t.Fatalf("ensure the authority: %v", err)
	}

	block, _ := pem.Decode([]byte(cert.Certificate))
	if block == nil {
		t.Fatal("the authority is not a certificate")
	}
	parsed, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("parse the authority: %v", err)
	}
	if !parsed.IsCA {
		t.Fatal("the certificate of the panel must be an authority")
	}
	if parsed.Subject.CommonName == "" {
		t.Fatal("the authority needs a name")
	}
}
