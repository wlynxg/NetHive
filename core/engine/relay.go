package engine

import (
	"context"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
)

func (e *Engine) autoRelayFinder(ctx context.Context) {
	e.log.Debugf("successfully start auto relay finder!")
	peers := e.host.Network().Peers()
	for _, p := range peers {
		addrs := e.host.Peerstore().Addrs(p)
		if len(addrs) == 0 {
			continue
		}
		node := peer.AddrInfo{ID: p, Addrs: addrs}
		select {
		case e.relayChan <- node:
			e.log.Debugf("find relay candidate node %s", node)
		default:
		}
	}

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			closestPeers, err := e.dht.GetClosestPeers(ctx, e.host.ID().String())
			if err != nil {
				e.log.Warnf("autoRelay get cloest peers error: %v", err)
				continue
			}

			for _, p := range closestPeers {
				addrs := e.host.Peerstore().Addrs(p)
				if len(addrs) == 0 {
					continue
				}
				node := peer.AddrInfo{ID: p, Addrs: addrs}
				select {
				case e.relayChan <- node:
					e.log.Debugf("find relay candidate node %s", node)
				default:
				}
			}
		}
	}
}
