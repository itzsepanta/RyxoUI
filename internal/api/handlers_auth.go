package api

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/ryxoui/ryxoui/internal/audit"
	"github.com/ryxoui/ryxoui/internal/auth"
	"github.com/ryxoui/ryxoui/internal/authz"
	"github.com/ryxoui/ryxoui/internal/db"
	"github.com/ryxoui/ryxoui/internal/events"
)

// loginRequest is the body of a sign in.
type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Remember bool   `json:"remember"`
}

// loginResponse is returned after a successful sign in.
type loginResponse struct {
	Token     string       `json:"token,omitempty"`
	Operator  operatorView `json:"operator"`
	Role      authz.Role   `json:"role"`
	Scopes    []string     `json:"scopes"`
	NeedsMFA  bool         `json:"needsMfa"`
	ExpiresAt int64        `json:"expiresAt"`
}

// operatorView is the public representation for an operator.
type operatorView struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	RoleID      string `json:"roleId"`
	RoleName    string `json:"roleName"`
	IsOwner     bool   `json:"isOwner"`
	Status      string `json:"status"`
	LastLoginAt int64  `json:"lastLoginAt"`
	LastLoginIP string `json:"lastLoginIp"`
	TOTPEnabled bool   `json:"totpEnabled"`
	CreatedAt   int64  `json:"createdAt"`
	UpdatedAt   int64  `json:"updatedAt"`
}

// viewOperator renders an operator for the API. The role lookup goes through
// the given querier so the function can be used inside a transaction.
func (s *Server) viewOperator(ctx context.Context, q db.Querier, op auth.Operator) operatorView {
	view := operatorView{
		ID:          op.ID,
		Username:    op.Username,
		RoleID:      op.RoleID,
		IsOwner:     op.IsOwner,
		Status:      op.Status,
		LastLoginAt: op.LastLoginAt,
		LastLoginIP: op.LastLoginIP,
		TOTPEnabled: op.TOTPEnabled,
		CreatedAt:   op.CreatedAt,
		UpdatedAt:   op.UpdatedAt,
	}
	if role, err := authz.NewStore(q).Get(ctx, op.RoleID); err == nil {
		view.RoleName = role.Name
	}
	return view
}

// handleSetupState reports whether the panel still needs its first owner.
func (s *Server) handleSetupState(w http.ResponseWriter, r *http.Request) {
	count, err := s.operators.Count(r.Context())
	if err != nil {
		handleError(w, r, s.log, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"initialized": count > 0,
		"operators":   count,
	})
}

// setupRequest creates the first owner.
type setupRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// handleSetup creates the first owner account. It only works while the panel
// has no operators at all.
func (s *Server) handleSetup(w http.ResponseWriter, r *http.Request) {
	var body setupRequest
	if err := decodeJSON(r, &body); err != nil {
		handleError(w, r, s.log, err)
		return
	}
	body.Username = strings.TrimSpace(body.Username)
	if body.Username == "" || body.Password == "" {
		handleError(w, r, s.log, badRequest("username and password are required"))
		return
	}
	if len(body.Password) < 8 {
		handleError(w, r, s.log, badRequest("password must be at least 8 characters"))
		return
	}

	var created auth.Operator
	err := s.db.InTx(r.Context(), func(tx *db.Tx) error {
		operators := auth.NewOperatorStore(tx)
		roles := authz.NewStore(tx)

		count, err := operators.Count(r.Context())
		if err != nil {
			return err
		}
		if count > 0 {
			return conflict("the panel is already initialised")
		}

		role, err := roles.GetByName(r.Context(), authz.RoleOwner)
		if err != nil {
			return err
		}
		hash, err := auth.HashPassword(body.Password)
		if err != nil {
			return err
		}
		op, err := operators.Create(r.Context(), auth.Operator{
			Username:     body.Username,
			PasswordHash: hash,
			RoleID:       role.ID,
			IsOwner:      true,
			Status:       auth.StatusActive,
		})
		if err != nil {
			return err
		}
		created = op

		if err := audit.NewStore(tx).Record(r.Context(), tx, audit.Entry{
			ActorID:   op.ID,
			ActorName: op.Username,
			Action:    "operator.create",
			Entity:    "operator",
			EntityID:  op.ID,
			After:     s.viewOperator(r.Context(), tx, op),
			IP:        clientIPAddress(r),
			UserAgent: userAgent(r),
			RequestID: requestID(r.Context()),
		}); err != nil {
			return err
		}
		return s.outbox.Publish(r.Context(), tx, "operator.created", map[string]any{
			"operatorId": op.ID,
			"username":   op.Username,
		})
	})
	if err != nil {
		handleError(w, r, s.log, err)
		return
	}

	token, _, err := s.sessions.Issue(r.Context(), created.ID, clientIPAddress(r), userAgent(r), false)
	if err != nil {
		handleError(w, r, s.log, err)
		return
	}
	s.setSessionCookie(w, token, false)
	role, _ := s.roles.Get(r.Context(), created.RoleID)
	writeJSON(w, http.StatusCreated, loginResponse{
		Token:    token,
		Operator: s.viewOperator(r.Context(), s.db, created),
		Role:     role,
		Scopes:   role.Scopes,
	})
}

