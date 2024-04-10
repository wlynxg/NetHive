package engine

import (
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
)

func (e *Engine) HandlePeerFound(pi peer.AddrInfo) {
	e.log.Infof("find %s by mDNS", pi)
	e.host.Peerstore().AddAddrs(pi.ID, pi.Addrs, 5*time.Minute)
}
