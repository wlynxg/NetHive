package engine

import (
	"time"

	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/routing"
	"github.com/libp2p/go-libp2p/p2p/discovery/util"
	"github.com/pkg/errors"
)

const (
	DHTRetryInterval = 5 * time.Minute
)

func (e *Engine) EnableDHT() error {
	var err error
	// init dht serve
	e.dht, err = dht.New(e.ctx, e.host)
	if err != nil {
		return err
	}
	e.discovery = routing.NewRoutingDiscovery(e.dht)

	// init DHT
	if err := e.dht.Bootstrap(e.ctx); err != nil {
		return err
	}

	go e.connectBootstrapsLoop()
	return nil
}

func (e *Engine) connectBootstrapsLoop() {
	ticker := time.NewTicker(DHTRetryInterval)
	defer ticker.Stop()

	for i := 1; ; i++ {
		if err := e.connectBootstraps(); err != nil {
			e.log.Warnf("fail to connect to DHT for the %dth time: %v", i, err)
		} else {
			e.log.Infof("successfully connect DHT!")
			// advertise node info
			util.Advertise(e.ctx, e.discovery, e.host.ID().String())
			return
		}

		select {
		case <-e.ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (e *Engine) connectBootstraps() error {
	sc := make(chan struct{}, len(e.cfg.Bootstraps))
	fc := make(chan struct{}, len(e.cfg.Bootstraps))
	for _, s := range e.cfg.Bootstraps {
		addr, err := peer.AddrInfoFromString(s)
		if err != nil {
			e.log.Debugf("fail to parse %s: %v", s, err)
			continue
		}

		go func(addr peer.AddrInfo) {
			if err := e.host.Connect(e.ctx, addr); err != nil {
				e.log.Warn(err)
				fc <- struct{}{}
				return
			}
			sc <- struct{}{}
			e.log.Infof("success connect bootstrap %s", addr.ID)
		}(*addr)
	}

	for fcnt := 0; ; {
		select {
		case <-fc:
			fcnt++
			if fcnt >= len(e.cfg.Bootstraps) {
				return errors.New("can't connect bootstrap network")
			}
		case <-sc:
			return nil
		}
	}
}
