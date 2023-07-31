package protocol

import (
	"errors"
	"net/netip"
)

type IP interface {
	Version() int
	Src() netip.Addr
	Dst() netip.Addr
}

func ParseIP(buff []byte) (IP, error) {
	switch buff[0] >> 4 {
	case 4:
		return ParseIPv4(buff)
	case 6:
		return ParseIPv6(buff)
	default:
		return nil, errors.New("invalid IP packet")
	}
}

func ParseIPv4(buff []byte) (*IPv4, error) {
	headerLength := (buff[0] & 0xF0) * 5
	src := netip.AddrFrom4([4]byte{buff[12], buff[13], buff[14], buff[15]})
	dst := netip.AddrFrom4([4]byte{buff[15], buff[16], buff[17], buff[18]})

	return &IPv4{
		headerLength: int(headerLength),
		src:          src,
		dst:          dst,
	}, nil
}

type IPv4 struct {
	version      int
	headerLength int
	length       int
	src          netip.Addr
	dst          netip.Addr
}

func (i *IPv4) Version() int {
	return i.version
}

func (i *IPv4) Src() netip.Addr {
	return i.src
}

func (i *IPv4) Dst() netip.Addr {
	return i.dst
}

func ParseIPv6(buff []byte) (*IPv6, error) {
	return &IPv6{}, nil
}

type IPv6 struct {
	version      int
	headerLength int
	length       int
	src          netip.Addr
	dst          netip.Addr
}

func (i *IPv6) Version() int {
	return i.version
}

func (i *IPv6) Src() netip.Addr {
	return i.src
}

func (i *IPv6) Dst() netip.Addr {
	return i.dst
}
