package engine

import (
	"NetHive/core/device"
	"NetHive/core/protocol"
	"NetHive/core/route"
	"NetHive/pkgs/xsync"
	"context"
	"fmt"
	"io"
	"net/netip"
	"sync"
	"time"

	"github.com/gogf/gf/v2/os/glog"
	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
	"github.com/libp2p/go-libp2p/p2p/discovery/routing"
	"github.com/libp2p/go-libp2p/p2p/discovery/util"
	"go4.org/netipx"
)

const (
	BuffSize          = 1500
	ChanSize          = 1500
	VPNStreamProtocol = "/NetHive/vpn"
)

type PacketChan chan Payload

type Engine struct {
	log    *glog.Logger
	ctx    context.Context
	cancel context.CancelFunc
	opt    *Option
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
		m    xsync.Map[peer.ID, []netip.Prefix]
		set  xsync.Map[peer.ID, *netipx.IPSet]
		id   xsync.Map[peer.ID, PacketChan]
		addr xsync.Map[netip.Addr, PacketChan]
	}
}

func New(opt *Option) (*Engine, error) {
	var (
		e = new(Engine)
	)
	e.log = glog.New()
	e.opt = opt
	ctx, cancel := context.WithCancel(context.Background())
	e.ctx = ctx
	e.cancel = cancel
	e.devWriter = make(PacketChan, ChanSize)
	e.devReader = make(PacketChan, ChanSize)
	e.relayChan = make(chan peer.AddrInfo, ChanSize)
	e.routeTable.m = xsync.Map[peer.ID, []netip.Prefix]{}
	e.routeTable.set = xsync.Map[peer.ID, *netipx.IPSet]{}
	e.routeTable.id = xsync.Map[peer.ID, PacketChan]{}
	e.routeTable.addr = xsync.Map[netip.Addr, PacketChan]{}

	// create tun
	tun, err := device.CreateTUN(opt.TUNName, opt.MTU)
	if err != nil {
		return nil, err
	}
	e.device = tun

	name, err := e.device.Name()
	if err != nil {
		return nil, err
	}

	for id, prefixes := range e.opt.PeersRouteTable {
		e.routeTable.m.Store(id, prefixes)

		b := netipx.IPSetBuilder{}
		for _, prefix := range prefixes {
			err := route.Add(name, prefix)
			if err != nil {
				return nil, err
			}
			b.AddPrefix(prefix)
		}
		set, err := b.IPSet()
		if err != nil {
			return nil, err
		}
		e.routeTable.set.Store(id, set)
	}

	pk, err := opt.PrivateKey.PrivKey()
	if err != nil {
		return nil, err
	}
	node, err := libp2p.New(
		libp2p.Identity(pk),
		libp2p.EnableAutoRelayWithPeerSource(func(ctx context.Context, num int) <-chan peer.AddrInfo { return e.relayChan }),
	)
	if err != nil {
		return nil, err
	}
	e.host = node
	e.log.Infof(e.ctx, "host ID: %s", node.ID().String())
	e.dht, err = dht.New(e.ctx, e.host)
	if err != nil {
		return nil, err
	}
	e.discovery = routing.NewRoutingDiscovery(e.dht)
	e.mdns = mdns.NewMdnsService(e.host, "_net._hive", e)

	return e, nil
}

