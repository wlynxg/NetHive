package conn

import (
	"net"
	"net/netip"
)

type udpConn struct {
	conn *net.UDPConn
}

func NewUDPConn(addr netip.AddrPort) (Conn, error) {
	udp, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP(addr.Addr().String()), Port: int(addr.Port())})
	if err != nil {
		return nil, err
	}
	return &udpConn{conn: udp}, nil
}

func (u *udpConn) WriteTo(buff []byte, addr net.Addr) (int, error) {
	return u.conn.WriteTo(buff, addr)
}

func (u *udpConn) ReadFrom(buff []byte) (int, net.Addr, error) {
	return u.conn.ReadFrom(buff)
}

func (u *udpConn) Close() error {
	return u.conn.Close()
}
