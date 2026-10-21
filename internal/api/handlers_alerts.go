package api

import (
	"net/http"
)

// handleNodeAlerts returns the warnings of a node.
func (s *Server) handleNodeAlerts(w http.ResponseWriter, r *http.Request) {
	nodeID := r.PathValue("id")
	if _, err := s.nodes.Get(r.Context(), nodeID); err != nil {
		handleError(w, r, s.log, notFound("node not found"))
		return
	}

	active := r.URL.Query().Get("all") != "1"
	alerts, err := s.alerts.Alerts(r.Context(), nodeID, active)
	if err != nil {
		handleError(w, r, s.log, err)
		return
	}
	writeList(w, alerts, listMeta{Count: len(alerts), Limit: len(alerts)})
}

// handleAlerts returns the warnings of every node.
func (s *Server) handleAlerts(w http.ResponseWriter, r *http.Request) {
	active := r.URL.Query().Get("all") != "1"
	alerts, err := s.alerts.Alerts(r.Context(), "", active)
	if err != nil {
		handleError(w, r, s.log, err)
		return
	}
	writeList(w, alerts, listMeta{Count: len(alerts), Limit: len(alerts)})
}

// handleClearAlerts ends the warnings of a node.
func (s *Server) handleClearAlerts(w http.ResponseWriter, r *http.Request) {
	nodeID := r.PathValue("id")
	if _, err := s.nodes.Get(r.Context(), nodeID); err != nil {
		handleError(w, r, s.log, notFound("node not found"))
		return
	}
	if err := s.alerts.ClearAll(r.Context(), nodeID); err != nil {
		handleError(w, r, s.log, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
