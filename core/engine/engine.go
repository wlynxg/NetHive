package engine

import (
	"context"
	"fmt"
	"io"
	"net/netip"
	"sync"

	"github.com/wlynxg/NetHive/core/config"
	"github.com/wlynxg/NetHive/core/device"
	"github.com/wlynxg/NetHive/core/protocol"
	"github.com/wlynxg/NetHive/core/route"
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
	"go4.org/netipx"
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
		set  xsync.Map[string, *netipx.IPSet]
		id   xsync.Map[string, PacketChan]
		addr xsync.Map[netip.Addr, PacketChan]
	}
}

func New(cfg *config.Config) (*Engine, error) {
	var (
		e = new(Engine)
	)
	e.log = mlog.New("engine")
	e.cfg = cfg
	e.ctx, e.cancel = context.WithCancel(context.Background())
	e.devWriter = make(PacketChan, ChanSize)
	e.devReader = make(PacketChan, ChanSize)
	e.relayChan = make(chan peer.AddrInfo, ChanSize)

	// create tun
	tun, err := device.CreateTUN(cfg.TUNName, cfg.MTU)
	if err != nil {
		return nil, err
	}
	e.device = tun

	name, err := e.device.Name()
	if err != nil {
		return nil, err
	}

	for id, prefix := range e.cfg.PeersRouteTable {
		e.log.Debugf("r: %s -> %s", id, prefix)
		e.routeTable.m.Store(id, prefix)

		b := netipx.IPSetBuilder{}
		err := route.Add(name, prefix)
		set, err := b.IPSet()
		if err != nil {
			return nil, err
		}
		e.routeTable.set.Store(id, set)
	}

	pk, err := cfg.PrivateKey.PrivKey()
	if err != nil {
		return nil, err
	}
	node, err := libp2p.New(
		libp2p.Identity(pk),
	)
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

func (e *Engine) Start() error {
	defer e.cancel()
	cfg := e.cfg

	if err := e.device.AddAddress(cfg.LocalAddr); err != nil {
		return err
	}

	if err := e.device.Up(); err != nil {
		return err
	}

	wg := sync.WaitGroup{}
	for _, info := range cfg.Bootstraps {
		addrInfo, err := peer.AddrInfoFromString(info)
		if err != nil {
			e.log.Debugf("fail to parse '%s': %v", info, err)
			continue
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := e.host.Connect(e.ctx, *addrInfo); err != nil {
				e.log.Warnf("connection %s fails, because of error :%s", addrInfo.String(), err)
			}
		}()
	}
	wg.Wait()

	e.host.SetStreamHandler(VPNStreamProtocol, e.Handler)
	util.Advertise(e.ctx, e.discovery, e.host.ID().String())

	if cfg.EnableMDNS {
		e.mdns = mdns.NewMdnsService(e.host, "_net._hive", e)
		if err := e.mdns.Start(); err != nil {
			e.log.Warnf("fail to run mdns: %v", err)
		}
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
			e.log.Warnf("[RoutineTUNReader] drop packet, because %s", err)
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
			e.log.Warnf("[RoutineTUNReader] drop packet: %s, because the sending queue is already full", payload.Dst)
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

		e.log.Debugf("%s -> %s", payload.Src, payload.Dst)
		if payload.Dst.IsMulticast() {
			e.routeTable.m.Range(func(key string, value netip.Prefix) bool {
				if key == e.host.ID().String() {
					return true
				}

				if conn, ok := e.routeTable.id.Load(key); ok {
					select {
					case conn <- payload:
					default:
						e.log.Warnf("[RoutineRouteTableWriter] drop packet: %s, because the sending queue is already full", payload.Dst)
					}
					return true
				}

				conn, err := e.addConnByID(key)
				if err != nil {
					e.log.Warnf("[RoutineRouteTableWriter] drop packet: %s, because %s", payload.Dst, err)
					return true
				}

				select {
				case conn <- payload:
				default:
					e.log.Warnf("[RoutineRouteTableWriter] drop packet: %s, because the sending queue is already full", payload.Dst)
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
					e.log.Warnf("[RoutineRouteTableWriter] drop packet: %s, because %s", payload.Dst, err)
					continue
				}
				conn = c
			}

			select {
			case conn <- payload:
			default:
				e.log.Warnf("[RoutineRouteTableWriter] drop packet: %s, because the sending queue is already full", payload.Dst)
			}
		}
	}
}

func (e *Engine) Handler(stream network.Stream) {
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
			e.log.Errorf("Peer [%s] stream write error: %s", string(id), err)
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
		e.log.Errorf("Peer [%s] stream read error: %s", string(id), err)
	}
}
