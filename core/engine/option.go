package engine

import (
	"net/netip"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
)

type Option struct {
	TUNName         string
	MTU             int
	PrivateKey      crypto.PrivKey
	PeersRouteTable map[peer.ID][]netip.Prefix
	LocalRoute      []netip.Prefix
	LocalAddr       netip.Prefix
}
