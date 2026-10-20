package api

import (
	"net/http"

	"github.com/ryxoui/ryxoui/internal/authz"
	"github.com/ryxoui/ryxoui/internal/update"
)

// updateManager is set by the command line entry point. It stays nil when
// the panel runs without the self update capability.
var updateManager *update.Manager

// SetUpdateManager wires the self update capability into the API.
func SetUpdateManager(manager *update.Manager) { updateManager = manager }

// UpdateManager returns the self update capability.
func UpdateManager() *update.Manager { return updateManager }

// handleUpdateStatus reports the state of the self update capability.
func (s *Server) handleUpdateStatus(w http.ResponseWriter, r *http.Request) {
	if updateManager == nil {
		writeError(w, http.StatusNotImplemented, CodeInternal, "self update is not available in this build", nil)
		return
	}
	writeJSON(w, http.StatusOK, updateManager.State(r.Context()))
}

// handleUpdateCheck asks the release feed for a newer version.
func (s *Server) handleUpdateCheck(w http.ResponseWriter, r *http.Request) {
	if updateManager == nil {
		writeError(w, http.StatusNotImplemented, CodeInternal, "self update is not available in this build", nil)
		return
	}
	state, err := updateManager.Check(r.Context())
	if err != nil {
		writeJSON(w, http.StatusOK, state)
		return
	}
	writeJSON(w, http.StatusOK, state)
}

// updateApplyRequest asks for an update to be installed.
type updateApplyRequest struct {
	Version string `json:"version"`
	Confirm bool   `json:"confirm"`
}

// handleUpdateApply downloads and installs a newer release.
func (s *Server) handleUpdateApply(w http.ResponseWriter, r *http.Request) {
	if updateManager == nil {
		writeError(w, http.StatusNotImplemented, CodeInternal, "self update is not available in this build", nil)
		return
	}

	var body updateApplyRequest
	if err := decodeJSON(r, &body); err != nil {
		handleError(w, r, s.log, err)
		return
	}
	if !body.Confirm {
		handleError(w, r, s.log, badRequest("the request must be confirmed"))
		return
	}

	state, err := updateManager.Apply(r.Context(), body.Version)
	if err != nil {
		writeJSON(w, http.StatusOK, state)
		return
	}
	writeJSON(w, http.StatusOK, state)
}

// handleUpdateScopes documents which scope guards the update endpoints.
func handleUpdateScopes() []string {
	return []string{
		string(authz.ScopePanelUpdate),
		string(authz.ScopeSettingsRead),
	}
}
