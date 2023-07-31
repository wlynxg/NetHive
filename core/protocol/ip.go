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

	src, ok := netip.AddrFromSlice(buff[12:16])
	if !ok || !src.IsValid() {
		return nil, errors.New("invalid IP packet")
	}

	dst, ok := netip.AddrFromSlice(buff[16:20])
	if !ok || !dst.IsValid() {
		return nil, errors.New("invalid IP packet")
	}

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
