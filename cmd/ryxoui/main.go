// Command ryxoui runs the RyxoUI panel.
//
// The panel is a single binary: "serve" starts the HTTP server, "migrate"
// applies the schema and "version" prints the build information.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/ryxoui/ryxoui/internal/api"
	"github.com/ryxoui/ryxoui/internal/db"
	"github.com/ryxoui/ryxoui/internal/engines"
	"github.com/ryxoui/ryxoui/internal/events"
	"github.com/ryxoui/ryxoui/internal/nodes"
	"github.com/ryxoui/ryxoui/internal/update"
	"github.com/ryxoui/ryxoui/internal/web"

	// The protocol engines the panel knows.
	_ "github.com/ryxoui/ryxoui/internal/engines/xray"

	// Storage drivers.
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

// defaultUpdateRepository is the release feed of the panel.
const defaultUpdateRepository = "itzsepanta/RyxoUI"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "ryxoui:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	command := "serve"
	if len(args) > 0 {
		command = args[0]
		args = args[1:]
	}

	switch command {
	case "serve":
		return serve(args)
	case "migrate":
		return migrate(args)
	case "update":
		return updateCommand(args)
	case "version":
		fmt.Printf("ryxoui %s\n", version)
		return nil
	case "help", "-h", "--help":
		usage()
		return nil
	default:
		usage()
		return fmt.Errorf("unknown command %q", command)
	}
}

func usage() {
	fmt.Println(`ryxoui — the RyxoUI panel

Usage:
  ryxoui serve   [flags]   start the panel
  ryxoui migrate [flags]   apply the database schema
  ryxoui update  [flags]   check for and install a newer release
  ryxoui version            print the version

Serve flags:
  --listen string      address to listen on (default "127.0.0.1:8080")
  --db string          database path or connection string (default "ryxoui.db")
  --driver string      "sqlite" (default) or "postgres"
  --basepath string    prefix for every route, for example "/panel"
  --tls string         "none" (default) or "auto"
  --cert-file string   TLS certificate for "auto"
  --key-file string    TLS key for "auto"
  --log-level string   "debug", "info", "warn" or "error" (default "info")

Update flags:
  --repo string        release feed as owner/name (default "itzsepanta/RyxoUI")
  --interval duration  delay between two automatic checks (default 6h)
  --service string     systemd unit to restart after an update
  --apply              install the release that was found`)
}

// flags is a small parser for the command line options used by the panel.
type flags struct {
	values map[string]string
}

func parseFlags(args []string) (*flags, error) {
	out := &flags{values: map[string]string{}}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if !strings.HasPrefix(arg, "--") {
			return nil, fmt.Errorf("unexpected argument %q", arg)
		}
		name := strings.TrimPrefix(arg, "--")
		value := ""
		if idx := strings.Index(name, "="); idx >= 0 {
			name, value = name[:idx], name[idx+1:]
			out.values[name] = value
			continue
		}
		if i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") {
			value = args[i+1]
			i++
		}
		out.values[name] = value
	}
	return out, nil
}

func (f *flags) get(name, fallback string) string {
	if value, ok := f.values[name]; ok && value != "" {
		return value
	}
	return fallback
}

// has reports whether a flag was given, even without a value.
func (f *flags) has(name string) bool {
	_, ok := f.values[name]
	return ok
}

// openDatabase opens the configured database.
func openDatabase(ctx context.Context, parsed *flags) (*db.DB, error) {
	driver := parsed.get("driver", "sqlite")
	dsn := parsed.get("db", "ryxoui.db")

	switch driver {
	case "sqlite":
		return db.Open(ctx, db.Options{
			Driver:          "sqlite",
			DSN:             dsn,
			Dialect:         db.SQLite,
			MaxOpenConns:    1,
			MaxIdleConns:    1,
			ConnMaxLifetime: 0,
		})
	case "postgres":
		return db.Open(ctx, db.Options{
			Driver:  "pgx",
			DSN:     dsn,
			Dialect: db.Postgres,
		})
	default:
		return nil, fmt.Errorf("unknown driver %q", driver)
	}
}

// migrate applies the schema.
func migrate(args []string) error {
	parsed, err := parseFlags(args)
	if err != nil {
		return err
	}
	ctx := context.Background()
	handle, err := openDatabase(ctx, parsed)
	if err != nil {
		return err
	}
	defer handle.Close()

	applied, err := db.NewMigrator(handle).Migrate()
	if err != nil {
		return err
	}
	if len(applied) == 0 {
		fmt.Println("schema is up to date")
		return nil
	}
	fmt.Printf("applied %d migration(s): %s\n", len(applied), strings.Join(applied, ", "))
	return nil
}

