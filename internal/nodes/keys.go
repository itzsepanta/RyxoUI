package nodes

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/ryxoui/ryxoui/internal/db"
	"github.com/ryxoui/ryxoui/internal/id"
)

// ErrKeyNotFound is returned when a key does not exist.
var ErrKeyNotFound = errors.New("nodes: key not found")

// SSHKey is a key pair the panel uses to reach a node.
//
// The private key never leaves the panel: it is not part of the JSON the API
// hands out.
type SSHKey struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	PublicKey  string `json:"publicKey"`
	PrivateKey string `json:"-"`
	Comment    string `json:"comment"`
	CreatedAt  int64  `json:"createdAt"`
}

// KeyStore reads and writes the keys of the panel.
type KeyStore struct {
	db db.Querier
}

// NewKeyStore returns a key store.
func NewKeyStore(q db.Querier) *KeyStore { return &KeyStore{db: q} }

// List returns every key ordered by name.
func (s *KeyStore) List(ctx context.Context) ([]SSHKey, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT id, name, public_key, private_key, comment, created_at FROM ssh_keys ORDER BY name")
	if err != nil {
		return nil, fmt.Errorf("nodes: list keys: %w", err)
	}
	defer rows.Close()

	out := []SSHKey{}
	for rows.Next() {
		var key SSHKey
		if err := rows.Scan(&key.ID, &key.Name, &key.PublicKey, &key.PrivateKey, &key.Comment, &key.CreatedAt); err != nil {
			return nil, fmt.Errorf("nodes: scan key: %w", err)
		}
		out = append(out, key)
	}
	return out, rows.Err()
}

// Get reads one key.
func (s *KeyStore) Get(ctx context.Context, keyID string) (SSHKey, error) {
	var key SSHKey
	err := s.db.QueryRowContext(ctx,
		"SELECT id, name, public_key, private_key, comment, created_at FROM ssh_keys WHERE id = ?", keyID).
		Scan(&key.ID, &key.Name, &key.PublicKey, &key.PrivateKey, &key.Comment, &key.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return SSHKey{}, ErrKeyNotFound
	}
	if err != nil {
		return SSHKey{}, fmt.Errorf("nodes: read key: %w", err)
	}
	return key, nil
}

// Create stores a new key.
func (s *KeyStore) Create(ctx context.Context, key SSHKey) (SSHKey, error) {
	if key.ID == "" {
		key.ID = id.New()
	}
	if key.CreatedAt == 0 {
		key.CreatedAt = db.Now()
	}

	_, err := s.db.ExecContext(ctx,
		"INSERT INTO ssh_keys (id, name, public_key, private_key, comment, created_at) VALUES (?, ?, ?, ?, ?, ?)",
		key.ID, key.Name, key.PublicKey, key.PrivateKey, key.Comment, key.CreatedAt)
	if err != nil {
		return SSHKey{}, fmt.Errorf("nodes: insert key: %w", err)
	}
	return key, nil
}

// Delete removes a key.
func (s *KeyStore) Delete(ctx context.Context, keyID string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM ssh_keys WHERE id = ?", keyID)
	if err != nil {
		return fmt.Errorf("nodes: delete key: %w", err)
	}
	return nil
}
