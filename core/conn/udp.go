package conn

import (
	"net"
	"net/netip"
)

type UdpConn struct {
	conn *net.UDPConn
}

func NewUDPConn(addr netip.AddrPort) (*UdpConn, error) {
	udp, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP(addr.Addr().String()), Port: int(addr.Port())})
	if err != nil {
		return nil, err
	}
	return &UdpConn{conn: udp}, nil
}

func (u *UdpConn) WriteTo(buff []byte, addr net.Addr) (int, error) {
	return u.conn.WriteTo(buff, addr)
}

func (u *UdpConn) ReadFrom(buff []byte) (int, net.Addr, error) {
	return u.conn.ReadFrom(buff)
}

func (u *UdpConn) Close() error {
	return u.conn.Close()
}
