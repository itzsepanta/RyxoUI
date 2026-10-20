package update

import (
	"context"
	"fmt"
	"time"

	"github.com/ryxoui/ryxoui/internal/db"
)

// State describes the update status of the panel.
type State struct {
	CurrentVersion string `json:"currentVersion"`
	LatestVersion  string `json:"latestVersion"`
	ReleaseURL     string `json:"releaseUrl"`
	ReleaseNotes   string `json:"releaseNotes"`
	AssetName      string `json:"assetName"`
	Checksum       string `json:"checksum"`
	Status         string `json:"status"`
	Message        string `json:"message"`
	CheckedAt      int64  `json:"checkedAt"`
	AppliedAt      int64  `json:"appliedAt"`
	UpdatedAt      int64  `json:"updatedAt"`
}

// Status values of the update state.
const (
	StatusIdle      = "idle"
	StatusChecking  = "checking"
	StatusAvailable = "available"
	StatusCurrent   = "current"
	StatusDownload  = "downloading"
	StatusInstalled = "installed"
	StatusFailed    = "failed"
)

// UpdateAvailable reports whether a newer release is known.
func (s State) UpdateAvailable() bool {
	if s.LatestVersion == "" || s.CurrentVersion == "" {
		return false
	}
	current, err := Parse(s.CurrentVersion)
	if err != nil {
		return false
	}
	latest, err := Parse(s.LatestVersion)
	if err != nil {
		return false
	}
	return latest.IsNewer(current)
}

// Store persists the update state.
type Store struct {
	db db.Querier
}

// NewStore returns an update store.
func NewStore(q db.Querier) *Store { return &Store{db: q} }

// Load returns the stored state.
func (s *Store) Load(ctx context.Context) (State, error) {
	var state State
	row := s.db.QueryRowContext(ctx,
		`SELECT current_version, latest_version, release_url, release_notes, asset_name, checksum,
		 status, message, checked_at, applied_at, updated_at FROM update_state WHERE id = 'singleton'`)

	err := row.Scan(&state.CurrentVersion, &state.LatestVersion, &state.ReleaseURL, &state.ReleaseNotes,
		&state.AssetName, &state.Checksum, &state.Status, &state.Message,
		&state.CheckedAt, &state.AppliedAt, &state.UpdatedAt)
	if err != nil {
		return State{Status: StatusIdle}, nil
	}
	return state, nil
}

// Save writes the state.
func (s *Store) Save(ctx context.Context, state State) error {
	now := db.Now()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO update_state (id, current_version, latest_version, release_url, release_notes, asset_name,
		 checksum, status, message, checked_at, applied_at, updated_at)
		 VALUES ('singleton', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT (id) DO UPDATE SET current_version = excluded.current_version,
		 latest_version = excluded.latest_version, release_url = excluded.release_url,
		 release_notes = excluded.release_notes, asset_name = excluded.asset_name,
		 checksum = excluded.checksum, status = excluded.status, message = excluded.message,
		 checked_at = excluded.checked_at, applied_at = excluded.applied_at, updated_at = excluded.updated_at`,
		state.CurrentVersion, state.LatestVersion, state.ReleaseURL, state.ReleaseNotes, state.AssetName,
		state.Checksum, state.Status, state.Message, state.CheckedAt, state.AppliedAt, now)
	if err != nil {
		return fmt.Errorf("update: save state: %w", err)
	}
	return nil
}

// Touch records that a check happened.
func (s *Store) Touch(ctx context.Context, state State) error {
	state.CheckedAt = db.Now()
	return s.Save(ctx, state)
}

// AppliedAtNow returns the current time in the schema format.
func AppliedAtNow() int64 { return time.Now().UTC().Unix() }
