package netcheck

import (
	"errors"
	"math/rand"
	"net"
	"net/netip"
	"sync"
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

func IsSymmetric(udp *net.UDPConn) (bool, error) {
	wg := sync.WaitGroup{}
	result := make(chan netip.AddrPort, len(STUNServers))
	wg.Add(len(STUNServers))
	for _, server := range STUNServers {
		go func(server string) {
			defer wg.Done()

			addr, err := net.ResolveIPAddr("ip", server)
			if err != nil {
				return
			}
			remoteAddr := &net.UDPAddr{IP: addr.IP, Port: DefaultSTUNPort}
			resp, err := sendAndRecv(udp, NoAction, remoteAddr)
			if err != nil {
				return
			}
			endpoint := resp.Attributes[MappedAddress]
			result <- netip.AddrPortFrom(netip.MustParseAddr(endpoint.IP.String()), uint16(endpoint.Port))
		}(server)
	}

	wg.Wait()
	ep := make(map[netip.AddrPort]struct{})
	count := 0
	for value := range result {
		ep[value] = struct{}{}
		count++
	}

	if count == 0 {
		return false, errors.New("stun request failed")
	} else {
		return count > 1, nil
	}
}
