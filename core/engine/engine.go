package engine

import (
	"context"
	"io"
	"net/netip"

	"github.com/wlynxg/NetHive/core/route"

	"github.com/wlynxg/NetHive/core/config"
	"github.com/wlynxg/NetHive/core/device"
	mlog "github.com/wlynxg/NetHive/pkgs/log"
	"github.com/wlynxg/NetHive/pkgs/xsync"

	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
	"github.com/libp2p/go-libp2p/p2p/discovery/routing"
	"github.com/libp2p/go-libp2p/p2p/discovery/util"
)

const (
	BuffSize          = 1500
	ChanSize          = 1500
	VPNStreamProtocol = "/NetHive/vpn"
)

type PacketChan chan Payload

type Engine struct {
	log    *mlog.Logger
	ctx    context.Context
	cancel context.CancelFunc
	cfg    *config.Config
	// tun device
	device device.Device

	host      host.Host
	dht       *dht.IpfsDHT
	discovery *routing.RoutingDiscovery
	mdns      mdns.Service

	relayChan chan peer.AddrInfo

	devWriter PacketChan
	devReader PacketChan
	errChan   chan error

	routeTable struct {
		m    xsync.Map[string, netip.Prefix]
		id   xsync.Map[string, PacketChan]
		addr xsync.Map[netip.Addr, PacketChan]
	}
}

func Run(ctx context.Context, cfg *config.Config) (*Engine, error) {
	var (
		e       = new(Engine)
		err     error
		options []libp2p.Option
	)

	e.cfg = cfg
	mlog.SetOutputTypes(cfg.LogConfigs...)
	e.log = mlog.New("engine")
	e.ctx, e.cancel = context.WithCancel(ctx)
	e.devWriter = make(PacketChan, ChanSize)
	e.devReader = make(PacketChan, ChanSize)

	pk, err := cfg.PrivateKey.PrivKey()
	if err != nil {
		return nil, err
	}
	options = append(options, libp2p.Identity(pk))

	if len(cfg.Relays) > 0 {
		var relays []peer.AddrInfo
		for _, relay := range cfg.Relays {
			addrInfo, err := peer.AddrInfoFromString(relay)
			if err != nil {
				e.log.Warnf("fail to parse '%s': %v", relay, err)
				continue
			}
			relays = append(relays, *addrInfo)
		}
		options = append(options, libp2p.EnableAutoRelayWithStaticRelays(relays))
	} else if cfg.EnableAutoRelay {
		e.relayChan = make(chan peer.AddrInfo, ChanSize)
		options = append(options, libp2p.EnableAutoRelayWithPeerSource(func(ctx context.Context, num int) <-chan peer.AddrInfo {
			c := make(chan peer.AddrInfo, num)
			go func() {
				defer close(c)
				for ; num >= 0; num-- {
					select {
					case v, ok := <-e.relayChan:
						if !ok {
							return
						}
						e.log.Debugf("auto relay find node: %v", v)
						select {
						case c <- v:
						case <-ctx.Done():
							return
						}
					case <-ctx.Done():
						return
					}
				}
			}()
			return c
		}))
	}

	node, err := libp2p.New(options...)
	if err != nil {
		return nil, err
	}

	e.host = node
	e.log.Infof("host ID: %s", node.ID().String())
	e.dht, err = dht.New(e.ctx, e.host)
	if err != nil {
		return nil, err
	}
	e.discovery = routing.NewRoutingDiscovery(e.dht)

	return e, nil
}

func (e *Engine) Run() error {
	var err error
	defer e.cancel()

	// TUN init
	e.device, err = device.CreateTUN(e.cfg.TUNName, e.cfg.MTU)
	if err != nil {
		return err
	}

	name, err := e.device.Name()
	if err != nil {
		return err
	}

	if err := e.device.AddAddress(e.cfg.LocalAddr); err != nil {
		return err
	}

	if err := e.device.Up(); err != nil {
		return err
	}

	for id, prefix := range e.cfg.PeersRouteTable {
		e.routeTable.m.Store(id, prefix)

		err := route.Add(name, prefix)
		if err != nil {
			e.log.Warnf("fail to add %s's route %s: %v", id, prefix, err)
			continue
		}
		e.log.Debugf("successfully add %s's route: %s", id, prefix)
	}

	if len(e.cfg.Bootstraps) > 0 {
		if err := e.EnableDHT(); err != nil {
			return err
		}
	}

	if e.cfg.EnableMDNS {
		if err := e.EnableMdns(); err != nil {
			return err
		}
	}

	if len(e.cfg.Relays) == 0 && e.cfg.EnableAutoRelay {
		// start auto relay detect
		go e.autoRelayFinder(e.ctx)
	}

	e.host.SetStreamHandler(VPNStreamProtocol, e.VPNHandler)
	util.Advertise(e.ctx, e.discovery, e.host.ID().String())

	go e.RoutineTUNReader()
	go e.RoutineTUNWriter()
	go e.RoutineRouteTableWriter()

	if err := <-e.errChan; err != nil {
		return err
	}
	return nil
}

func (e *Engine) VPNHandler(stream network.Stream) {
	e.log.Debugf("%s connect", stream.Conn().RemotePeer())

	id := stream.Conn().RemotePeer().String()
	if _, ok := e.routeTable.m.Load(id); !ok {
		stream.Close()
		return
	}

	peerChan := make(PacketChan, ChanSize)
	e.routeTable.id.Store(id, peerChan)
	defer e.routeTable.id.Delete(id)

	dev := &devWrapper{w: e.devWriter, r: peerChan}
	go func() {
		defer stream.Close()
		_, err := io.Copy(stream, dev)
		if err != nil && err != io.EOF {
			e.log.Errorf("Peer [%s] stream write error: %s", id, err)
		}
	}()

	_, err := io.Copy(dev, stream)
	if err != nil && err != io.EOF {
		e.log.Errorf("Peer [%s] stream read error: %s", id, err)
	}
}
