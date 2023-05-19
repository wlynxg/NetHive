package conn

import (
	"net"
)

type Conn interface {
	// WriteTo writes a packet to addr.
	WriteTo([]byte, net.Addr) (int, error)
	// ReadFrom reads a packet from the connection
	ReadFrom([]byte) (int, net.Addr, error)
	// Close device
	Close() error
}
