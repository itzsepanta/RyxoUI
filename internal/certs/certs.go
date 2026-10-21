// Package certs keeps the certificates of the panel.
//
// The panel signs the certificates of the nodes with a certificate authority
// of its own. The authority is created once, when the panel starts for the
// first time, and stays the same for the life of the panel, so a node that
// was set up once keeps working.
package certs

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"database/sql"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/ryxoui/ryxoui/internal/db"
	"github.com/ryxoui/ryxoui/internal/id"
)

// The kinds of certificate the panel holds.
const (
	KindCA = "ca"
)

// ErrNotFound is returned when a certificate does not exist.
var ErrNotFound = errors.New("certs: certificate not found")

// Certificate is a certificate of the panel.
type Certificate struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Kind        string `json:"kind"`
	Certificate string `json:"certificate"`
	PrivateKey  string `json:"-"`
	Fingerprint string `json:"fingerprint"`
	CreatedAt   int64  `json:"createdAt"`
	UpdatedAt   int64  `json:"updatedAt"`
}

// Store reads and writes the certificates of the panel.
type Store struct {
	db db.Querier
}

// NewStore returns a certificate store.
func NewStore(q db.Querier) *Store { return &Store{db: q} }

// Get reads a certificate by name.
func (s *Store) Get(ctx context.Context, name string) (Certificate, error) {
	var cert Certificate
	err := s.db.QueryRowContext(ctx,
		"SELECT id, name, kind, certificate, private_key, fingerprint, created_at, updated_at FROM panel_certificates WHERE name = ?", name).
		Scan(&cert.ID, &cert.Name, &cert.Kind, &cert.Certificate, &cert.PrivateKey, &cert.Fingerprint, &cert.CreatedAt, &cert.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Certificate{}, ErrNotFound
	}
	if err != nil {
		return Certificate{}, fmt.Errorf("certs: read the certificate: %w", err)
	}
	return cert, nil
}

// EnsureCA returns the certificate authority of the panel and creates it when
// the panel has none yet.
func (s *Store) EnsureCA(ctx context.Context) (Certificate, error) {
	cert, err := s.Get(ctx, "panel-ca")
	if err == nil {
		return cert, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return Certificate{}, err
	}

	cert, err = createCA()
	if err != nil {
		return Certificate{}, err
	}

	now := db.Now()
	cert.ID = id.New()
	cert.CreatedAt = now
	cert.UpdatedAt = now

	_, err = s.db.ExecContext(ctx,
		"INSERT INTO panel_certificates (id, name, kind, certificate, private_key, fingerprint, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		cert.ID, cert.Name, cert.Kind, cert.Certificate, cert.PrivateKey, cert.Fingerprint, cert.CreatedAt, cert.UpdatedAt)
	if err != nil {
		return Certificate{}, fmt.Errorf("certs: store the authority: %w", err)
	}
	return cert, nil
}

// createCA makes a self signed certificate authority.
func createCA() (Certificate, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return Certificate{}, fmt.Errorf("certs: generate the key: %w", err)
	}

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return Certificate{}, fmt.Errorf("certs: generate the serial: %w", err)
	}

	template := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: "RyxoUI Panel Authority", Organization: []string{"RyxoUI"}},
		NotBefore:             time.Now().UTC().Add(-time.Hour),
		NotAfter:              time.Now().UTC().AddDate(10, 0, 0),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            1,
	}

	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return Certificate{}, fmt.Errorf("certs: sign the authority: %w", err)
	}

	keyBytes, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return Certificate{}, fmt.Errorf("certs: marshal the key: %w", err)
	}

	certificatePEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes})

	return Certificate{
		Name:        "panel-ca",
		Kind:        KindCA,
		Certificate: string(certificatePEM),
		PrivateKey:  string(keyPEM),
		Fingerprint: Fingerprint(der),
	}, nil
}

// Fingerprint is the digest of a certificate.
func Fingerprint(der []byte) string {
	sum := sha256.Sum256(der)
	return hex.EncodeToString(sum[:])
}
