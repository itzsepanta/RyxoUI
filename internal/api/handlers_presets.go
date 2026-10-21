package api

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/ryxoui/ryxoui/internal/authz"
	"github.com/ryxoui/ryxoui/internal/nodes"
)

// handleListPresets returns the presets of the panel.
func (s *Server) handleListPresets(w http.ResponseWriter, r *http.Request) {
	kind := strings.TrimSpace(r.URL.Query().Get("kind"))
	presets, err := s.presets.Presets(r.Context(), kind)
	if err != nil {
		handleError(w, r, s.log, err)
		return
	}
	writeList(w, presets, listMeta{Count: len(presets), Limit: len(presets)})
}

// handleCreatePreset stores a new preset.
func (s *Server) handleCreatePreset(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name   string         `json:"name"`
		Kind   string         `json:"kind"`
		Type   string         `json:"type"`
		Remark string         `json:"remark"`
		Config map[string]any `json:"config"`
	}
	if err := decodeJSON(r, &body); err != nil {
		handleError(w, r, s.log, err)
		return
	}
	if strings.TrimSpace(body.Name) == "" {
		handleError(w, r, s.log, badRequest("the name of the preset must not be empty"))
		return
	}

	created, err := s.presets.Create(r.Context(), nodes.Preset{
		Name:   body.Name,
		Kind:   body.Kind,
		Type:   body.Type,
		Remark: body.Remark,
		Config: body.Config,
	})
	if err != nil {
		if errors.Is(err, nodes.ErrDuplicatePreset) {
			handleError(w, r, s.log, conflict("the preset name is already taken"))
			return
		}
		handleError(w, r, s.log, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

// handleDeletePreset removes a preset.
func (s *Server) handleDeletePreset(w http.ResponseWriter, r *http.Request) {
	presetID := r.PathValue("id")
	if err := s.presets.Delete(r.Context(), presetID); err != nil {
		handleError(w, r, s.log, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// ensurePresets writes the presets the panel ships with, when they are
// missing. A preset that an operator changed keeps its change.
func (s *Server) ensurePresets(ctx context.Context) error {
	existing, err := s.presets.Presets(ctx, "")
	if err != nil {
		return err
	}

	known := map[string]bool{}
	for _, item := range existing {
		known[item.Name] = true
	}

	for _, preset := range nodes.BuiltinPresets() {
		if known[preset.Name] {
			continue
		}
		if _, err := s.presets.Create(ctx, preset); err != nil {
			return err
		}
	}
	return nil
}

var _ = authz.ScopeNodesWrite
