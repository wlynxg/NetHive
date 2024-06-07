package stack

import (
	"encoding/binary"
	"net/netip"

	"github.com/libp2p/go-cidranger/net"
)

type Hash uint32

func NetHash(network net.IPVersion, src, dst netip.AddrPort) Hash {
	switch network {
	case net.IPv4:
		return net4HashFn(src, dst)
	case net.IPv6:
		return net6HashFn(src, dst)
	}
	return 0
}

func net4HashFn(src, dst netip.AddrPort) Hash {
	srcIP := src.Addr().As4()
	srcTotal := binary.BigEndian.Uint32(srcIP[:])

	dstIP := dst.Addr().As4()
	dstTotal := binary.BigEndian.Uint32(dstIP[:])
	return Hash(jHashMix(srcTotal, dstTotal, uint32(src.Port())<<16|uint32(dst.Port())))
}

func net6HashFn(src, dst netip.AddrPort) Hash {
	srcIP := src.Addr().As16()
	srcTotal := binary.BigEndian.Uint32(srcIP[12:])

	dstIP := dst.Addr().As16()
	dstTotal := jHashMix(binary.BigEndian.Uint32(dstIP[:4])^binary.BigEndian.Uint32(dstIP[4:8]),
		binary.BigEndian.Uint32(dstIP[8:12]), binary.BigEndian.Uint32(dstIP[12:]))
	return Hash(jHashMix(srcTotal, dstTotal, uint32(src.Port())<<16|uint32(dst.Port())))
}

func jHashMix(a, b, c uint32) uint32 {
	a -= c
	a ^= rol32(c, 4)
	c += b
	b -= a
	b ^= rol32(a, 6)
	a += c
	c -= b
	c ^= rol32(b, 8)
	b += a
	a -= c
	a ^= rol32(c, 16)
	c += b
	b -= a
	b ^= rol32(a, 19)
	a += c
	c -= b
	c ^= rol32(b, 4)
	b += a
	return c
}

func rol32(word uint32, shift uint) uint32 {
	return (word << shift) | (word >> ((-shift) & 31))
}
