package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/ryxoui/ryxoui/internal/audit"
	"github.com/ryxoui/ryxoui/internal/authz"
	"github.com/ryxoui/ryxoui/internal/db"
	"github.com/ryxoui/ryxoui/internal/engines"
	"github.com/ryxoui/ryxoui/internal/nodes"
)

// nodeRequest creates or updates a node.
type nodeRequest struct {
	Name            string   `json:"name"`
	Address         string   `json:"address"`
	Port            int      `json:"port"`
	Remark          string   `json:"remark"`
	Tags            []string `json:"tags"`
	Multiplier      float64  `json:"multiplier"`
	Enabled         *bool    `json:"enabled"`
	LimitBytes      int64    `json:"limitBytes"`
	LimitPeriod     string   `json:"limitPeriod"`
	LimitStartDay   int      `json:"limitStartDay"`
	LimitStartMonth int      `json:"limitStartMonth"`
	LimitDisabled   bool     `json:"limitDisabled"`
	TemplateID      string   `json:"templateId"`
	SSHHost         string   `json:"sshHost"`
	SSHPort         int      `json:"sshPort"`
	SSHUser         string   `json:"sshUser"`
	SSHKeyID        string   `json:"sshKeyId"`
	KeyInstalled    *bool    `json:"keyInstalled"`
}

// handleListNodes returns every node.
func (s *Server) handleListNodes(w http.ResponseWriter, r *http.Request) {
	list, err := s.nodes.List(r.Context())
	if err != nil {
		handleError(w, r, s.log, err)
		return
	}
	writeList(w, list, listMeta{Count: len(list), Limit: len(list)})
}

// handleGetNode reads one node.
func (s *Server) handleGetNode(w http.ResponseWriter, r *http.Request) {
	node, err := s.nodes.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		if errors.Is(err, nodes.ErrNotFound) {
			handleError(w, r, s.log, notFound("node not found"))
			return
		}
		handleError(w, r, s.log, err)
		return
	}
	writeJSON(w, http.StatusOK, node)
}

