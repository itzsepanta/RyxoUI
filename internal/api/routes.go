package api

import (
	"net/http"

	"github.com/ryxoui/ryxoui/internal/authz"
)

// routeTable builds the route table. Every entry declares the scope it
// needs so that authorisation cannot be forgotten.
func (s *Server) routeTable() []Route {
	return []Route{
		// Setup and authentication.
		{Method: http.MethodGet, Path: "/api/setup", Summary: "Report whether the panel is initialised.", Public: true, Handler: s.handleSetupState},
		{Method: http.MethodPost, Path: "/api/setup", Summary: "Create the first owner account.", Public: true, Handler: s.handleSetup},
		{Method: http.MethodPost, Path: "/api/login", Summary: "Sign in with a username and password.", Public: true, Handler: s.handleLogin},
		{Method: http.MethodPost, Path: "/api/login/mfa", Summary: "Complete a sign in with a second factor.", Public: true, Handler: s.handleLoginMFA},
		{Method: http.MethodPost, Path: "/api/logout", Summary: "End the current session.", Handler: s.handleLogout},

		// Current operator.
		{Method: http.MethodGet, Path: "/api/me", Summary: "Describe the signed in operator.", Handler: s.handleMe},
		{Method: http.MethodGet, Path: "/api/me/sessions", Summary: "List the sessions of the signed in operator.", Handler: s.handleMySessions},
		{Method: http.MethodDelete, Path: "/api/me/sessions/{id}", Summary: "Revoke one of your sessions.", Handler: s.handleRevokeSession},

		// Second factor.
		{Method: http.MethodGet, Path: "/api/me/2fa", Summary: "Report the state of your second factor.", Handler: s.handle2FAStatus},
		{Method: http.MethodPost, Path: "/api/me/2fa/totp", Summary: "Start the enrollment of a time based code.", Handler: s.handleTOTPStart},
		{Method: http.MethodPost, Path: "/api/me/2fa/totp/confirm", Summary: "Confirm the enrollment and receive recovery codes.", Handler: s.handleTOTPConfirm},
		{Method: http.MethodDelete, Path: "/api/me/2fa", Summary: "Turn the second factor off.", Handler: s.handle2FADisable},
		{Method: http.MethodPost, Path: "/api/me/2fa/recovery", Summary: "Consume a recovery code.", Handler: s.handle2FARecovery},

		// Operators.
		{Method: http.MethodGet, Path: "/api/operators", Summary: "List panel operators.", Scope: authz.ScopeOperatorsRead, Handler: s.handleListOperators},
		{Method: http.MethodPost, Path: "/api/operators", Summary: "Create an operator.", Scope: authz.ScopeOperatorsWrite, Handler: s.handleCreateOperator},
		{Method: http.MethodGet, Path: "/api/operators/{id}", Summary: "Read one operator.", Scope: authz.ScopeOperatorsRead, Handler: s.handleGetOperator},
		{Method: http.MethodPut, Path: "/api/operators/{id}", Summary: "Update an operator.", Scope: authz.ScopeOperatorsWrite, Handler: s.handleUpdateOperator},
		{Method: http.MethodDelete, Path: "/api/operators/{id}", Summary: "Remove an operator.", Scope: authz.ScopeOperatorsWrite, Handler: s.handleDeleteOperator},

		// Roles.
		{Method: http.MethodGet, Path: "/api/roles", Summary: "List roles.", Scope: authz.ScopeRolesRead, Handler: s.handleListRoles},
		{Method: http.MethodPost, Path: "/api/roles", Summary: "Create a role.", Scope: authz.ScopeRolesWrite, Handler: s.handleCreateRole},
		{Method: http.MethodGet, Path: "/api/roles/{id}", Summary: "Read one role.", Scope: authz.ScopeRolesRead, Handler: s.handleGetRole},
		{Method: http.MethodPut, Path: "/api/roles/{id}", Summary: "Update a role.", Scope: authz.ScopeRolesWrite, Handler: s.handleUpdateRole},
		{Method: http.MethodDelete, Path: "/api/roles/{id}", Summary: "Remove a role.", Scope: authz.ScopeRolesWrite, Handler: s.handleDeleteRole},
		{Method: http.MethodGet, Path: "/api/scopes", Summary: "List the scope catalogue.", Scope: authz.ScopeRolesRead, Handler: s.handleListScopes},

		// Settings.
		{Method: http.MethodGet, Path: "/api/settings", Summary: "List panel settings.", Scope: authz.ScopeSettingsRead, Handler: s.handleListSettings},
		{Method: http.MethodPut, Path: "/api/settings/{key}", Summary: "Change a panel setting.", Scope: authz.ScopeSettingsWrite, Handler: s.handleUpdateSetting},

		// API tokens.
		{Method: http.MethodGet, Path: "/api/tokens", Summary: "List your API tokens.", Scope: authz.ScopeTokensRead, Handler: s.handleListTokens},
		{Method: http.MethodPost, Path: "/api/tokens", Summary: "Create an API token.", Scope: authz.ScopeTokensWrite, Handler: s.handleCreateToken},
		{Method: http.MethodDelete, Path: "/api/tokens/{id}", Summary: "Revoke an API token.", Scope: authz.ScopeTokensWrite, Handler: s.handleDeleteToken},

		// Observability.
		{Method: http.MethodGet, Path: "/api/audit", Summary: "Read the audit log.", Scope: authz.ScopeAuditRead, Handler: s.handleListAudit},
		{Method: http.MethodGet, Path: "/api/event-types", Summary: "List the event catalogue.", Scope: authz.ScopeRolesRead, Handler: s.handleListEventTypes},
		{Method: http.MethodGet, Path: "/api/panel/info", Summary: "Describe the panel build.", Scope: authz.ScopeSettingsRead, Handler: s.handlePanelInfo},

		// Nodes.
		{Method: http.MethodGet, Path: "/api/nodes", Summary: "List the nodes.", Scope: authz.ScopeNodesRead, Handler: s.handleListNodes},
		{Method: http.MethodPost, Path: "/api/nodes", Summary: "Register a node.", Scope: authz.ScopeNodesWrite, Handler: s.handleCreateNode},
		{Method: http.MethodGet, Path: "/api/nodes/{id}", Summary: "Read one node.", Scope: authz.ScopeNodesRead, Handler: s.handleGetNode},
		{Method: http.MethodPut, Path: "/api/nodes/{id}", Summary: "Change a node.", Scope: authz.ScopeNodesWrite, Handler: s.handleUpdateNode},
		{Method: http.MethodDelete, Path: "/api/nodes/{id}", Summary: "Remove a node.", Scope: authz.ScopeNodesWrite, Handler: s.handleDeleteNode},
		{Method: http.MethodGet, Path: "/api/nodes/{id}/health", Summary: "Read the health of a node.", Scope: authz.ScopeNodesRead, Handler: s.handleNodeHealth},
		{Method: http.MethodGet, Path: "/api/nodes/{id}/usage", Summary: "Read the traffic of a node.", Scope: authz.ScopeNodesRead, Handler: s.handleNodeUsage},
		{Method: http.MethodGet, Path: "/api/nodes/{id}/config", Summary: "Render the configuration of a node.", Scope: authz.ScopeNodesRead, Handler: s.handleNodeConfig},
		{Method: http.MethodGet, Path: "/api/nodes/{id}/logs", Summary: "Read the log of a node.", Scope: authz.ScopeNodesRead, Handler: s.handleNodeLogs},
		{Method: http.MethodGet, Path: "/api/nodes/{id}/logs/stream", Summary: "Follow the log of a node.", Scope: authz.ScopeNodesRead, Handler: s.handleNodeLogStream},
		{Method: http.MethodGet, Path: "/api/engines", Summary: "Describe the protocol engine of the panel.", Scope: authz.ScopeNodesRead, Handler: s.handleNodeEngine},
		{Method: http.MethodPost, Path: "/api/nodes/{id}/probe", Summary: "Ask a node about itself over SSH.", Scope: authz.ScopeNodesInstall, Handler: s.handleProbeNode},
		{Method: http.MethodPost, Path: "/api/nodes/{id}/install", Summary: "Set a node up over SSH.", Scope: authz.ScopeNodesInstall, Handler: s.handleInstallNode},
		{Method: http.MethodPost, Path: "/api/nodes/{id}/apply", Summary: "Write the configuration of a node on the node.", Scope: authz.ScopeNodesInstall, Handler: s.handleApplyNode},
		{Method: http.MethodGet, Path: "/api/nodes/{id}/install", Summary: "Read the install jobs of a node.", Scope: authz.ScopeNodesRead, Handler: s.handleInstallJobs},

		// Keys of the panel.
		{Method: http.MethodGet, Path: "/api/ssh-keys", Summary: "List the keys of the panel.", Scope: authz.ScopeNodesRead, Handler: s.handleListKeys},
		{Method: http.MethodPost, Path: "/api/ssh-keys", Summary: "Generate a key for the panel.", Scope: authz.ScopeNodesWrite, Handler: s.handleCreateKey},
		{Method: http.MethodDelete, Path: "/api/ssh-keys/{id}", Summary: "Remove a key of the panel.", Scope: authz.ScopeNodesWrite, Handler: s.handleDeleteKey},

		// Configuration of the nodes.
		{Method: http.MethodGet, Path: "/api/entrypoints", Summary: "List the listeners.", Scope: authz.ScopeNodesRead, Handler: s.handleListEntryPoints},
		{Method: http.MethodPost, Path: "/api/entrypoints", Summary: "Create a listener.", Scope: authz.ScopeNodesWrite, Handler: s.handleCreateEntryPoint},
		{Method: http.MethodPut, Path: "/api/entrypoints/{id}", Summary: "Change a listener.", Scope: authz.ScopeNodesWrite, Handler: s.handleUpdateEntryPoint},
		{Method: http.MethodDelete, Path: "/api/entrypoints/{id}", Summary: "Remove a listener.", Scope: authz.ScopeNodesWrite, Handler: s.handleDeleteEntryPoint},
		{Method: http.MethodGet, Path: "/api/egresses", Summary: "List the outgoing connections.", Scope: authz.ScopeNodesRead, Handler: s.handleListEgresses},
		{Method: http.MethodPost, Path: "/api/egresses", Summary: "Create an outgoing connection.", Scope: authz.ScopeNodesWrite, Handler: s.handleCreateEgress},
		{Method: http.MethodPut, Path: "/api/egresses/{id}", Summary: "Change an outgoing connection.", Scope: authz.ScopeNodesWrite, Handler: s.handleUpdateEgress},
		{Method: http.MethodDelete, Path: "/api/egresses/{id}", Summary: "Remove an outgoing connection.", Scope: authz.ScopeNodesWrite, Handler: s.handleDeleteEgress},
		{Method: http.MethodGet, Path: "/api/endpoints", Summary: "List the destinations.", Scope: authz.ScopeNodesRead, Handler: s.handleListEndpoints},
		{Method: http.MethodPost, Path: "/api/endpoints", Summary: "Create a destination.", Scope: authz.ScopeNodesWrite, Handler: s.handleCreateEndpoint},
		{Method: http.MethodPut, Path: "/api/endpoints/{id}", Summary: "Change a destination.", Scope: authz.ScopeNodesWrite, Handler: s.handleUpdateEndpoint},
		{Method: http.MethodDelete, Path: "/api/endpoints/{id}", Summary: "Remove a destination.", Scope: authz.ScopeNodesWrite, Handler: s.handleDeleteEndpoint},
		{Method: http.MethodGet, Path: "/api/templates", Summary: "List the templates.", Scope: authz.ScopeNodesRead, Handler: s.handleListTemplates},
		{Method: http.MethodPost, Path: "/api/templates", Summary: "Create a template.", Scope: authz.ScopeNodesWrite, Handler: s.handleCreateTemplate},
		{Method: http.MethodDelete, Path: "/api/templates/{id}", Summary: "Remove a template.", Scope: authz.ScopeNodesWrite, Handler: s.handleDeleteTemplate},

		// Documentation.
		{Method: http.MethodGet, Path: "/api/openapi.json", Summary: "The OpenAPI description of this API.", Public: true, Handler: s.handleOpenAPIJSON},
		{Method: http.MethodGet, Path: "/api/openapi.yaml", Summary: "The OpenAPI description of this API as YAML.", Public: true, Handler: s.handleOpenAPIYAML},

		// Self update.
		{Method: http.MethodGet, Path: "/api/panel/update", Summary: "Report the self update status.", Scope: authz.ScopeSettingsRead, Handler: s.handleUpdateStatus},
		{Method: http.MethodPost, Path: "/api/panel/update/check", Summary: "Look for a newer release.", Scope: authz.ScopePanelUpdate, Handler: s.handleUpdateCheck},
		{Method: http.MethodPost, Path: "/api/panel/update/apply", Summary: "Install a newer release.", Scope: authz.ScopePanelUpdate, Handler: s.handleUpdateApply},
	}
}
