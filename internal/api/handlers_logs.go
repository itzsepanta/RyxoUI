package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/ryxoui/ryxoui/internal/nodes"
)

// handleNodeLogs returns the last lines of the log of a node.
func (s *Server) handleNodeLogs(w http.ResponseWriter, r *http.Request) {
	node, key, ok := s.nodeWithKey(w, r)
	if !ok {
		return
	}

	lines, err := nodes.Log(r.Context(), node, key, nodes.LogOptions{
		Lines:   queryInt(r, "lines", 200),
		Since:   queryInt(r, "since", 0),
		Service: r.URL.Query().Get("service"),
	})
	if err != nil {
		handleError(w, r, s.log, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": lines, "meta": listMeta{Count: len(lines), Limit: len(lines)}})
}

// handleNodeLogStream follows the log of a node and writes every line as an
// event of the stream.
func (s *Server) handleNodeLogStream(w http.ResponseWriter, r *http.Request) {
	node, key, ok := s.nodeWithKey(w, r)
	if !ok {
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		handleError(w, r, s.log, unavailable("the server cannot stream"))
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	// The stream ends when the browser closes it.
	ctx := r.Context()
	heartbeat := time.NewTicker(20 * time.Second)
	defer heartbeat.Stop()

	err := nodes.TailLog(ctx, node, key, nodes.LogOptions{
		Service: r.URL.Query().Get("service"),
	}, func(line nodes.LogLine) {
		payload, err := json.Marshal(line)
		if err != nil {
			return
		}
		fmt.Fprintf(w, "data: %s\n\n", payload)
		flusher.Flush()
	})

	if err != nil && !errors.Is(err, context.Canceled) {
		s.log.Warn("the log stream ended", "node", node.Name, "error", err)
	}
}

// nodeWithKey reads the node of a request and the key the panel uses to
// reach it.
func (s *Server) nodeWithKey(w http.ResponseWriter, r *http.Request) (nodes.Node, string, bool) {
	nodeID := r.PathValue("id")
	node, err := s.nodes.Get(r.Context(), nodeID)
	if err != nil {
		if errors.Is(err, nodes.ErrNotFound) {
			handleError(w, r, s.log, notFound("node not found"))
			return nodes.Node{}, "", false
		}
		handleError(w, r, s.log, err)
		return nodes.Node{}, "", false
	}

	if !node.KeyInstalled {
		handleError(w, r, s.log, conflict("the key of the node is not installed yet"))
		return nodes.Node{}, "", false
	}

	key, err := s.keys.Get(r.Context(), node.SSHKeyID)
	if err != nil {
		if errors.Is(err, nodes.ErrKeyNotFound) {
			handleError(w, r, s.log, notFound("the node has no key"))
			return nodes.Node{}, "", false
		}
		handleError(w, r, s.log, err)
		return nodes.Node{}, "", false
	}
	return node, key.PrivateKey, true
}
