// Package app composes the standalone HTTP service and owns its storage lifetime.
package app

import (
	"context"
	"errors"
	"net"
	"sync"
	"sync/atomic"

	"github.com/deepfurry/uptime/internal/config"
	"github.com/deepfurry/uptime/storage/bbolt"
	uptimestorage "github.com/gofiber/contrib/v3/uptime/storage"
	"github.com/gofiber/fiber/v3"
)

type runtimeStore interface {
	uptimestorage.Store
	Ping(context.Context) error
	Close() error
}

type dependencies struct {
	openStore func(string) (runtimeStore, error)
	listen    func(string, string) (net.Listener, error)
}

// Server is single-use. Its configuration must come from config.LoadFile and
// must not be mutated while the Server is in use.
type Server struct {
	cfg          config.Config
	ready        atomic.Bool
	started      atomic.Bool
	app          *fiber.App
	store        runtimeStore
	listener     *runtimeListener
	shutdownOnce sync.Once
	shutdownErr  error
	deps         dependencies
}

// New checks supported capabilities and allocates memory only. Run owns all I/O.
func New(cfg config.Config) (*Server, error) {
	if cfg.Storage.Type == config.StorageRedis {
		return nil, errors.New("Redis storage is not yet supported")
	}
	if cfg.Server.TLS != nil {
		return nil, errors.New("TLS serving is not yet supported")
	}
	if cfg.Auth != nil {
		return nil, errors.New("Basic Auth is not yet supported")
	}
	if cfg.Storage.Type != config.StorageBbolt || cfg.Storage.Bbolt == nil {
		return nil, errors.New("bbolt storage configuration is required")
	}
	return &Server{cfg: cfg, deps: dependencies{
		openStore: func(path string) (runtimeStore, error) { return bbolt.Open(bbolt.Config{Path: path}) },
		listen:    net.Listen,
	}}, nil
}

// Keep error provenance for callers without printing paths or secret-bearing
// values from an OS/upstream error. Joined errors retain each operation label.
type operationError struct {
	operation string
	cause     error
}

func (e *operationError) Error() string { return e.operation }
func (e *operationError) Unwrap() error { return e.cause }

func safeError(operation string, cause error) error {
	if cause == nil {
		return nil
	}
	return &operationError{operation: operation, cause: cause}
}