// handleCreateNode registers a node.
func (s *Server) handleCreateNode(w http.ResponseWriter, r *http.Request) {
	var body nodeRequest
	if err := decodeJSON(r, &body); err != nil {
		handleError(w, r, s.log, err)
		return
	}
	if strings.TrimSpace(body.Name) == "" || strings.TrimSpace(body.Address) == "" {
		handleError(w, r, s.log, badRequest("name and address are required"))
		return
	}

	enabled := true
	if body.Enabled != nil {
		enabled = *body.Enabled
	}
	keyInstalled := false
	if body.KeyInstalled != nil {
		keyInstalled = *body.KeyInstalled
	}
	multiplier := body.Multiplier
	if multiplier == 0 {
		multiplier = 1
	}

	var created nodes.Node
	err := s.db.InTx(r.Context(), func(tx *db.Tx) error {
		store := nodes.NewStore(tx)
		node, err := store.Create(r.Context(), nodes.Node{
			Name:            body.Name,
			Address:         body.Address,
			Port:            body.Port,
			Remark:          body.Remark,
			Tags:            body.Tags,
			Multiplier:      multiplier,
			Enabled:         enabled,
			LimitBytes:      body.LimitBytes,
			LimitPeriod:     body.LimitPeriod,
			LimitStartDay:   body.LimitStartDay,
			LimitStartMonth: body.LimitStartMonth,
			LimitDisabled:   body.LimitDisabled,
			TemplateID:      body.TemplateID,
			SSHHost:         body.SSHHost,
			SSHPort:         body.SSHPort,
			SSHUser:         body.SSHUser,
			SSHKeyID:        body.SSHKeyID,
			KeyInstalled:    keyInstalled,
			Status:          nodes.StatusConnecting,
		})
		if err != nil {
			return err
		}
		created = node

		if err := audit.NewStore(tx).Record(r.Context(), tx, audit.Entry{
			ActorID:   principal(r.Context()).Operator.ID,
			ActorName: principal(r.Context()).Operator.Username,
			Action:    "node.create",
			Entity:    "node",
			EntityID:  node.ID,
			After:     node,
			IP:        clientIPAddress(r),
			UserAgent: userAgent(r),
			RequestID: requestID(r.Context()),
		}); err != nil {
			return err
		}
		return s.outbox.Publish(r.Context(), tx, "node.created", map[string]any{
			"nodeId": node.ID,
			"name":   node.Name,
		})
	})
	if err != nil {
		if errors.Is(err, nodes.ErrDuplicateName) {
			handleError(w, r, s.log, conflict("the node name is already taken"))
			return
		}
		handleError(w, r, s.log, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

// handleUpdateNode changes a node.
func (s *Server) handleUpdateNode(w http.ResponseWriter, r *http.Request) {
	nodeID := r.PathValue("id")
	current, err := s.nodes.Get(r.Context(), nodeID)
	if err != nil {
		if errors.Is(err, nodes.ErrNotFound) {
			handleError(w, r, s.log, notFound("node not found"))
			return
		}
		handleError(w, r, s.log, err)
		return
	}

	var body nodeRequest
	if err := decodeJSON(r, &body); err != nil {
		handleError(w, r, s.log, err)
		return
	}

	before := current
	if body.Name != "" {
		current.Name = body.Name
	}
	if body.Address != "" {
		current.Address = body.Address
	}
	if body.Port != 0 {
		current.Port = body.Port
	}
	current.Remark = body.Remark
	if body.Tags != nil {
		current.Tags = body.Tags
	}
	if body.Multiplier != 0 {
		current.Multiplier = body.Multiplier
	}
	if body.Enabled != nil {
		current.Enabled = *body.Enabled
	}
	current.LimitBytes = body.LimitBytes
	current.LimitPeriod = body.LimitPeriod
	if body.LimitStartDay != 0 {
		current.LimitStartDay = body.LimitStartDay
	}
	if body.LimitStartMonth != 0 {
		current.LimitStartMonth = body.LimitStartMonth
	}
	current.LimitDisabled = body.LimitDisabled
	current.TemplateID = body.TemplateID
	if body.SSHHost != "" {
		current.SSHHost = body.SSHHost
	}
	if body.SSHPort != 0 {
		current.SSHPort = body.SSHPort
	}
	if body.SSHUser != "" {
		current.SSHUser = body.SSHUser
	}
	if body.SSHKeyID != "" {
		current.SSHKeyID = body.SSHKeyID
	}
	if body.KeyInstalled != nil {
		current.KeyInstalled = *body.KeyInstalled
	}

	err = s.db.InTx(r.Context(), func(tx *db.Tx) error {
		store := nodes.NewStore(tx)
		if err := store.Update(r.Context(), current); err != nil {
			return err
		}
		if err := audit.NewStore(tx).Record(r.Context(), tx, audit.Entry{
			ActorID:   principal(r.Context()).Operator.ID,
			ActorName: principal(r.Context()).Operator.Username,
			Action:    "node.update",
			Entity:    "node",
			EntityID:  nodeID,
			Before:    before,
			After:     current,
			IP:        clientIPAddress(r),
			UserAgent: userAgent(r),
			RequestID: requestID(r.Context()),
		}); err != nil {
			return err
		}
		return s.outbox.Publish(r.Context(), tx, "node.updated", map[string]any{
			"nodeId": nodeID,
			"name":   current.Name,
		})
	})
	if err != nil {
		if errors.Is(err, nodes.ErrDuplicateName) {
			handleError(w, r, s.log, conflict("the node name is already taken"))
			return
		}
		handleError(w, r, s.log, err)
		return
	}

	updated, err := s.nodes.Get(r.Context(), nodeID)
	if err != nil {
		handleError(w, r, s.log, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

// handleDeleteNode removes a node.
func (s *Server) handleDeleteNode(w http.ResponseWriter, r *http.Request) {
	nodeID := r.PathValue("id")
	current, err := s.nodes.Get(r.Context(), nodeID)
	if err != nil {
		if errors.Is(err, nodes.ErrNotFound) {
			handleError(w, r, s.log, notFound("node not found"))
			return
		}
		handleError(w, r, s.log, err)
		return
	}

	err = s.db.InTx(r.Context(), func(tx *db.Tx) error {
		store := nodes.NewStore(tx)
		if err := store.Delete(r.Context(), nodeID); err != nil {
			return err
		}
		if err := audit.NewStore(tx).Record(r.Context(), tx, audit.Entry{
			ActorID:   principal(r.Context()).Operator.ID,
			ActorName: principal(r.Context()).Operator.Username,
			Action:    "node.delete",
			Entity:    "node",
			EntityID:  nodeID,
			Before:    current,
			IP:        clientIPAddress(r),
			UserAgent: userAgent(r),
			RequestID: requestID(r.Context()),
		}); err != nil {
			return err
		}
		return s.outbox.Publish(r.Context(), tx, "node.deleted", map[string]any{
			"nodeId": nodeID,
			"name":   current.Name,
		})
	})
	if err != nil {
		handleError(w, r, s.log, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// handleNodeHealth returns the health readings of a node.
func (s *Server) handleNodeHealth(w http.ResponseWriter, r *http.Request) {
	nodeID := r.PathValue("id")
	samples, err := s.nodes.Health(r.Context(), nodeID, queryInt(r, "limit", 50))
	if err != nil {
		handleError(w, r, s.log, err)
		return
	}
	writeList(w, samples, listMeta{Count: len(samples), Limit: len(samples)})
}

// handleNodeUsage returns the traffic of a node.
func (s *Server) handleNodeUsage(w http.ResponseWriter, r *http.Request) {
	nodeID := r.PathValue("id")
	since := queryInt64(r, "since", 0)
	usage, err := s.nodes.Usage(r.Context(), nodeID, since)
	if err != nil {
		handleError(w, r, s.log, err)
		return
	}
	writeList(w, usage, listMeta{Count: len(usage), Limit: len(usage)})
}

// handleNodeConfig renders the configuration of a node with the engine of
// the panel. It shows what the node would run, without touching the node.
func (s *Server) handleNodeConfig(w http.ResponseWriter, r *http.Request) {
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

	config, err := s.installer.ConfigOf(r.Context(), nodeID)
	if err != nil {
		handleError(w, r, s.log, err)
		return
	}

	target := engines.Target{
		NodeID:    node.ID,
		Host:      node.SSHHostName(),
		Port:      node.SSHPort,
		User:      node.SSHUser,
		KnownHost: node.HostKey,
		OS:        node.OS,
		Arch:      node.Arch,
	}

	content, err := s.engine.Render(r.Context(), target, config)
	if err != nil {
		handleError(w, r, s.log, err)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(content); err != nil {
		s.log.Error("write the configuration of the node", "error", err)
	}
}

// handleNodeEngine reports the engine of the panel and the releases it can
// install.
func (s *Server) handleNodeEngine(w http.ResponseWriter, r *http.Request) {
	if s.engine == nil {
		handleError(w, r, s.log, unavailable("the panel has no protocol engine"))
		return
	}

	body := map[string]any{"name": s.engine.Name()}
	if versions, err := s.engine.Versions(r.Context()); err == nil {
		body["versions"] = versions
	}
	writeJSON(w, http.StatusOK, body)
}

// handleListEntryPoints returns the listeners of a node.
func (s *Server) handleListEntryPoints(w http.ResponseWriter, r *http.Request) {
	items, err := s.config.EntryPoints(r.Context(), r.URL.Query().Get("nodeId"))
	if err != nil {
		handleError(w, r, s.log, err)
		return
	}
	writeList(w, items, listMeta{Count: len(items), Limit: len(items)})
}

// handleCreateEntryPoint creates a listener.
func (s *Server) handleCreateEntryPoint(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name    string         `json:"name"`
		Tag     string         `json:"tag"`
		Type    string         `json:"type"`
		Config  map[string]any `json:"config"`
		NodeID  string         `json:"nodeId"`
		Enabled *bool          `json:"enabled"`
	}
	if err := decodeJSON(r, &body); err != nil {
		handleError(w, r, s.log, err)
		return
	}
	if body.Tag == "" || body.Type == "" {
		handleError(w, r, s.log, badRequest("tag and type are required"))
		return
	}

	enabled := true
	if body.Enabled != nil {
		enabled = *body.Enabled
	}

	created, err := s.config.CreateEntryPoint(r.Context(), nodes.EntryPoint{
		Name:    body.Name,
		Tag:     body.Tag,
		Type:    body.Type,
		Config:  body.Config,
		NodeID:  body.NodeID,
		Enabled: enabled,
	})
	if err != nil {
		if errors.Is(err, nodes.ErrDuplicateTag) {
			handleError(w, r, s.log, conflict("the tag is already taken"))
			return
		}
		handleError(w, r, s.log, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

// handleDeleteEntryPoint removes a listener.
func (s *Server) handleDeleteEntryPoint(w http.ResponseWriter, r *http.Request) {
	if err := s.config.DeleteEntryPoint(r.Context(), r.PathValue("id")); err != nil {
		handleError(w, r, s.log, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// handleListEgresses returns the outgoing connections of a node.
func (s *Server) handleListEgresses(w http.ResponseWriter, r *http.Request) {
	items, err := s.config.Egresses(r.Context(), r.URL.Query().Get("nodeId"))
	if err != nil {
		handleError(w, r, s.log, err)
		return
	}
	writeList(w, items, listMeta{Count: len(items), Limit: len(items)})
}

// handleCreateEgress creates an outgoing connection.
func (s *Server) handleCreateEgress(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name    string         `json:"name"`
		Tag     string         `json:"tag"`
		Type    string         `json:"type"`
		Config  map[string]any `json:"config"`
		NodeID  string         `json:"nodeId"`
		Enabled *bool          `json:"enabled"`
	}
	if err := decodeJSON(r, &body); err != nil {
		handleError(w, r, s.log, err)
		return
	}
	if body.Tag == "" || body.Type == "" {
		handleError(w, r, s.log, badRequest("tag and type are required"))
		return
	}

	enabled := true
	if body.Enabled != nil {
		enabled = *body.Enabled
	}

	created, err := s.config.CreateEgress(r.Context(), nodes.Egress{
		Name:    body.Name,
		Tag:     body.Tag,
		Type:    body.Type,
		Config:  body.Config,
		NodeID:  body.NodeID,
		Enabled: enabled,
	})
	if err != nil {
		if errors.Is(err, nodes.ErrDuplicateTag) {
			handleError(w, r, s.log, conflict("the tag is already taken"))
			return
		}
		handleError(w, r, s.log, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

// handleListEndpoints returns the destinations of a node.
func (s *Server) handleListEndpoints(w http.ResponseWriter, r *http.Request) {
	items, err := s.config.Endpoints(r.Context(), r.URL.Query().Get("nodeId"))
	if err != nil {
		handleError(w, r, s.log, err)
		return
	}
	writeList(w, items, listMeta{Count: len(items), Limit: len(items)})
}

// handleCreateEndpoint creates a destination.
func (s *Server) handleCreateEndpoint(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name   string         `json:"name"`
		NodeID string         `json:"nodeId"`
		Type   string         `json:"type"`
		Config map[string]any `json:"config"`
	}
	if err := decodeJSON(r, &body); err != nil {
		handleError(w, r, s.log, err)
		return
	}
	if body.Type == "" {
		handleError(w, r, s.log, badRequest("type is required"))
		return
	}

	created, err := s.config.CreateEndpoint(r.Context(), nodes.Endpoint{
		Name:   body.Name,
		NodeID: body.NodeID,
		Type:   body.Type,
		Config: body.Config,
	})
	if err != nil {
		handleError(w, r, s.log, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

// handleListTemplates returns every template.
func (s *Server) handleListTemplates(w http.ResponseWriter, r *http.Request) {
	items, err := s.config.Templates(r.Context())
	if err != nil {
		handleError(w, r, s.log, err)
		return
	}
	writeList(w, items, listMeta{Count: len(items), Limit: len(items)})
}

// handleCreateTemplate creates a template.
func (s *Server) handleCreateTemplate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name   string         `json:"name"`
		Remark string         `json:"remark"`
		Route  map[string]any `json:"route"`
		Core   map[string]any `json:"core"`
	}
	if err := decodeJSON(r, &body); err != nil {
		handleError(w, r, s.log, err)
		return
	}
	if strings.TrimSpace(body.Name) == "" {
		handleError(w, r, s.log, badRequest("name is required"))
		return
	}

	created, err := s.config.CreateTemplate(r.Context(), nodes.Template{
		Name:   body.Name,
		Remark: body.Remark,
		Route:  body.Route,
		Core:   body.Core,
	})
	if err != nil {
		if errors.Is(err, nodes.ErrDuplicateTag) {
			handleError(w, r, s.log, conflict("the template name is already taken"))
			return
		}
		handleError(w, r, s.log, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

// handleDeleteTemplate removes a template.
func (s *Server) handleDeleteTemplate(w http.ResponseWriter, r *http.Request) {
	if err := s.config.DeleteTemplate(r.Context(), r.PathValue("id")); err != nil {
		handleError(w, r, s.log, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// nodeScopes documents the scopes that guard the node endpoints.
func nodeScopes() []string {
	return []string{
		string(authz.ScopeNodesRead),
		string(authz.ScopeNodesWrite),
	}
}
