package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/ryxoui/ryxoui/internal/nodes"
)

// handleInstallNode sets a node up over SSH.
//
// The job runs in the background and the endpoint answers with it right away.
// The password authenticates the first contact and is never stored.
func (s *Server) handleInstallNode(w http.ResponseWriter, r *http.Request) {
	nodeID := r.PathValue("id")
	node, err := s.nodes.Get(r.Context(), nodeID)
	if err != nil {
		if errors.Is(err, nodes.ErrNotFound) {
			handleError(w, r, s.log, notFound("node not found"))
			return
		}
		handleError(w, r, s.log, err)
		return
	}

	var body struct {
		Password string `json:"password"`
	}
	if r.ContentLength > 0 {
		if err := decodeJSON(r, &body); err != nil {
			handleError(w, r, s.log, err)
			return
		}
	}
	password := strings.TrimSpace(body.Password)
	if password == "" {
		handleError(w, r, s.log, badRequest("the password of the node is required"))
		return
	}

	job, err := s.installer.Start(r.Context(), node, password)
	if err != nil {
		handleError(w, r, s.log, err)
		return
	}
	writeJSON(w, http.StatusAccepted, job)
}

// handleInstallJobs returns the install jobs of a node with their log.
func (s *Server) handleInstallJobs(w http.ResponseWriter, r *http.Request) {
	nodeID := r.PathValue("id")
	if _, err := s.nodes.Get(r.Context(), nodeID); err != nil {
		if errors.Is(err, nodes.ErrNotFound) {
			handleError(w, r, s.log, notFound("node not found"))
			return
		}
		handleError(w, r, s.log, err)
		return
	}

	limit := queryInt(r, "limit", 10)
	jobs, err := s.jobs.List(r.Context(), nodeID, limit)
	if err != nil {
		handleError(w, r, s.log, err)
		return
	}

	for i := range jobs {
		if log := s.installer.Log(jobs[i].ID); log != "" {
			jobs[i].Log = log
		}
	}
	writeList(w, jobs, listMeta{Count: len(jobs), Limit: limit})
}
