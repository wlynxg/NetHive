package engine

import (
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
)

const (
	MDNSRetryInterval = 5 * time.Minute
)

func (e *Engine) HandlePeerFound(info peer.AddrInfo) {
	e.log.Debugf("mDNS get node addr info: %s", info)
	e.host.Peerstore().AddAddrs(info.ID, info.Addrs, peerstore.AddressTTL)
}

func (e *Engine) EnableMdns() error {
	if e.mdns == nil {
		// init mdns serve
		e.mdns = mdns.NewMdnsService(e.host, "_p2proxy._udp", e)
	}
	go e.mdnsLoop()
	return nil
}

func (e *Engine) mdnsLoop() {
	if err := e.mdns.Start(); err != nil {
		e.log.Warnf("fail to run mDNS service for the %dth time: %v", 1, err)
	} else {
		e.log.Infof("successfully run mDNS service!")
		return
	}

	ticker := time.NewTicker(MDNSRetryInterval)
	defer ticker.Stop()

	for i := 2; ; i++ {
		select {
		case <-e.ctx.Done():
		case <-ticker.C:
			if err := e.mdns.Start(); err != nil {
				e.log.Warnf("fail to run mDNS service for the %dth time: %v", 1, err)
			} else {
				e.log.Infof("successfully run mDNS service!")
				return
			}
		}
	}
}
