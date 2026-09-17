package app

import (
	"net"
	"sync"
)

// runtimeListener bridges two upstream lifecycle details: OnListen runs before
// fasthttp registers the listener, and a timed-out shutdown may leave live
// connections. Accept signals actual Serve entry; tracked connections can be
// closed on timeout before waiting for the remaining handlers to finish.
type runtimeListener struct {
	net.Listener
	accepting   chan struct{}
	acceptOnce  sync.Once
	closeOnce   sync.Once
	closeErr    error
	mu          sync.Mutex
	connections map[*runtimeConn]struct{}
	forced      bool
	drained     sync.WaitGroup
}

func wrapListener(ln net.Listener) *runtimeListener {
	return &runtimeListener{Listener: ln, accepting: make(chan struct{}), connections: make(map[*runtimeConn]struct{})}
}

func (ln *runtimeListener) Accept() (net.Conn, error) {
	ln.acceptOnce.Do(func() { close(ln.accepting) })
	conn, err := ln.Listener.Accept()
	if err != nil {
		return nil, err
	}
	ln.mu.Lock()
	defer ln.mu.Unlock()
	if ln.forced {
		_ = conn.Close()
		return nil, net.ErrClosed
	}
	c := &runtimeConn{Conn: conn, listener: ln}
	ln.drained.Add(1)
	ln.connections[c] = struct{}{}
	return c, nil
}

func (ln *runtimeListener) Close() error {
	ln.closeOnce.Do(func() { ln.closeErr = ln.Listener.Close() })
	return ln.closeErr
}

func (ln *runtimeListener) closeConnections() {
	ln.mu.Lock()
	ln.forced = true
	connections := make([]*runtimeConn, 0, len(ln.connections))
	for conn := range ln.connections {
		connections = append(connections, conn)
	}
	ln.mu.Unlock()
	// Closing the transport unblocks I/O, but only the server's later Close
	// signals that its handler has actually finished. forced prevents new Adds.
	for _, conn := range connections {
		_ = conn.Conn.Close()
	}
}

type runtimeConn struct {
	net.Conn
	listener *runtimeListener
	closed   sync.Once
}

func (c *runtimeConn) Close() error {
	err := c.Conn.Close()
	c.closed.Do(func() {
		c.listener.mu.Lock()
		delete(c.listener.connections, c)
		c.listener.mu.Unlock()
		c.listener.drained.Done()
	})
	return err
}