func (e *Engine) Start() error {
	defer e.cancel()
	opt := e.opt

	if err := e.device.AddAddress(opt.LocalAddr); err != nil {
		return err
	}

	if err := e.device.Up(); err != nil {
		return err
	}

	wg := sync.WaitGroup{}
	for _, info := range dht.GetDefaultBootstrapPeerAddrInfos() {
		info := info
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := e.host.Connect(e.ctx, info); err != nil {
				e.log.Warningf(e.ctx, "connection %s fails, because of error :%s", info.String(), err)
			}
		}()
	}
	wg.Wait()

	e.host.SetStreamHandler(VPNStreamProtocol, func(stream network.Stream) {
		id := peer.ID(stream.Conn().RemotePeer().String())
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
				e.log.Errorf(e.ctx, "Peer [%s] stream write error: %s", string(id), err)
			}
		}()

		defer func() {
			stream.Close()
			set, ok := e.routeTable.set.Load(id)
			if !ok {
				return
			}

			var addr []netip.Addr
			e.routeTable.addr.Range(func(key netip.Addr, value PacketChan) bool {
				if set.Contains(key) {
					addr = append(addr, key)
				}
				return true
			})

			for _, a := range addr {
				e.routeTable.addr.Delete(a)
			}
		}()
		_, err := io.Copy(dev, stream)
		if err != nil && err != io.EOF {
			e.log.Errorf(e.ctx, "Peer [%s] stream read error: %s", string(id), err)
		}
	})

	util.Advertise(e.ctx, e.discovery, e.host.ID().String())
	go func() {
		ticker := time.NewTimer(5 * time.Minute)
		for {
			select {
			case <-ticker.C:
				peers, err := e.dht.GetClosestPeers(e.ctx, string(e.host.ID()))
				if err != nil {
					e.log.Warningf(e.ctx, "Failed to get nearest node: %s", err)
					continue
				}

				for _, id := range peers {
					e.relayChan <- e.host.Peerstore().PeerInfo(id)
				}
			case <-e.ctx.Done():
				ticker.Stop()
				return
			}
		}
	}()

	if err := e.mdns.Start(); err != nil {
		return err
	}

	go e.RoutineTUNReader()
	go e.RoutineTUNWriter()
	go e.RoutineRouteTableWriter()

	if err := <-e.errChan; err != nil {
		return err
	}
	return nil
}

func (e *Engine) RoutineTUNReader() {
	var (
		buff = make([]byte, BuffSize)
		err  error
		n    int
	)
	for {
		n, err = e.device.Read(buff)
		if err != nil {
			e.errChan <- fmt.Errorf("[RoutineTUNReader]: %s", err)
			return
		}
		ip, err := protocol.ParseIP(buff[:n])
		if err != nil {
			e.log.Warningf(e.ctx, "[RoutineTUNReader] drop packet, because %s", err)
			continue
		}
		payload := Payload{
			Src:  ip.Src(),
			Dst:  ip.Dst(),
			Data: make([]byte, n),
		}
		copy(payload.Data, buff[:n])
		select {
		case e.devReader <- payload:
		default:
			e.log.Warningf(e.ctx, "[RoutineTUNReader] drop packet: %s, because the sending queue is already full", payload.Dst)
		}
	}
}

func (e *Engine) RoutineTUNWriter() {
	var (
		payload Payload
		err     error
	)

	for payload = range e.devWriter {
		_, err = e.device.Write(payload.Data)
		if err != nil {
			e.log.Error(e.ctx, fmt.Errorf("[RoutineTUNWriter]: %s", err))
			return
		}
	}
}

func (e *Engine) RoutineRouteTableWriter() {
	var (
		payload Payload
	)

	for payload = range e.devReader {
		var conn PacketChan

		if payload.Dst.IsMulticast() {
			e.routeTable.m.Range(func(key peer.ID, value []netip.Prefix) bool {
				if key == peer.ID(e.host.ID().String()) {
					return true
				}

				if conn, ok := e.routeTable.id.Load(key); ok {
					select {
					case conn <- payload:
					default:
						e.log.Warningf(e.ctx, "[RoutineRouteTableWriter] drop packet: %s, because the sending queue is already full", payload.Dst)
					}
					return true
				}

				conn, err := e.addConnByID(key)
				if err != nil {
					e.log.Warningf(e.ctx, "[RoutineRouteTableWriter] drop packet: %s, because %s", payload.Dst, err)
					return true
				}

				select {
				case conn <- payload:
				default:
					e.log.Warningf(e.ctx, "[RoutineRouteTableWriter] drop packet: %s, because the sending queue is already full", payload.Dst)
				}

				return true
			})
		} else {
			c, ok := e.routeTable.addr.Load(payload.Dst)
			if ok {
				conn = c
			} else {
				c, err := e.addConnByDst(payload.Dst)
				if err != nil {
					e.log.Warningf(e.ctx, "[RoutineRouteTableWriter] drop packet: %s, because %s", payload.Dst, err)
					continue
				}
				conn = c
			}

			select {
			case conn <- payload:
			default:
				e.log.Warningf(e.ctx, "[RoutineRouteTableWriter] drop packet: %s, because the sending queue is already full", payload.Dst)
			}
		}
	}
}
