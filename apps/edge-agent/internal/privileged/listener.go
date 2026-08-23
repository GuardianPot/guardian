package privileged

import (
	"net"
	"sync"
)

type limitedListener struct {
	net.Listener
	tokens   chan struct{}
	closed   chan struct{}
	closeOne sync.Once
	closeErr error
}

func newLimitedListener(listener net.Listener, limit int) net.Listener {
	return &limitedListener{
		Listener: listener,
		tokens:   make(chan struct{}, limit),
		closed:   make(chan struct{}),
	}
}

func (l *limitedListener) Accept() (net.Conn, error) {
	select {
	case l.tokens <- struct{}{}:
	case <-l.closed:
		return nil, net.ErrClosed
	}
	connection, err := l.Listener.Accept()
	if err != nil {
		<-l.tokens
		return nil, err
	}
	return &limitedConn{
		Conn:    connection,
		release: func() { <-l.tokens },
	}, nil
}

func (l *limitedListener) Close() error {
	l.closeOne.Do(func() {
		close(l.closed)
		l.closeErr = l.Listener.Close()
	})
	return l.closeErr
}

type limitedConn struct {
	net.Conn
	release func()
	once    sync.Once
	err     error
}

func (c *limitedConn) Unwrap() net.Conn { return c.Conn }

func (c *limitedConn) Close() error {
	c.once.Do(func() {
		c.err = c.Conn.Close()
		c.release()
	})
	return c.err
}
