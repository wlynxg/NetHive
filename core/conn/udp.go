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

func (u *UdpConn) WriteTo(buff []byte, addr netip.AddrPort) (int, error) {
	return u.conn.WriteToUDPAddrPort(buff, addr)
}

func (u *UdpConn) ReadFrom(buff []byte) (int, netip.AddrPort, error) {
	return u.conn.ReadFromUDPAddrPort(buff)
}

func (u *UdpConn) Close() error {
	return u.conn.Close()
}
