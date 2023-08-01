package engine

import (
	"github.com/libp2p/go-libp2p/core/peer"
)

func (e *Engine) HandlePeerFound(pi peer.AddrInfo) {
	e.log.Infof(e.ctx, "find %s by mDNS", pi)
	e.mdnsMap.Store(pi.ID, pi.Addrs)
}
