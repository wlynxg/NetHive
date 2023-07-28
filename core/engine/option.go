package engine

import (
	"net/netip"

	"github.com/libp2p/go-libp2p/core/peer"
)

type Option struct {
	Device struct {
		TUNName string
		MTU     int
	}
	PeersRouteTable map[peer.ID][]netip.Prefix
	LocalRoute      []netip.Prefix
}
