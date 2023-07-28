package engine

import "net/netip"

type Payload struct {
	Data []byte
	Addr netip.Addr
}
