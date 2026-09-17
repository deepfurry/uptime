package app

import (
	"context"
	"errors"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/log"
	recoverer "github.com/gofiber/fiber/v3/middleware/recover"
)

// Run opens storage, binds before constructing Uptime, and blocks until stopped.
// Cancellation is successful only when shutdown and resource cleanup succeed.
func (s *Server) Run(ctx context.Context) (result error) {
	if !s.started.CompareAndSwap(false, true) {
		return errors.New("server has already been run")
	}
	if ctx.Err() != nil {
		return nil
	}
	store, err := s.deps.openStore(s.cfg.Storage.Bbolt.Path)
	if err != nil {
		return safeError("cannot open bbolt storage", err)
	}
	s.store = store
	defer func() { result = errors.Join(result, safeError("cannot close bbolt storage", store.Close())) }()
	ln, err := s.deps.listen("tcp", s.cfg.Server.Address)
	if err != nil {
		return safeError("cannot bind HTTP listener", err)
	}
	s.listener = wrapListener(ln)
	defer func() { result = errors.Join(result, safeError("cannot close HTTP listener", s.listener.Close())) }()
	if ctx.Err() != nil {
		return nil
	}

	s.app = fiber.New(fiber.Config{AppName: "DeepFurry Uptime"})
	s.app.Use(recoverer.New())
	s.registerHealth()
	s.app.Hooks().OnPreShutdown(func() error { s.ready.Store(false); return nil })
	// Covers constructor rejection and every subsequent failure path.
	defer func() { result = errors.Join(result, s.shutdown()) }()
	handler, err := newUptime(buildUptimeConfig(s.app, store, s.cfg))
	if err != nil {
		return err
	}
	s.app.Use(handler)
	s.app.Hooks().OnListen(func(fiber.ListenData) error {
		s.ready.Store(true)
		log.Info("Uptime listening")
		return nil
	})

	finished := make(chan struct{})
	watcherDone := make(chan struct{})
	go func() {
		defer close(watcherDone)
		select {
		case <-ctx.Done():
			// Shutdown before Serve registers ln can otherwise miss the listener.
			select {
			case <-s.listener.accepting:
				_ = s.shutdown()
			case <-finished:
			}
		case <-finished:
		}
	}()
	err = s.app.Listener(s.listener, fiber.ListenConfig{DisableStartupMessage: true})
	unrequested := ctx.Err() == nil
	close(finished)
	shutdownErr := s.shutdown() // sync.Once also waits for a watcher in shutdown.
	<-watcherDone
	if unrequested {
		if err == nil {
			err = errors.New("listener returned without a shutdown request")
		}
		result = safeError("HTTP serving stopped unexpectedly", err)
	} else if err != nil && !errors.Is(err, context.Canceled) {
		result = safeError("HTTP serving failed", err)
	}
	// The deferred shutdown returns this same error once, alongside Close errors.
	if result == nil && shutdownErr == nil {
		log.Info("Uptime HTTP server stopped")
	}
	return result
}

func (s *Server) shutdown() error {
	s.shutdownOnce.Do(func() {
		s.ready.Store(false)
		err := s.app.ShutdownWithTimeout(s.cfg.Server.ShutdownTimeout)
		if err != nil {
			// fasthttp can return a deadline while handlers still use the store.
			// Force connection I/O closed, then drain before releasing bbolt.
			s.listener.closeConnections()
			s.listener.drained.Wait()
		}
		s.shutdownErr = safeError("HTTP shutdown failed", err)
	})
	return s.shutdownErr
}
