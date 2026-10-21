package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/ryxoui/ryxoui/internal/audit"
	"github.com/ryxoui/ryxoui/internal/auth"
	"github.com/ryxoui/ryxoui/internal/authz"
	"github.com/ryxoui/ryxoui/internal/certs"
	"github.com/ryxoui/ryxoui/internal/db"
	"github.com/ryxoui/ryxoui/internal/engines"
	"github.com/ryxoui/ryxoui/internal/events"
	"github.com/ryxoui/ryxoui/internal/nodes"
	"github.com/ryxoui/ryxoui/internal/update"
)

// SessionCookieName is the name of the session cookie.
const SessionCookieName = "ryxoui_session"

// APIVersion is reported in the X-RyxoUI-Api-Version header.
const APIVersion = "1"

// Route describes one endpoint of the API.
type Route struct {
	Method  string
	Path    string
	Summary string
	Scope   authz.Scope
	Public  bool
	Handler http.HandlerFunc
}

// Config configures the server.
type Config struct {
	// DB is the storage handle.
	DB *db.DB
	// Logger receives the request log.
	Logger *slog.Logger
	// BasePath is prefixed to every route, for example "/panel".
	BasePath string
	// SecureCookies marks session cookies as secure.
	SecureCookies bool
	// RateLimitPerSecond limits requests per client. Zero disables the limit.
	RateLimitPerSecond float64
	// RateLimitBurst is the bucket size of the limiter.
	RateLimitBurst int
	// Version is the version of the running panel.
	Version string
	// Platform describes the machine the panel runs on.
	Platform string
	// Engine is the protocol engine the panel drives on the nodes. It may
	// be nil, then the panel only manages the nodes themselves.
	Engine engines.Engine
	// Repository is the release feed of the panel, as "owner/name". It is
	// used to serve the binary of the panel to the nodes.
	Repository string
}

// repositoryOf returns the release feed of the panel.
func repositoryOf(cfg Config) string {
	if cfg.Repository != "" {
		return cfg.Repository
	}
	return "itzsepanta/RyxoUI"
}

// Server serves the panel API.
type Server struct {
	cfg       Config
	db        *db.DB
	log       *slog.Logger
	mux       *http.ServeMux
	routes    []Route
	operators *auth.OperatorStore
	sessions  *auth.SessionService
	tokens    *auth.APITokenStore
	roles     *authz.Store
	audit     *audit.Store
	mfa       *auth.MFAService
	nodes     *nodes.Store
	config    *nodes.ConfigStore
	keys      *nodes.KeyStore
	jobs      *nodes.JobStore
	installer *nodes.Installer
	outbox    *events.Outbox
	registry  *events.Registry
	limiter   *limiter
	engine    engines.Engine
	certs     *certs.Store
	releases  *update.GitHubClient
	probe     func(ctx context.Context, node nodes.Node, privateKey string) (nodes.ProbeResult, error)
}

// New builds a server and registers every route.
func New(cfg Config) (*Server, error) {
	if cfg.DB == nil {
		return nil, errors.New("api: database handle is required")
	}
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	if cfg.RateLimitBurst <= 0 {
		cfg.RateLimitBurst = 60
	}

	srv := &Server{
		cfg:       cfg,
		db:        cfg.DB,
		log:       logger,
		mux:       http.NewServeMux(),
		operators: auth.NewOperatorStore(cfg.DB),
		sessions:  auth.NewSessionService(cfg.DB),
		tokens:    auth.NewAPITokenStore(cfg.DB),
		roles:     authz.NewStore(cfg.DB),
		audit:     audit.NewStore(cfg.DB),
		mfa:       auth.NewMFAService(cfg.DB),
		nodes:     nodes.NewStore(cfg.DB),
		config:    nodes.NewConfigStore(cfg.DB),
		keys:      nodes.NewKeyStore(cfg.DB),
		jobs:      nodes.NewJobStore(cfg.DB),
		outbox:    events.NewOutbox(cfg.DB),
		registry:  events.DefaultRegistry(),
		limiter:   newLimiter(cfg.RateLimitPerSecond, cfg.RateLimitBurst),
		engine:    cfg.Engine,
		certs:     certs.NewStore(cfg.DB),
		releases:  update.NewGitHubClient(repositoryOf(cfg)),
		probe:     nodes.Probe,
	}
	srv.installer = nodes.NewInstaller(srv.nodes, srv.keys, srv.jobs, srv.config, srv.probe, nodes.InstallerOptions{
		Logger: logger,
		Engine: srv.engine,
	})

	if err := srv.roles.EnsureBuiltins(context.Background()); err != nil {
		return nil, err
	}
	srv.routes = srv.routeTable()
	srv.registerRoutes()
	return srv, nil
}

// Registry returns the event catalogue.
func (s *Server) Registry() *events.Registry { return s.registry }

// Outbox returns the event queue.
func (s *Server) Outbox() *events.Outbox { return s.outbox }

// Roles returns the role store.
func (s *Server) Roles() *authz.Store { return s.roles }

