package api

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// openAPIVersion is the OpenAPI specification version emitted.
const openAPIVersion = "3.0.3"

// openAPIDocument builds the OpenAPI description of the API from the route
// table, so the documentation can never drift from the implementation.
func (s *Server) openAPIDocument() map[string]any {
	paths := map[string]any{}
	tags := map[string]bool{}

	for _, route := range s.routes {
		if route.Path == "/api/openapi.json" || route.Path == "/api/openapi.yaml" {
			continue
		}
		item, _ := paths[route.Path].(map[string]any)
		if item == nil {
			item = map[string]any{}
			paths[route.Path] = item
		}
		operation := map[string]any{
			"summary": route.Summary,
			"tags":    []string{tagFor(route.Path)},
			"responses": map[string]any{
				"200": map[string]any{"description": "successful response"},
				"400": map[string]any{"description": "the request was rejected"},
				"401": map[string]any{"description": "authentication is required"},
				"403": map[string]any{"description": "the caller lacks the required scope"},
				"404": map[string]any{"description": "the resource does not exist"},
				"500": map[string]any{"description": "the request could not be completed"},
			},
		}
		operation["operationId"] = operationID(route)
		if route.Scope != "" {
			operation["security"] = []any{
				map[string]any{"sessionCookie": []string{}},
				map[string]any{"bearerAuth": []string{}},
			}
			operation["x-required-scope"] = string(route.Scope)
		} else if !route.Public {
			operation["security"] = []any{
				map[string]any{"sessionCookie": []string{}},
				map[string]any{"bearerAuth": []string{}},
			}
		}
		item[strings.ToLower(route.Method)] = operation
		tags[tagFor(route.Path)] = true
	}

	tagNames := make([]string, 0, len(tags))
	for tag := range tags {
		tagNames = append(tagNames, tag)
	}
	sort.Strings(tagNames)

	tagList := make([]any, 0, len(tagNames))
	for _, tag := range tagNames {
		tagList = append(tagList, map[string]any{"name": tag})
	}

	return map[string]any{
		"openapi": openAPIVersion,
		"info": map[string]any{
			"title":       "RyxoUI Panel API",
			"version":     APIVersion,
			"description": "Management API of the RyxoUI panel.",
		},
		"tags":  tagList,
		"paths": paths,
		"components": map[string]any{
			"securitySchemes": map[string]any{
				"sessionCookie": map[string]any{
					"type": "apiKey",
					"in":   "cookie",
					"name": SessionCookieName,
				},
				"bearerAuth": map[string]any{
					"type":   "http",
					"scheme": "bearer",
				},
			},
		},
	}
}

// tagFor groups an endpoint by the first segment of its path.
func tagFor(path string) string {
	trimmed := strings.TrimPrefix(path, "/api/")
	if trimmed == "" {
		return "panel"
	}
	segment := strings.Split(trimmed, "/")[0]
	return segment
}

// operationID builds a stable identifier for an operation.
func operationID(route Route) string {
	parts := []string{strings.ToLower(route.Method)}
	for _, segment := range strings.Split(strings.Trim(route.Path, "/"), "/") {
		segment = strings.Trim(segment, "{}")
		if segment == "" {
			continue
		}
		parts = append(parts, segment)
	}
	return strings.Join(parts, "_")
}

// handleOpenAPIJSON serves the OpenAPI description as JSON.
func (s *Server) handleOpenAPIJSON(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.openAPIDocument())
}

// handleOpenAPIYAML serves the OpenAPI description as YAML.
func (s *Server) handleOpenAPIYAML(w http.ResponseWriter, r *http.Request) {
	raw, err := yaml.Marshal(s.openAPIDocument())
	if err != nil {
		handleError(w, r, s.log, err)
		return
	}
	w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(raw); err != nil {
		s.log.Error("write openapi", "error", err)
	}
}

// openAPIRaw returns the document as JSON, used by the tests.
func (s *Server) openAPIRaw() []byte {
	raw, err := json.Marshal(s.openAPIDocument())
	if err != nil {
		return nil
	}
	return raw
}