// handleLogin signs an operator in.
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var body loginRequest
	if err := decodeJSON(r, &body); err != nil {
		handleError(w, r, s.log, err)
		return
	}
	body.Username = strings.TrimSpace(body.Username)
	if body.Username == "" || body.Password == "" {
		handleError(w, r, s.log, badRequest("username and password are required"))
		return
	}

	// Throttle repeated failures for the same account.
	failures, err := s.operators.RecentFailures(r.Context(), body.Username, time.Now().Add(-15*time.Minute).Unix())
	if err != nil {
		handleError(w, r, s.log, err)
		return
	}
	if failures >= 10 {
		w.Header().Set("Retry-After", "60")
		handleError(w, r, s.log, errorf(http.StatusTooManyRequests, CodeRateLimited, "too many failed sign in attempts"))
		return
	}

	op, err := s.operators.GetByUsername(r.Context(), body.Username)
	if err != nil {
		s.registerFailedLogin(r, body.Username)
		handleError(w, r, s.log, errorf(http.StatusUnauthorized, CodeUnauthorized, "invalid credentials"))
		return
	}
	if !auth.VerifyPassword(op.PasswordHash, body.Password) {
		s.registerFailedLogin(r, body.Username)
		handleError(w, r, s.log, errorf(http.StatusUnauthorized, CodeUnauthorized, "invalid credentials"))
		return
	}
	if op.Status != auth.StatusActive {
		handleError(w, r, s.log, errorf(http.StatusForbidden, CodeForbidden, "the account is disabled"))
		return
	}

	if op.TOTPEnabled {
		// The second factor is completed through the mfa endpoint.
		pending, err := auth.RandomToken(16)
		if err != nil {
			handleError(w, r, s.log, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"needsMfa": true,
			"pending":  pending,
		})
		return
	}

	s.completeLogin(w, r, op, body.Remember)
}

// mfaRequest carries the second factor.
type mfaRequest struct {
	Pending string `json:"pending"`
	Code    string `json:"code"`
}

// handleLoginMFA completes a sign in that requires a second factor.
func (s *Server) handleLoginMFA(w http.ResponseWriter, r *http.Request) {
	var body mfaRequest
	if err := decodeJSON(r, &body); err != nil {
		handleError(w, r, s.log, err)
		return
	}
	if body.Pending == "" || body.Code == "" {
		handleError(w, r, s.log, badRequest("pending token and code are required"))
		return
	}
	handleError(w, r, s.log, errorf(http.StatusNotImplemented, CodeInvalidRequest, "multi factor sign in is not wired yet"))
}