// Operators returns the operator store.
func (s *Server) Operators() *auth.OperatorStore { return s.operators }

// Sessions returns the session service.
func (s *Server) Sessions() *auth.SessionService { return s.sessions }

// Tokens returns the API token store.
func (s *Server) Tokens() *auth.APITokenStore { return s.tokens }

// AuditStore returns the audit store.
func (s *Server) AuditStore() *audit.Store { return s.audit }

// Routes returns the registered routes.
func (s *Server) Routes() []Route { return s.routes }

// Handler returns the HTTP handler of the API.
func (s *Server) Handler() http.Handler {
	var handler http.Handler = s.mux
	handler = withRateLimit(s.limiter)(handler)
	handler = withLogging(s.log)(handler)
	handler = withRecover(s.log)(handler)
	handler = withRequestID(handler)
	handler = withSecurityHeaders(handler)
	return handler
}

// registerRoutes wires the route table into the multiplexer.
func (s *Server) registerRoutes() {
	for _, route := range s.routes {
		s.mux.Handle(route.Method+" "+route.Path, s.wrap(route))
	}
}

func (s *Server) wrap(route Route) http.Handler {
	var handler http.Handler = route.Handler
	if !route.Public {
		handler = s.requireScope(route.Scope)(handler)
		handler = s.authenticate(handler)
	}
	return handler
}

// requireScope rejects callers without the required scope.
func (s *Server) requireScope(scope authz.Scope) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p := principal(r.Context())
			if p == nil {
				writeError(w, http.StatusUnauthorized, CodeUnauthorized, "authentication required", nil)
				return
			}
			if scope != "" && !p.HasScope(scope) {
				writeError(w, http.StatusForbidden, CodeForbidden, "missing scope "+string(scope), nil)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// authenticate resolves the caller from a session cookie or an API token.
func (s *Server) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, err := s.resolvePrincipal(r)
		if err != nil {
			writeError(w, http.StatusUnauthorized, CodeUnauthorized, "authentication required", nil)
			return
		}
		ctx := context.WithValue(r.Context(), contextKeyPrincipal, p)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Server) resolvePrincipal(r *http.Request) (*Principal, error) {
	ctx := r.Context()

	if token := bearerToken(r); token != "" {
		record, err := s.tokens.Resolve(ctx, token)
		if err != nil {
			return nil, err
		}
		op, err := s.operators.Get(ctx, record.OperatorID)
		if err != nil {
			return nil, err
		}
		role, err := s.roles.Get(ctx, op.RoleID)
		if err != nil {
			return nil, err
		}
		scopes := record.Scopes
		if len(scopes) == 0 {
			scopes, err = s.roles.Effective(ctx, role)
			if err != nil {
				return nil, err
			}
		}
		_ = s.tokens.MarkUsed(ctx, record.ID, clientIP(r))
		return &Principal{
			Operator: op,
			Role:     role,
			Scopes:   scopes,
			TokenID:  record.ID,
			viaToken: true,
		}, nil
	}

	cookie, err := r.Cookie(SessionCookieName)
	if err != nil || cookie.Value == "" {
		return nil, auth.ErrInvalidSession
	}
	session, err := s.sessions.Resolve(ctx, cookie.Value)
	if err != nil {
		return nil, err
	}
	op, err := s.operators.Get(ctx, session.OperatorID)
	if err != nil {
		return nil, err
	}
	if op.Status != auth.StatusActive {
		return nil, auth.ErrInvalidSession
	}
	role, err := s.roles.Get(ctx, op.RoleID)
	if err != nil {
		return nil, err
	}
	scopes, err := s.roles.Effective(ctx, role)
	if err != nil {
		return nil, err
	}
	_ = s.sessions.Touch(ctx, session.ID)
	return &Principal{
		Operator:  op,
		Role:      role,
		Scopes:    scopes,
		SessionID: session.ID,
	}, nil
}

// setSessionCookie writes the session cookie.
func (s *Server) setSessionCookie(w http.ResponseWriter, token string, remember bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   s.cfg.SecureCookies,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   sessionCookieAge(remember),
	})
}

// clearSessionCookie removes the session cookie.
func (s *Server) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   s.cfg.SecureCookies,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})
}

func sessionCookieAge(remember bool) int {
	if remember {
		return int(auth.RememberTTL.Seconds())
	}
	return int(auth.SessionTTL.Seconds())
}

// clientIPAddress returns the address of the caller.
func clientIPAddress(r *http.Request) string { return clientIP(r) }

// userAgent returns the user agent of the caller.
func userAgent(r *http.Request) string {
	agent := r.Header.Get("User-Agent")
	if len(agent) > 255 {
		return agent[:255]
	}
	return agent
}

// basePath joins the configured base path with a route path.
func (s *Server) basePath(path string) string {
	base := strings.TrimSuffix(s.cfg.BasePath, "/")
	if base == "" {
		return path
	}
	return base + path
}

// now returns the current time.
func now() time.Time { return time.Now().UTC() }
