package api

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"github.com/ryxoui/ryxoui/internal/audit"
	"github.com/ryxoui/ryxoui/internal/authz"
	"github.com/ryxoui/ryxoui/internal/db"
)

// handleListRoles returns every role.
func (s *Server) handleListRoles(w http.ResponseWriter, r *http.Request) {
	roles, err := s.roles.List(r.Context())
	if err != nil {
		handleError(w, r, s.log, err)
		return
	}
	writeList(w, roles, listMeta{Count: len(roles), Limit: len(roles)})
}

// handleGetRole reads one role.
func (s *Server) handleGetRole(w http.ResponseWriter, r *http.Request) {
	role, err := s.roles.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		if errors.Is(err, authz.ErrNotFound) {
			handleError(w, r, s.log, notFound("role not found"))
			return
		}
		handleError(w, r, s.log, err)
		return
	}
	writeJSON(w, http.StatusOK, role)
}

// roleRequest creates or updates a role.
type roleRequest struct {
	Name           string   `json:"name"`
	Base           string   `json:"base"`
	Description    string   `json:"description"`
	Scopes         []string `json:"scopes"`
	UserLimit      int      `json:"userLimit"`
	MaxDeviceLimit int      `json:"maxDeviceLimit"`
	MaxDuration    int      `json:"maxDuration"`
}

