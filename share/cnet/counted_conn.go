package cnet

import (
	"net"
)

// CountedConn wraps a net.Conn and invokes callbacks when bytes are read or written
type CountedConn struct {
	net.Conn
	OnRead  func(int)
	OnWrite func(int)
}

// NewCountedConn creates a new CountedConn
func NewCountedConn(c net.Conn, onRead, onWrite func(int)) *CountedConn {
	return &CountedConn{
		Conn:    c,
		OnRead:  onRead,
		OnWrite: onWrite,
	}
}

func (c *CountedConn) Read(b []byte) (int, error) {
	n, err := c.Conn.Read(b)
	if n > 0 && c.OnRead != nil {
		c.OnRead(n)
	}
	return n, err
}

func (c *CountedConn) Write(b []byte) (int, error) {
	n, err := c.Conn.Write(b)
	if n > 0 && c.OnWrite != nil {
		c.OnWrite(n)
	}
	return n, err
}
