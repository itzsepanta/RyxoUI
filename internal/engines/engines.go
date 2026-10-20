// Package engines is the contract between the panel and the protocol
// engines it drives.
//
// Every engine speaks the same language: the panel renders a configuration,
// hands it to the node and asks the engine how it is doing. An adapter
// translates that language into the one of its engine, so a new protocol
// never touches the rest of the panel.
package engines

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
)

// ErrNotFound is returned when an engine is unknown.
var ErrNotFound = errors.New("engines: unknown engine")

// Target is a node an engine runs on.
type Target struct {
	// NodeID is the identifier of the node in the panel.
	NodeID string
	// Host is the address of the node.
	Host string
	// Port is the SSH port of the node.
	Port int
	// User is the account the panel uses.
	User string
	// PrivateKey is the PEM encoded key of the panel.
	PrivateKey string
	// KnownHost is the pinned host key of the node.
	KnownHost string
	// OS is the system of the node, for example "linux".
	OS string
	// Arch is the architecture of the node, for example "amd64".
	Arch string
	// Service is the name of the systemd unit of the engine.
	Service string
	// Directory is where the engine lives on the node.
	Directory string
}

// User is a client of a node.
type User struct {
	ID       string `json:"id"`
	Email    string `json:"email"`
	UUID     string `json:"uuid"`
	Password string `json:"password,omitempty"`
	Flow     string `json:"flow,omitempty"`
	LimitGB  int64  `json:"limitGb,omitempty"`
	Expiry   int64  `json:"expiry,omitempty"`
	Enabled  bool   `json:"enabled"`
}

// Usage is the traffic of a node.
type Usage struct {
	Up    int64 `json:"up"`
	Down  int64 `json:"down"`
	Total int64 `json:"total"`
}

// Health is the state of an engine on a node.
type Health struct {
	Running     bool   `json:"running"`
	Version     string `json:"version"`
	Uptime      int64  `json:"uptime"`
	Connections int    `json:"connections"`
}

// Entrypoint is a listener of a node.
type Entrypoint struct {
	ID      string
	Name    string
	Tag     string
	Type    string
	Config  map[string]any
	Enabled bool
}

// Egress is an outgoing connection of a node.
type Egress struct {
	ID      string
	Name    string
	Tag     string
	Type    string
	Config  map[string]any
	Enabled bool
}

// Endpoint is a destination of a node.
type Endpoint struct {
	ID      string
	Name    string
	Tag     string
	Type    string
	Config  map[string]any
	Enabled bool
}

// Config is the configuration of a node.
type Config struct {
	// Entrypoints are the listeners of the node.
	Entrypoints []Entrypoint
	// Egresses are the outgoing connections of the node.
	Egresses []Egress
	// Endpoints are the destinations of the node.
	Endpoints []Endpoint
	// Users are the clients of the node.
	Users []User
	// LogLevel is the level the engine writes its log with.
	LogLevel string
}

// Engine is a protocol engine the panel can drive on a node.
type Engine interface {
	// Name is the name of the engine, for example "xray".
	Name() string

	// Versions returns the releases the panel can install, newest first.
	Versions(ctx context.Context) ([]Release, error)

	// Install puts the engine on a node and starts it.
	Install(ctx context.Context, target Target, version string) error

	// Render turns the configuration of a node into the file of the engine.
	Render(ctx context.Context, target Target, config Config) ([]byte, error)

	// Apply writes the configuration on a node and restarts the engine.
	Apply(ctx context.Context, target Target, config Config) error

	// Health reports the state of the engine on a node.
	Health(ctx context.Context, target Target) (Health, error)

	// Users returns the clients the engine knows on a node.
	Users(ctx context.Context, target Target) ([]User, error)

	// UpsertUser adds a client to a node or changes it.
	UpsertUser(ctx context.Context, target Target, user User) error

	// SuspendUser removes a client from a node without losing its record.
	SuspendUser(ctx context.Context, target Target, userID string) error

	// RemoveUser removes a client from a node.
	RemoveUser(ctx context.Context, target Target, userID string) error
}

// Stats is implemented by the engines that can report their traffic.
//
// The traffic is not part of the contract of every engine, because not every
// engine can count it. The panel asks for it when the engine can.
type Stats interface {
	// Usage is the traffic of a node since a moment in time.
	Usage(ctx context.Context, target Target, since int64) (Usage, error)
}

// Release is a version of an engine the panel can install.
type Release struct {
	Version string `json:"version"`
	Asset   string `json:"asset"`
	URL     string `json:"url"`
	Size    int64  `json:"size"`
}

// Factory builds an engine by name.
type Factory func() Engine

var (
	mu        sync.RWMutex
	factories = map[string]Factory{}
)

// Register adds an engine to the catalogue. It is meant to be called from
// the init function of the package of an engine.
func Register(name string, factory Factory) {
	mu.Lock()
	defer mu.Unlock()
	factories[strings.ToLower(strings.TrimSpace(name))] = factory
}

// Get builds an engine by name.
func Get(name string) (Engine, error) {
	mu.RLock()
	factory, ok := factories[strings.ToLower(strings.TrimSpace(name))]
	mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrNotFound, name)
	}
	return factory(), nil
}

// Names returns the names of the known engines, sorted.
func Names() []string {
	mu.RLock()
	defer mu.RUnlock()

	out := make([]string, 0, len(factories))
	for name := range factories {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}
