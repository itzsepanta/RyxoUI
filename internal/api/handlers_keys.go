package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/ryxoui/ryxoui/internal/db"
	"github.com/ryxoui/ryxoui/internal/nodes"
	"github.com/ryxoui/ryxoui/internal/ssh"
)

// handleProbeNode asks a node about itself over SSH.
//
// The probe updates the status of the node, the versions it reports, the
// pinned host key and the health samples of the panel.
func (s *Server) handleProbeNode(w http.ResponseWriter, r *http.Request) {
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

	result, err := s.probe(r.Context(), node, key.PrivateKey)
	if err != nil {
		if setErr := s.nodes.SetStatus(r.Context(), nodeID, nodes.StatusError, err.Error()); setErr != nil {
			s.log.Error("set the status of the node", "node", nodeID, "error", setErr)
		}
		handleError(w, r, s.log, err)
		return
	}

	if err := s.nodes.ApplyProbe(r.Context(), nodeID, result, nodes.StatusConnected, ""); err != nil {
		handleError(w, r, s.log, err)
		return
	}
	if err := s.nodes.RecordHealth(r.Context(), result.HealthSample(nodeID, db.Now())); err != nil {
		s.log.Error("record the health of the node", "node", nodeID, "error", err)
	}

	writeJSON(w, http.StatusOK, result)
}

// handleListKeys returns the keys of the panel. The private key never leaves
// the panel.
func (s *Server) handleListKeys(w http.ResponseWriter, r *http.Request) {
	keys, err := s.keys.List(r.Context())
	if err != nil {
		handleError(w, r, s.log, err)
		return
	}
	writeList(w, keys, listMeta{Count: len(keys), Limit: len(keys)})
}

// handleCreateKey generates a key pair for the panel.
func (s *Server) handleCreateKey(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
	}
	if r.ContentLength > 0 {
		if err := decodeJSON(r, &body); err != nil {
			handleError(w, r, s.log, err)
			return
		}
	}

	name := strings.TrimSpace(body.Name)
	if name == "" {
		name = "node-key"
	}

	pair, err := ssh.GenerateKeyPair("ryxoui")
	if err != nil {
		handleError(w, r, s.log, err)
		return
	}

	created, err := s.keys.Create(r.Context(), nodes.SSHKey{
		Name:       name,
		PublicKey:  pair.PublicKey,
		PrivateKey: pair.PrivateKey,
		Comment:    "ryxoui",
	})
	if err != nil {
		handleError(w, r, s.log, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

// handleDeleteKey removes a key from the panel.
func (s *Server) handleDeleteKey(w http.ResponseWriter, r *http.Request) {
	keyID := r.PathValue("id")
	if _, err := s.keys.Get(r.Context(), keyID); err != nil {
		if errors.Is(err, nodes.ErrKeyNotFound) {
			handleError(w, r, s.log, notFound("key not found"))
			return
		}
		handleError(w, r, s.log, err)
		return
	}
	if err := s.keys.Delete(r.Context(), keyID); err != nil {
		handleError(w, r, s.log, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