// completeLogin issues the session and returns the login response.
func (s *Server) completeLogin(w http.ResponseWriter, r *http.Request, op auth.Operator, remember bool) {
	token, session, err := s.sessions.Issue(r.Context(), op.ID, clientIPAddress(r), userAgent(r), remember)
	if err != nil {
		handleError(w, r, s.log, err)
		return
	}
	_ = s.operators.MarkLogin(r.Context(), op.ID, clientIPAddress(r))
	s.operators.RecordAttempt(r.Context(), op.Username, clientIPAddress(r), true)
	s.setSessionCookie(w, token, remember)

	role, err := s.roles.Get(r.Context(), op.RoleID)
	if err != nil {
		handleError(w, r, s.log, err)
		return
	}
	scopes, err := s.roles.Effective(r.Context(), role)
	if err != nil {
		handleError(w, r, s.log, err)
		return
	}

	err = s.db.InTx(r.Context(), func(tx *db.Tx) error {
		if err := audit.NewStore(tx).Record(r.Context(), tx, audit.Entry{
			ActorID:   op.ID,
			ActorName: op.Username,
			Action:    "operator.login",
			Entity:    "operator",
			EntityID:  op.ID,
			After:     map[string]any{"ip": clientIPAddress(r), "remember": remember},
			IP:        clientIPAddress(r),
			UserAgent: userAgent(r),
			RequestID: requestID(r.Context()),
		}); err != nil {
			return err
		}
		return s.outbox.Publish(r.Context(), tx, "operator.login", map[string]any{
			"operatorId": op.ID,
			"username":   op.Username,
		})
	})
	if err != nil {
		handleError(w, r, s.log, err)
		return
	}

	writeJSON(w, http.StatusOK, loginResponse{
		Token:     token,
		Operator:  s.viewOperator(r.Context(), s.db, op),
		Role:      role,
		Scopes:    scopes,
		ExpiresAt: session.ExpiresAt,
	})
}

// registerFailedLogin stores a failed sign in attempt and publishes the
// matching event.
func (s *Server) registerFailedLogin(r *http.Request, username string) {
	s.operators.RecordAttempt(r.Context(), username, clientIPAddress(r), false)
	_ = s.db.InTx(r.Context(), func(tx *db.Tx) error {
		return s.outbox.Publish(r.Context(), tx, "operator.login_failed", map[string]any{"username": username})
	})
}

// handleLogout ends the current session.
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	p := principal(r.Context())
	if p != nil && p.SessionID != "" {
		if err := s.sessions.Revoke(r.Context(), p.SessionID); err != nil {
			handleError(w, r, s.log, err)
			return
		}
	}
	s.clearSessionCookie(w)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// handleMe describes the signed in operator.
func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	p := principal(r.Context())
	role, err := s.roles.Get(r.Context(), p.Operator.RoleID)
	if err != nil {
		handleError(w, r, s.log, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"operator": s.viewOperator(r.Context(), s.db, p.Operator),
		"role":     role,
		"scopes":   p.Scopes,
		"viaToken": p.IsToken(),
	})
}

// handleMySessions lists the sessions of the signed in operator.
func (s *Server) handleMySessions(w http.ResponseWriter, r *http.Request) {
	p := principal(r.Context())
	sessions, err := s.sessions.List(r.Context(), p.Operator.ID)
	if err != nil {
		handleError(w, r, s.log, err)
		return
	}
	writeList(w, sessions, listMeta{Count: len(sessions), Limit: len(sessions)})
}

