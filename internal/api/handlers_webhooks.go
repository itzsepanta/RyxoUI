package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/ryxoui/ryxoui/internal/webhooks"
)

// handleListWebhooks returns the subscriptions of the panel.
func (s *Server) handleListWebhooks(w http.ResponseWriter, r *http.Request) {
	subscriptions, err := s.webhooks.List(r.Context())
	if err != nil {
		handleError(w, r, s.log, err)
		return
	}
	writeList(w, subscriptions, listMeta{Count: len(subscriptions), Limit: len(subscriptions)})
}

// handleCreateWebhook stores a subscription.
func (s *Server) handleCreateWebhook(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name    string   `json:"name"`
		URL     string   `json:"url"`
		Secret  string   `json:"secret"`
		Events  []string `json:"events"`
		Enabled *bool    `json:"enabled"`
	}
	if err := decodeJSON(r, &body); err != nil {
		handleError(w, r, s.log, err)
		return
	}
	if strings.TrimSpace(body.Name) == "" || strings.TrimSpace(body.URL) == "" {
		handleError(w, r, s.log, badRequest("the name and the address are required"))
		return
	}

	enabled := true
	if body.Enabled != nil {
		enabled = *body.Enabled
	}

	created, err := s.webhooks.Create(r.Context(), webhooks.Subscription{
		Name:    body.Name,
		URL:     body.URL,
		Secret:  body.Secret,
		Events:  body.Events,
		Enabled: enabled,
	})
	if err != nil {
		if errors.Is(err, webhooks.ErrDuplicateName) {
			handleError(w, r, s.log, conflict("the name of the subscription is already taken"))
			return
		}
		handleError(w, r, s.log, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

// handleUpdateWebhook writes a subscription.
func (s *Server) handleUpdateWebhook(w http.ResponseWriter, r *http.Request) {
	subscriptionID := r.PathValue("id")
	current, err := s.webhooks.Get(r.Context(), subscriptionID)
	if err != nil {
		if errors.Is(err, webhooks.ErrNotFound) {
			handleError(w, r, s.log, notFound("subscription not found"))
			return
		}
		handleError(w, r, s.log, err)
		return
	}

	var body struct {
		Name    string   `json:"name"`
		URL     string   `json:"url"`
		Secret  string   `json:"secret"`
		Events  []string `json:"events"`
		Enabled *bool    `json:"enabled"`
	}
	if err := decodeJSON(r, &body); err != nil {
		handleError(w, r, s.log, err)
		return
	}

	if body.Name != "" {
		current.Name = body.Name
	}
	if body.URL != "" {
		current.URL = body.URL
	}
	if body.Secret != "" {
		current.Secret = body.Secret
	}
	if body.Events != nil {
		current.Events = body.Events
	}
	if body.Enabled != nil {
		current.Enabled = *body.Enabled
	}

	if err := s.webhooks.Update(r.Context(), current); err != nil {
		if errors.Is(err, webhooks.ErrDuplicateName) {
			handleError(w, r, s.log, conflict("the name of the subscription is already taken"))
			return
		}
		handleError(w, r, s.log, err)
		return
	}
	writeJSON(w, http.StatusOK, current)
}

// handleDeleteWebhook removes a subscription.
func (s *Server) handleDeleteWebhook(w http.ResponseWriter, r *http.Request) {
	subscriptionID := r.PathValue("id")
	if _, err := s.webhooks.Get(r.Context(), subscriptionID); err != nil {
		if errors.Is(err, webhooks.ErrNotFound) {
			handleError(w, r, s.log, notFound("subscription not found"))
			return
		}
		handleError(w, r, s.log, err)
		return
	}
	if err := s.webhooks.Delete(r.Context(), subscriptionID); err != nil {
		handleError(w, r, s.log, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
