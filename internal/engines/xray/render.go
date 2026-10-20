package xray

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ryxoui/ryxoui/internal/engines"
)

// Render turns the configuration of a node into the file of the engine.
//
// The listeners and the outgoing connections of the node carry the settings
// of the engine in their own language, so the panel only adds what it owns:
// the tag of the entry, the address it listens on and the clients of the
// users.
func (e *Engine) Render(ctx context.Context, target engines.Target, config engines.Config) ([]byte, error) {
	inbounds := []map[string]any{}
	for _, entry := range config.Entrypoints {
		if !entry.Enabled {
			continue
		}
		inbound := copyMap(entry.Config)
		if inbound == nil {
			inbound = map[string]any{}
		}
		inbound["tag"] = entry.Tag
		if _, ok := inbound["listen"]; !ok {
			inbound["listen"] = "0.0.0.0"
		}
		if _, ok := inbound["protocol"]; !ok && entry.Type != "" {
			inbound["protocol"] = entry.Type
		}
		if clients := clientsFor(entry, config.Users); len(clients) > 0 {
			inbound["clients"] = clients
		}
		inbounds = append(inbounds, inbound)
	}

	outbounds := []map[string]any{}
	for _, egress := range config.Egresses {
		if !egress.Enabled {
			continue
		}
		outbound := copyMap(egress.Config)
		if outbound == nil {
			outbound = map[string]any{}
		}
		outbound["tag"] = egress.Tag
		if _, ok := outbound["protocol"]; !ok {
			outbound["protocol"] = "freedom"
		}
		outbounds = append(outbounds, outbound)
	}

	// The engine needs at least one way out.
	if len(outbounds) == 0 {
		outbounds = append(outbounds, map[string]any{
			"protocol": "freedom",
			"tag":      "direct",
		})
	}

	document := map[string]any{
		"log":       map[string]any{"loglevel": logLevel(config)},
		"inbounds":  inbounds,
		"outbounds": outbounds,
	}

	if rules := routingRules(config); len(rules) > 0 {
		document["routing"] = map[string]any{"rules": rules}
	}

	content, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("xray: encode the configuration: %w", err)
	}
	return content, nil
}

// clientsFor builds the clients of the users for one entry point.
//
// An entry point that carries no clients serves a single client that is
// written in its settings, so the panel leaves it alone.
func clientsFor(entry engines.Entrypoint, users []engines.User) []map[string]any {
	settings := entry.Config
	if settings == nil {
		return nil
	}
	if _, ok := settings["clients"]; !ok {
		return nil
	}

	clients := []map[string]any{}
	for _, user := range users {
		if !user.Enabled {
			continue
		}
		clients = append(clients, clientOf(user))
	}
	return clients
}

// clientOf builds the client of a user.
func clientOf(user engines.User) map[string]any {
	client := map[string]any{}
	if user.UUID != "" {
		client["id"] = user.UUID
	}
	if user.Email != "" {
		client["email"] = user.Email
	}
	if user.Password != "" {
		client["password"] = user.Password
	}
	if user.Flow != "" {
		client["flow"] = user.Flow
	}
	return client
}

// routingRules builds the rules of the destinations of the node.
func routingRules(config engines.Config) []map[string]any {
	rules := []map[string]any{}
	for _, endpoint := range config.Endpoints {
		if !endpoint.Enabled {
			continue
		}
		rule := copyMap(endpoint.Config)
		if rule == nil {
			continue
		}
		if endpoint.Tag != "" {
			if _, ok := rule["outboundTag"]; !ok {
				rule["outboundTag"] = endpoint.Tag
			}
		}
		if _, ok := rule["type"]; !ok {
			rule["type"] = "field"
		}
		rules = append(rules, rule)
	}
	return rules
}

// logLevel is the level the engine writes its log with.
func logLevel(config engines.Config) string {
	if config.LogLevel != "" {
		return config.LogLevel
	}
	return "warning"
}

// copyMap copies a map of settings.
func copyMap(source map[string]any) map[string]any {
	if source == nil {
		return nil
	}
	out := make(map[string]any, len(source))
	for key, value := range source {
		out[key] = value
	}
	return out
}

// clientsOf reads the clients out of a configuration of the engine.
func clientsOf(content []byte) ([]engines.User, error) {
	document, err := decodeConfig(content)
	if err != nil {
		return nil, err
	}

	inbounds, _ := document["inbounds"].([]any)
	users := []engines.User{}
	for _, item := range inbounds {
		inbound, ok := item.(map[string]any)
		if !ok {
			continue
		}
		clients, _ := inbound["clients"].([]any)
		for _, clientItem := range clients {
			client, ok := clientItem.(map[string]any)
			if !ok {
				continue
			}
			user := engines.User{}
			if value, ok := client["id"].(string); ok {
				user.UUID = value
			}
			if value, ok := client["email"].(string); ok {
				user.Email = value
			}
			if value, ok := client["password"].(string); ok {
				user.Password = value
			}
			if value, ok := client["flow"].(string); ok {
				user.Flow = value
			}
			user.ID = user.UUID
			user.Enabled = true
			users = append(users, user)
		}
	}
	return users, nil
}

// renderUsers writes the clients back into a configuration of the engine.
//
// The first entry point that takes clients carries them, so the engine
// keeps the clients it had.
func renderUsers(users []engines.User) ([]byte, error) {
	clients := []map[string]any{}
	for _, user := range users {
		clients = append(clients, clientOf(user))
	}

	document := map[string]any{
		"log":       map[string]any{"loglevel": "warning"},
		"inbounds":  []any{map[string]any{"tag": "ryxoui", "listen": "0.0.0.0", "protocol": "vless", "port": 443, "clients": clients}},
		"outbounds": []any{map[string]any{"protocol": "freedom", "tag": "direct"}},
	}

	content, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("xray: encode the configuration: %w", err)
	}
	return content, nil
}

// EngineNames returns the names of the engines that are known.
func EngineNames() string {
	return strings.Join(engines.Names(), ", ")
}
