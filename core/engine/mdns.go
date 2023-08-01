package engine

import (
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
)

func (e *Engine) HandlePeerFound(pi peer.AddrInfo) {
	e.log.Infof(e.ctx, "find %s by mDNS", pi)
	e.host.Peerstore().AddAddrs(peer.ID(pi.ID.String()), pi.Addrs, 5*time.Minute)
}
