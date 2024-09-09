package protocol

import (
	"errors"
	"net/netip"
	"sync"
)

var (
	ErrInvalidIPPacket = errors.New("invalid IP packet")
)

var ipPacketPool = sync.Pool{
	New: func() interface{} {
		return &IPPacket{}
	},
}

type IP interface {
	Version() int
	Src() netip.Addr
	Dst() netip.Addr
}

type IPPacket struct {
	version int
	src     netip.Addr
	dst     netip.Addr
}

func (ip *IPPacket) Version() int    { return ip.version }
func (ip *IPPacket) Src() netip.Addr { return ip.src }
func (ip *IPPacket) Dst() netip.Addr { return ip.dst }

func ParseIP(buff []byte) (IP, error) {
	if len(buff) < 20 {
		return nil, ErrInvalidIPPacket
	}

	version := int(buff[0] >> 4)
	ip := ipPacketPool.Get().(*IPPacket)
	ip.version = version

	var err error
	switch version {
	case 4:
		err = parseIPv4(ip, buff)
	case 6:
		err = parseIPv6(ip, buff)
	default:
		ipPacketPool.Put(ip)
		return nil, ErrInvalidIPPacket
	}

	if err != nil {
		ipPacketPool.Put(ip)
		return nil, err
	}

	return ip, nil
}

func parseIPv4(ip *IPPacket, buff []byte) error {
	var ok bool
	ip.src, ok = netip.AddrFromSlice(buff[12:16])
	if !ok || !ip.src.IsValid() {
		return ErrInvalidIPPacket
	}

	ip.dst, ok = netip.AddrFromSlice(buff[16:20])
	if !ok || !ip.dst.IsValid() {
		return ErrInvalidIPPacket
	}

	return nil
}

func parseIPv6(ip *IPPacket, buff []byte) error {
	if len(buff) < 40 {
		return ErrInvalidIPPacket
	}

	var ok bool
	ip.src, ok = netip.AddrFromSlice(buff[8:24])
	if !ok || !ip.src.IsValid() {
		return ErrInvalidIPPacket
	}

	ip.dst, ok = netip.AddrFromSlice(buff[24:40])
	if !ok || !ip.dst.IsValid() {
		return ErrInvalidIPPacket
	}

	return nil
}

func ReleaseIP(ip IP) {
	if ipPacket, ok := ip.(*IPPacket); ok {
		ipPacketPool.Put(ipPacket)
	}
}