// handleCreateRole creates a custom role.
func (s *Server) handleCreateRole(w http.ResponseWriter, r *http.Request) {
	var body roleRequest
	if err := decodeJSON(r, &body); err != nil {
		handleError(w, r, s.log, err)
		return
	}
	body.Name = strings.TrimSpace(body.Name)
	if body.Name == "" {
		handleError(w, r, s.log, badRequest("name is required"))
		return
	}
	if _, err := authz.ValidateScopes(body.Scopes); err != nil {
		handleError(w, r, s.log, badRequest("%s", err.Error()))
		return
	}

	var created authz.Role
	err := s.db.InTx(r.Context(), func(tx *db.Tx) error {
		roles := authz.NewStore(tx)
		role, err := roles.Create(r.Context(), authz.Role{
			Name:           body.Name,
			Base:           body.Base,
			Description:    body.Description,
			Scopes:         body.Scopes,
			UserLimit:      body.UserLimit,
			MaxDeviceLimit: body.MaxDeviceLimit,
			MaxDuration:    body.MaxDuration,
		})
		if err != nil {
			return err
		}
		created = role

		if err := audit.NewStore(tx).Record(r.Context(), tx, audit.Entry{
			ActorID:   principal(r.Context()).Operator.ID,
			ActorName: principal(r.Context()).Operator.Username,
			Action:    "role.create",
			Entity:    "role",
			EntityID:  role.ID,
			After:     role,
			IP:        clientIPAddress(r),
			UserAgent: userAgent(r),
			RequestID: requestID(r.Context()),
		}); err != nil {
			return err
		}
		return s.outbox.Publish(r.Context(), tx, "role.created", map[string]any{
			"roleId": role.ID,
			"name":   role.Name,
		})
	})
	if err != nil {
		handleError(w, r, s.log, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

// handleUpdateRole updates a custom role.
func (s *Server) handleUpdateRole(w http.ResponseWriter, r *http.Request) {
	roleID := r.PathValue("id")
	current, err := s.roles.Get(r.Context(), roleID)
	if err != nil {
		if errors.Is(err, authz.ErrNotFound) {
			handleError(w, r, s.log, notFound("role not found"))
			return
		}
		handleError(w, r, s.log, err)
		return
	}

	var body roleRequest
	if err := decodeJSON(r, &body); err != nil {
		handleError(w, r, s.log, err)
		return
	}
	if _, err := authz.ValidateScopes(body.Scopes); err != nil {
		handleError(w, r, s.log, badRequest("%s", err.Error()))
		return
	}
	if body.Name != "" {
		current.Name = strings.TrimSpace(body.Name)
	}
	current.Base = body.Base
	current.Description = body.Description
	current.Scopes = body.Scopes
	current.UserLimit = body.UserLimit
	current.MaxDeviceLimit = body.MaxDeviceLimit
	current.MaxDuration = body.MaxDuration

	err = s.db.InTx(r.Context(), func(tx *db.Tx) error {
		roles := authz.NewStore(tx)
		updated, err := roles.Update(r.Context(), current)
		if err != nil {
			return err
		}
		if err := audit.NewStore(tx).Record(r.Context(), tx, audit.Entry{
			ActorID:   principal(r.Context()).Operator.ID,
			ActorName: principal(r.Context()).Operator.Username,
			Action:    "role.update",
			Entity:    "role",
			EntityID:  roleID,
			Before:    current,
			After:     updated,
			IP:        clientIPAddress(r),
			UserAgent: userAgent(r),
			RequestID: requestID(r.Context()),
		}); err != nil {
			return err
		}
		return s.outbox.Publish(r.Context(), tx, "role.updated", map[string]any{
			"roleId": roleID,
			"name":   updated.Name,
		})
	})
	if err != nil {
		handleError(w, r, s.log, err)
		return
	}
	updated, err := s.roles.Get(r.Context(), roleID)
	if err != nil {
		handleError(w, r, s.log, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

// handleDeleteRole removes a custom role.
func (s *Server) handleDeleteRole(w http.ResponseWriter, r *http.Request) {
	roleID := r.PathValue("id")
	current, err := s.roles.Get(r.Context(), roleID)
	if err != nil {
		if errors.Is(err, authz.ErrNotFound) {
			handleError(w, r, s.log, notFound("role not found"))
			return
		}
		handleError(w, r, s.log, err)
		return
	}

	err = s.db.InTx(r.Context(), func(tx *db.Tx) error {
		roles := authz.NewStore(tx)
		if err := roles.Delete(r.Context(), roleID); err != nil {
			return err
		}
		if err := audit.NewStore(tx).Record(r.Context(), tx, audit.Entry{
			ActorID:   principal(r.Context()).Operator.ID,
			ActorName: principal(r.Context()).Operator.Username,
			Action:    "role.delete",
			Entity:    "role",
			EntityID:  roleID,
			Before:    current,
			IP:        clientIPAddress(r),
			UserAgent: userAgent(r),
			RequestID: requestID(r.Context()),
		}); err != nil {
			return err
		}
		return s.outbox.Publish(r.Context(), tx, "role.deleted", map[string]any{
			"roleId": roleID,
			"name":   current.Name,
		})
	})
	if err != nil {
		handleError(w, r, s.log, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// handleListScopes returns the scope catalogue.
func (s *Server) handleListScopes(w http.ResponseWriter, r *http.Request) {
	scopes := authz.Scopes()
	writeList(w, scopes, listMeta{Count: len(scopes), Limit: len(scopes)})
}

// handleListSettings returns every setting.
func (s *Server) handleListSettings(w http.ResponseWriter, r *http.Request) {
	rows, err := s.cfg.DB.QueryContext(r.Context(), "SELECT key, value FROM settings ORDER BY key")
	if err != nil {
		handleError(w, r, s.log, err)
		return
	}
	defer rows.Close()

	type setting struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}
	var out []setting
	for rows.Next() {
		var item setting
		if err := rows.Scan(&item.Key, &item.Value); err != nil {
			handleError(w, r, s.log, err)
			return
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		handleError(w, r, s.log, err)
		return
	}
	writeList(w, out, listMeta{Count: len(out), Limit: len(out)})
}

// settingRequest changes a setting.
type settingRequest struct {
	Value string `json:"value"`
}

// handleUpdateSetting changes a setting.
func (s *Server) handleUpdateSetting(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	if key == "" {
		handleError(w, r, s.log, badRequest("key is required"))
		return
	}
	var body settingRequest
	if err := decodeJSON(r, &body); err != nil {
		handleError(w, r, s.log, err)
		return
	}

	var previous string
	err := s.cfg.DB.QueryRowContext(r.Context(), "SELECT value FROM settings WHERE key = ?", key).Scan(&previous)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		handleError(w, r, s.log, err)
		return
	}

	err = s.db.InTx(r.Context(), func(tx *db.Tx) error {
		if _, err := tx.ExecContext(r.Context(),
			`INSERT INTO settings (key, value, updated_at) VALUES (?, ?, ?)
			 ON CONFLICT (key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
			key, body.Value, db.Now()); err != nil {
			return err
		}
		if err := audit.NewStore(tx).Record(r.Context(), tx, audit.Entry{
			ActorID:   principal(r.Context()).Operator.ID,
			ActorName: principal(r.Context()).Operator.Username,
			Action:    "settings.update",
			Entity:    "settings",
			EntityID:  key,
			Before:    map[string]any{"value": previous},
			After:     map[string]any{"value": body.Value},
			IP:        clientIPAddress(r),
			UserAgent: userAgent(r),
			RequestID: requestID(r.Context()),
		}); err != nil {
			return err
		}
		return s.outbox.Publish(r.Context(), tx, "settings.updated", map[string]any{"key": key})
	})
	if err != nil {
		handleError(w, r, s.log, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"key": key, "value": body.Value})
}

// handleListTokens returns the API tokens of the signed in operator.
func (s *Server) handleListTokens(w http.ResponseWriter, r *http.Request) {
	p := principal(r.Context())
	tokens, err := s.tokens.List(r.Context(), p.Operator.ID)
	if err != nil {
		handleError(w, r, s.log, err)
		return
	}
	writeList(w, tokens, listMeta{Count: len(tokens), Limit: len(tokens)})
}

// tokenRequest creates an API token.
type tokenRequest struct {
	Name      string   `json:"name"`
	Scopes    []string `json:"scopes"`
	ExpiresAt int64    `json:"expiresAt"`
	RateLimit int      `json:"rateLimit"`
}

// handleCreateToken creates an API token.
func (s *Server) handleCreateToken(w http.ResponseWriter, r *http.Request) {
	p := principal(r.Context())
	var body tokenRequest
	if err := decodeJSON(r, &body); err != nil {
		handleError(w, r, s.log, err)
		return
	}
	if strings.TrimSpace(body.Name) == "" {
		handleError(w, r, s.log, badRequest("name is required"))
		return
	}

	scopes := body.Scopes
	if len(scopes) == 0 {
		scopes = p.Scopes
	}
	validated, err := authz.ValidateScopes(scopes)
	if err != nil {
		handleError(w, r, s.log, badRequest("%s", err.Error()))
		return
	}

	token, err := s.tokens.Create(r.Context(), p.Operator.ID, body.Name, validated, body.ExpiresAt, body.RateLimit)
	if err != nil {
		handleError(w, r, s.log, err)
		return
	}
	writeJSON(w, http.StatusCreated, token)
}

// handleDeleteToken revokes an API token.
func (s *Server) handleDeleteToken(w http.ResponseWriter, r *http.Request) {
	p := principal(r.Context())
	tokenID := r.PathValue("id")

	tokens, err := s.tokens.List(r.Context(), p.Operator.ID)
	if err != nil {
		handleError(w, r, s.log, err)
		return
	}
	found := false
	for _, token := range tokens {
		if token.ID == tokenID {
			found = true
			break
		}
	}
	if !found {
		handleError(w, r, s.log, notFound("token not found"))
		return
	}
	if err := s.tokens.Delete(r.Context(), tokenID); err != nil {
		handleError(w, r, s.log, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// handleListAudit returns the audit log.
func (s *Server) handleListAudit(w http.ResponseWriter, r *http.Request) {
	filter := audit.Filter{
		ActorID:  r.URL.Query().Get("actorId"),
		Entity:   r.URL.Query().Get("entity"),
		EntityID: r.URL.Query().Get("entityId"),
		Action:   r.URL.Query().Get("action"),
		Since:    queryInt64(r, "since", 0),
		Until:    queryInt64(r, "until", 0),
		Limit:    queryInt(r, "limit", 100),
		Offset:   queryInt(r, "offset", 0),
	}
	entries, total, err := s.audit.List(r.Context(), filter)
	if err != nil {
		handleError(w, r, s.log, err)
		return
	}
	writeList(w, entries, listMeta{Count: len(entries), Limit: filter.Limit, Offset: filter.Offset, Total: total})
}
