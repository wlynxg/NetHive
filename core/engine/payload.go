package engine

import "net/netip"

type Payload struct {
	Src  netip.Addr
	Dst  netip.Addr
	Data []byte
}
