package engine

import (
	"context"
	"sync"

	"github.com/libp2p/go-libp2p/core/peer"
)

const (
	channelBuffLength = 10
)

func (e *Engine) SearchNode(ctx context.Context, id peer.ID) <-chan peer.AddrInfo {
	ch := make(chan peer.AddrInfo, channelBuffLength)

	go func(id peer.ID) {
		defer close(ch)

		wg := sync.WaitGroup{}
		wg.Add(2)

		go func() {
			defer wg.Done()
			e.searchByCache(ctx, id, ch)
		}()

		go func() {
			defer wg.Done()
			e.searchByDHT(ctx, id, ch)
		}()
		wg.Wait()
	}(id)
	return ch
}

func (e *Engine) searchByCache(ctx context.Context, id peer.ID, ch chan peer.AddrInfo) {
	info := e.host.Peerstore().PeerInfo(id)
	if len(info.Addrs) > 0 {
		e.log.Debugf("search %s info from static: %v", id, info)
		select {
		case <-ctx.Done():
		case ch <- info:
		}
		return
	}
	e.log.Debugf("fail to search %s by static", id)
}

func (e *Engine) searchByDHT(ctx context.Context, id peer.ID, ch chan peer.AddrInfo) {
	pch, err := e.discovery.FindPeers(ctx, id.String())
	if err != nil {
		err := e.connectBootstraps()
		if err != nil {
			e.log.Debugf("fail to reconnect bootstrap: %v", err)
		}
		e.log.Debugf("fail to search %s by DHT: %v", id, err)
	} else {
		for info := range pch {
			e.log.Debugf("search %s info from DHT: %v", id, info)
			if id == info.ID && len(info.Addrs) > 0 {
				ch <- info
			}
		}
	}
}