// handleRevokeSession revokes one session of the signed in operator.
func (s *Server) handleRevokeSession(w http.ResponseWriter, r *http.Request) {
	p := principal(r.Context())
	sessionID := r.PathValue("id")

	sessions, err := s.sessions.List(r.Context(), p.Operator.ID)
	if err != nil {
		handleError(w, r, s.log, err)
		return
	}
	found := false
	for _, session := range sessions {
		if session.ID == sessionID {
			found = true
			break
		}
	}
	if !found {
		handleError(w, r, s.log, notFound("session not found"))
		return
	}
	if err := s.sessions.Revoke(r.Context(), sessionID); err != nil {
		handleError(w, r, s.log, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// handleListOperators returns every operator.
func (s *Server) handleListOperators(w http.ResponseWriter, r *http.Request) {
	operators, err := s.operators.List(r.Context())
	if err != nil {
		handleError(w, r, s.log, err)
		return
	}
	views := make([]operatorView, 0, len(operators))
	for _, op := range operators {
		views = append(views, s.viewOperator(r.Context(), s.db, op))
	}
	writeList(w, views, listMeta{Count: len(views), Limit: len(views)})
}

// createOperatorRequest creates an operator.
type createOperatorRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	RoleID   string `json:"roleId"`
}

// handleCreateOperator creates an operator.
func (s *Server) handleCreateOperator(w http.ResponseWriter, r *http.Request) {
	var body createOperatorRequest
	if err := decodeJSON(r, &body); err != nil {
		handleError(w, r, s.log, err)
		return
	}
	body.Username = strings.TrimSpace(body.Username)
	if body.Username == "" || body.Password == "" {
		handleError(w, r, s.log, badRequest("username and password are required"))
		return
	}
	if len(body.Password) < 8 {
		handleError(w, r, s.log, badRequest("password must be at least 8 characters"))
		return
	}

	role, err := s.roles.Get(r.Context(), body.RoleID)
	if err != nil {
		if errors.Is(err, authz.ErrNotFound) {
			handleError(w, r, s.log, badRequest("unknown role"))
			return
		}
		handleError(w, r, s.log, err)
		return
	}

	hash, err := auth.HashPassword(body.Password)
	if err != nil {
		handleError(w, r, s.log, err)
		return
	}

	var created auth.Operator
	err = s.db.InTx(r.Context(), func(tx *db.Tx) error {
		operators := auth.NewOperatorStore(tx)
		op, err := operators.Create(r.Context(), auth.Operator{
			Username:     body.Username,
			PasswordHash: hash,
			RoleID:       role.ID,
			Status:       auth.StatusActive,
		})
		if err != nil {
			return err
		}
		created = op

		if err := audit.NewStore(tx).Record(r.Context(), tx, audit.Entry{
			ActorID:   principal(r.Context()).Operator.ID,
			ActorName: principal(r.Context()).Operator.Username,
			Action:    "operator.create",
			Entity:    "operator",
			EntityID:  op.ID,
			After:     s.viewOperator(r.Context(), tx, op),
			IP:        clientIPAddress(r),
			UserAgent: userAgent(r),
			RequestID: requestID(r.Context()),
		}); err != nil {
			return err
		}
		return s.outbox.Publish(r.Context(), tx, "operator.created", map[string]any{
			"operatorId": op.ID,
			"username":   op.Username,
		})
	})
	if err != nil {
		if errors.Is(err, auth.ErrDuplicateUsername) {
			handleError(w, r, s.log, conflict("username is already taken"))
			return
		}
		handleError(w, r, s.log, err)
		return
	}
	writeJSON(w, http.StatusCreated, s.viewOperator(r.Context(), s.db, created))
}

// handleGetOperator reads one operator.
func (s *Server) handleGetOperator(w http.ResponseWriter, r *http.Request) {
	op, err := s.operators.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		if errors.Is(err, auth.ErrNotFound) {
			handleError(w, r, s.log, notFound("operator not found"))
			return
		}
		handleError(w, r, s.log, err)
		return
	}
	writeJSON(w, http.StatusOK, s.viewOperator(r.Context(), s.db, op))
}

// updateOperatorRequest updates an operator.
type updateOperatorRequest struct {
	Username *string `json:"username"`
	Password *string `json:"password"`
	RoleID   *string `json:"roleId"`
	Status   *string `json:"status"`
}

// handleUpdateOperator updates an operator.
func (s *Server) handleUpdateOperator(w http.ResponseWriter, r *http.Request) {
	operatorID := r.PathValue("id")
	current, err := s.operators.Get(r.Context(), operatorID)
	if err != nil {
		if errors.Is(err, auth.ErrNotFound) {
			handleError(w, r, s.log, notFound("operator not found"))
			return
		}
		handleError(w, r, s.log, err)
		return
	}

	var body updateOperatorRequest
	if err := decodeJSON(r, &body); err != nil {
		handleError(w, r, s.log, err)
		return
	}

	before := s.viewOperator(r.Context(), s.db, current)
	updated := current
	if body.Username != nil {
		updated.Username = strings.TrimSpace(*body.Username)
		if updated.Username == "" {
			handleError(w, r, s.log, badRequest("username must not be empty"))
			return
		}
	}
	if body.RoleID != nil {
		role, err := s.roles.Get(r.Context(), *body.RoleID)
		if err != nil {
			if errors.Is(err, authz.ErrNotFound) {
				handleError(w, r, s.log, badRequest("unknown role"))
				return
			}
			handleError(w, r, s.log, err)
			return
		}
		updated.RoleID = role.ID
	}
	if body.Status != nil {
		if *body.Status != auth.StatusActive && *body.Status != auth.StatusDisabled {
			handleError(w, r, s.log, badRequest("status must be active or disabled"))
			return
		}
		updated.Status = *body.Status
	}
	if body.Password != nil && len(*body.Password) < 8 {
		handleError(w, r, s.log, badRequest("password must be at least 8 characters"))
		return
	}

	err = s.db.InTx(r.Context(), func(tx *db.Tx) error {
		operators := auth.NewOperatorStore(tx)
		if err := operators.Update(r.Context(), updated); err != nil {
			return err
		}
		if body.Password != nil {
			hash, err := auth.HashPassword(*body.Password)
			if err != nil {
				return err
			}
			if err := operators.SetPassword(r.Context(), operatorID, hash); err != nil {
				return err
			}
		}
		if err := audit.NewStore(tx).Record(r.Context(), tx, audit.Entry{
			ActorID:   principal(r.Context()).Operator.ID,
			ActorName: principal(r.Context()).Operator.Username,
			Action:    "operator.update",
			Entity:    "operator",
			EntityID:  operatorID,
			Before:    before,
			After:     s.viewOperator(r.Context(), tx, updated),
			IP:        clientIPAddress(r),
			UserAgent: userAgent(r),
			RequestID: requestID(r.Context()),
		}); err != nil {
			return err
		}
		return s.outbox.Publish(r.Context(), tx, "operator.updated", map[string]any{
			"operatorId": operatorID,
			"username":   updated.Username,
		})
	})
	if err != nil {
		if errors.Is(err, auth.ErrDuplicateUsername) {
			handleError(w, r, s.log, conflict("username is already taken"))
			return
		}
		var apiErr *apiError
		if errors.As(err, &apiErr) {
			handleError(w, r, s.log, apiErr)
			return
		}
		handleError(w, r, s.log, err)
		return
	}

	result, err := s.operators.Get(r.Context(), operatorID)
	if err != nil {
		handleError(w, r, s.log, err)
		return
	}
	writeJSON(w, http.StatusOK, s.viewOperator(r.Context(), s.db, result))
}

// handleDeleteOperator removes an operator.
func (s *Server) handleDeleteOperator(w http.ResponseWriter, r *http.Request) {
	operatorID := r.PathValue("id")
	current, err := s.operators.Get(r.Context(), operatorID)
	if err != nil {
		if errors.Is(err, auth.ErrNotFound) {
			handleError(w, r, s.log, notFound("operator not found"))
			return
		}
		handleError(w, r, s.log, err)
		return
	}
	if current.IsOwner {
		handleError(w, r, s.log, forbidden("the owner account cannot be removed"))
		return
	}
	if principal(r.Context()).Operator.ID == operatorID {
		handleError(w, r, s.log, forbidden("you cannot remove your own account"))
		return
	}

	err = s.db.InTx(r.Context(), func(tx *db.Tx) error {
		operators := auth.NewOperatorStore(tx)
		if err := operators.Delete(r.Context(), operatorID); err != nil {
			return err
		}
		if err := audit.NewStore(tx).Record(r.Context(), tx, audit.Entry{
			ActorID:   principal(r.Context()).Operator.ID,
			ActorName: principal(r.Context()).Operator.Username,
			Action:    "operator.delete",
			Entity:    "operator",
			EntityID:  operatorID,
			Before:    s.viewOperator(r.Context(), tx, current),
			IP:        clientIPAddress(r),
			UserAgent: userAgent(r),
			RequestID: requestID(r.Context()),
		}); err != nil {
			return err
		}
		return s.outbox.Publish(r.Context(), tx, "operator.deleted", map[string]any{
			"operatorId": operatorID,
			"username":   current.Username,
		})
	})
	if err != nil {
		handleError(w, r, s.log, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// handleListEventTypes returns the event catalogue.
func (s *Server) handleListEventTypes(w http.ResponseWriter, r *http.Request) {
	types := s.registry.List()
	writeList(w, types, listMeta{Count: len(types), Limit: len(types)})
}

// handlePanelInfo describes the running panel.
func (s *Server) handlePanelInfo(w http.ResponseWriter, r *http.Request) {
	body := map[string]any{
		"name":          "RyxoUI",
		"apiVersion":    APIVersion,
		"version":       s.cfg.Version,
		"platform":      s.cfg.Platform,
		"dialect":       string(s.cfg.DB.Dialect()),
		"basePath":      s.cfg.BasePath,
		"operatorCount": operatorCount(s),
	}
	if updateManager != nil {
		state := updateManager.State(r.Context())
		body["updateAvailable"] = state.UpdateAvailable()
		body["latestVersion"] = state.LatestVersion
	}
	writeJSON(w, http.StatusOK, body)
}

func operatorCount(s *Server) int {
	count, err := s.operators.Count(context.Background())
	if err != nil {
		return 0
	}
	return count
}

// ensure the events package is referenced by the API layer.
var _ = events.FamilyPanel
