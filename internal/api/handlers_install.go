package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/ryxoui/ryxoui/internal/engines"
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
		Version  string `json:"engineVersion"`
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

	job, err := s.installer.Start(r.Context(), node, password, strings.TrimSpace(body.Version))
	if err != nil {
		handleError(w, r, s.log, err)
		return
	}
	writeJSON(w, http.StatusAccepted, job)
}

// handleUninstallNode removes the protocol engine from a node.
func (s *Server) handleUninstallNode(w http.ResponseWriter, r *http.Request) {
	if s.engine == nil {
		handleError(w, r, s.log, unavailable("the panel has no protocol engine"))
		return
	}

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
	if !node.KeyInstalled {
		handleError(w, r, s.log, conflict("the key of the node is not installed yet"))
		return
	}

	key, err := s.keys.Get(r.Context(), node.SSHKeyID)
	if err != nil {
		if errors.Is(err, nodes.ErrKeyNotFound) {
			handleError(w, r, s.log, notFound("the node has no key"))
			return
		}
		handleError(w, r, s.log, err)
		return
	}

	target := engines.Target{
		NodeID:     node.ID,
		Host:       node.SSHHostName(),
		Port:       node.SSHPort,
		User:       node.SSHUser,
		PrivateKey: key.PrivateKey,
		KnownHost:  node.HostKey,
		OS:         node.OS,
		Arch:       node.Arch,
	}

	if err := s.engine.Uninstall(r.Context(), target); err != nil {
		handleError(w, r, s.log, err)
		return
	}

	if err := s.nodes.SetStatus(r.Context(), node.ID, nodes.StatusDisconnected, "the engine was removed"); err != nil {
		s.log.Error("set the status of the node", "node", node.Name, "error", err)
	}
	if err := s.alerts.ClearAll(r.Context(), node.ID); err != nil {
		s.log.Error("clear the warnings of the node", "node", node.Name, "error", err)
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "engine": s.engine.Name()})
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