// updateCommand runs the self update from the command line.
func updateCommand(args []string) error {
	parsed, err := parseFlags(args)
	if err != nil {
		return err
	}
	ctx := context.Background()

	handle, err := openDatabase(ctx, parsed)
	if err != nil {
		return err
	}
	defer handle.Close()

	if _, err := db.NewMigrator(handle).Migrate(); err != nil {
		return err
	}

	manager := newUpdateManager(parsed, handle, slog.Default())
	state, err := manager.Check(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("current: %s\nlatest:  %s\nstatus:  %s\n", state.CurrentVersion, state.LatestVersion, state.Message)

	if !parsed.has("apply") {
		return nil
	}
	if !state.UpdateAvailable() {
		fmt.Println("the panel is already up to date")
		return nil
	}

	state, err = manager.Apply(ctx, state.LatestVersion)
	if err != nil {
		return err
	}
	fmt.Println(state.Message)
	return nil
}

// newUpdateManager builds the self update capability.
func newUpdateManager(parsed *flags, handle *db.DB, logger *slog.Logger) *update.Manager {
	repo := parsed.get("repo", defaultUpdateRepository)
	interval := 6 * time.Hour
	if raw := parsed.get("interval", ""); raw != "" {
		if parsed, err := time.ParseDuration(raw); err == nil {
			interval = parsed
		}
	}
	service := parsed.get("service", "")

	options := update.Options{
		CurrentVersion: version,
		Repository:     repo,
		CheckInterval:  interval,
		Logger:         logger,
	}
	if service != "" {
		options.Restart = func(ctx context.Context) error {
			return exec.CommandContext(ctx, "systemctl", "restart", service).Run()
		}
	}
	return update.NewManager(options, update.NewStore(handle))
}

// serve starts the panel.
func serve(args []string) error {
	parsed, err := parseFlags(args)
	if err != nil {
		return err
	}

	level := slog.LevelInfo
	switch parsed.get("log-level", "info") {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	handle, err := openDatabase(ctx, parsed)
	if err != nil {
		return err
	}
	defer handle.Close()

	if _, err := db.NewMigrator(handle).Migrate(); err != nil {
		return err
	}

	// The protocol engine the panel drives on the nodes.
	engine, err := engines.Get("xray")
	if err != nil {
		logger.Warn("the panel has no protocol engine", "error", err)
	}

	server, err := api.New(api.Config{
		DB:                 handle,
		Logger:             logger,
		BasePath:           parsed.get("basepath", ""),
		SecureCookies:      parsed.get("tls", "none") != "none",
		RateLimitPerSecond: 20,
		RateLimitBurst:     60,
		Version:            version,
		Platform:           update.Current().String(),
		Engine:             engine,
	})
	if err != nil {
		return err
	}

	// The self update capability checks the release feed on a fixed
	// interval and can install a newer release of the panel.
	updateManager := newUpdateManager(parsed, handle, logger)
	api.SetUpdateManager(updateManager)
	go func() {
		if err := updateManager.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
			logger.Error("update scheduler stopped", "error", err)
		}
	}()

	// The event dispatcher moves events from the outbox to the subscribers.
	dispatcher := events.NewDispatcher(events.NewOutbox(handle), handle, nil, events.DispatcherOptions{
		Interval: 2 * time.Second,
		Logger:   logger,
	})
	go func() {
		if err := dispatcher.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
			logger.Error("dispatcher stopped", "error", err)
		}
	}()

	// The collector keeps the state of the nodes fresh by asking them over
	// SSH how they are doing.
	collector := nodes.NewCollector(nodes.NewStore(handle), nodes.NewKeyStore(handle), nodes.Probe, nodes.CollectorOptions{
		Interval: 5 * time.Minute,
		Logger:   logger,
	})
	go func() {
		if err := collector.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
			logger.Error("collector stopped", "error", err)
		}
	}()

	mux := http.NewServeMux()
	mux.Handle("/api/", server.Handler())
	mux.Handle("/", web.Handler())

	listen := parsed.get("listen", "127.0.0.1:8080")
	httpServer := &http.Server{
		Addr:              listen,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("panel listening", "address", listen, "version", version)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		logger.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return httpServer.Shutdown(shutdownCtx)
	}
}
