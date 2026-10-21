package api

import (
	"net/http"
	"strings"

	"github.com/ryxoui/ryxoui/internal/keys"
)

// handleX25519Key makes a key pair of the curve X25519.
func (s *Server) handleX25519Key(w http.ResponseWriter, r *http.Request) {
	pair, err := keys.GenerateX25519()
	if err != nil {
		handleError(w, r, s.log, err)
		return
	}
	writeJSON(w, http.StatusOK, pair)
}

// handleECHKey makes the key of a domain fronting.
func (s *Server) handleECHKey(w http.ResponseWriter, r *http.Request) {
	var body struct {
		PublicName string `json:"publicName"`
	}
	if r.ContentLength > 0 {
		if err := decodeJSON(r, &body); err != nil {
			handleError(w, r, s.log, err)
			return
		}
	}
	if strings.TrimSpace(body.PublicName) == "" {
		body.PublicName = "cloudflare.com"
	}

	key, err := keys.GenerateECH(body.PublicName)
	if err != nil {
		handleError(w, r, s.log, err)
		return
	}
	writeJSON(w, http.StatusOK, key)
}

// handleVLESSEncryptionKey makes the key of the encryption of the vless
// protocol.
func (s *Server) handleVLESSEncryptionKey(w http.ResponseWriter, r *http.Request) {
	key, err := keys.GenerateVLESSEncryption()
	if err != nil {
		handleError(w, r, s.log, err)
		return
	}
	writeJSON(w, http.StatusOK, key)
}
