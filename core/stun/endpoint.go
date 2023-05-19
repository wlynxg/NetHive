package netcheck

import (
	"math/rand"
	"net"
	"net/netip"
)

var (
	STUNServers = []string{
		"stun.syncthing.net",
		"stun.qq.com",
		"stun.miwifi.com",
		"stun.bige0.com",
		"stun.stunprotocol.org",
	}
)

func Endpoint(udp *net.UDPConn) (netip.AddrPort, error) {
	addr, err := net.ResolveIPAddr("ip", STUNServers[rand.Int31n(int32(len(STUNServers)))])
	if err != nil {
		return netip.AddrPort{}, err
	}
	remoteAddr := &net.UDPAddr{IP: addr.IP, Port: DefaultSTUNPort}
	resp, err := sendAndRecv(udp, NoAction, remoteAddr)
	if err != nil {
		return netip.AddrPort{}, err
	}
	endpoint := resp.Attributes[MappedAddress]
	return netip.AddrPortFrom(netip.MustParseAddr(endpoint.IP.String()), uint16(endpoint.Port)), nil
}
