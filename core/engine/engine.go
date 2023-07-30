package engine

import (
	"NetHive/core/device"
	"context"
	"errors"
	"fmt"
	"io"
	"net/netip"
	"sync"

	"github.com/gogf/gf/v2/os/glog"
	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
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
	log *glog.Logger
	ctx context.Context
	opt *Option
	// tun device
	device device.Device

	host      host.Host
	dht       *dht.IpfsDHT
	discovery *routing.RoutingDiscovery

	relayChan chan peer.AddrInfo

	devWriter PacketChan
	devReader PacketChan
	errChan   chan error

	routeTable struct {
		sync.RWMutex
		m map[netip.Prefix]peer.ID
		c map[netip.Addr]PacketChan
	}
}

func New(opt *Option) (*Engine, error) {
	var (
		e = new(Engine)
	)
	e.opt = opt

	return e, nil
}

func (e *Engine) Start() error {
	ctx, cancel := context.WithCancel(context.Background())
	e.ctx = ctx
	defer cancel()

	opt := e.opt
	e.log = glog.New()

	// create tun
	tun, err := device.CreateTUN(opt.TUNName, opt.MTU)
	if err != nil {
		return err
	}
	e.device = tun
	err = tun.AddAddress(opt.LocalAddr)
	if err != nil {
		return err
	}

	e.devWriter = make(PacketChan, ChanSize)
	e.devReader = make(PacketChan, ChanSize)
	e.relayChan = make(chan peer.AddrInfo, ChanSize)
	e.routeTable.m = make(map[netip.Prefix]peer.ID)
	e.routeTable.c = make(map[netip.Addr]PacketChan)

	node, err := libp2p.New(
		libp2p.Identity(opt.PrivateKey),
		libp2p.EnableAutoRelayWithPeerSource(func(ctx context.Context, num int) <-chan peer.AddrInfo { return e.relayChan }),
	)
	if err != nil {
		return err
	}
	e.host = node
	e.log.Infof(e.ctx, "host ID: %s", node.ID().String())
	node.SetStreamHandler(VPNStreamProtocol, func(stream network.Stream) {
		buff := make([]byte, 16)
		n, err := stream.Read(buff)
		if err != nil {
			return
		}
		dst, ok := netip.AddrFromSlice(buff[:n])
		if !ok {
			stream.Close()
			return
		}

		flag := false
		e.routeTable.RLock()
		for route, id := range e.routeTable.m {
			if route.Contains(dst) && id.String() == stream.ID() {
				flag = true
				break
			}
		}

		if !flag {
			stream.Close()
			return
		}

		peerChan := make(chan Payload, ChanSize)
		e.routeTable.c[dst] = peerChan
		dev := &devWrapper{r: e.devWriter, w: peerChan}
		io.Copy(stream, dev)
	})

	e.dht, err = dht.New(e.ctx, e.host)
	if err != nil {
		return err
	}
	e.discovery = routing.NewRoutingDiscovery(e.dht)
	util.Advertise(e.ctx, e.discovery, e.host.ID().String())

	e.routeTable.Lock()
	for id, prefixes := range opt.PeersRouteTable {
		for _, prefix := range prefixes {
			e.routeTable.m[prefix] = id
		}
	}
	e.routeTable.Unlock()

	go e.RoutineTUNReader()
	go e.RoutineTUNWriter()

	if err := <-e.errChan; err != nil {
		return err
	}
	return nil
}

func (e *Engine) RoutineTUNReader() {
	var (
		payload Payload
		buff    = make([]byte, BuffSize)
		err     error
		n       int
	)
	for {
		n, err = e.device.Read(buff)
		if err != nil {
			e.errChan <- fmt.Errorf("[RoutineTUNReader]: %s", err)
			return
		}
		copy(payload.Data, buff[:n])
		e.devReader <- payload
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
			e.errChan <- fmt.Errorf("[RoutineTUNWriter]: %s", err)
			return
		}
	}
}

func (e *Engine) RoutineRouteTableWriter() {
	var (
		payload Payload
	)

	for payload = range e.devReader {
		e.routeTable.Lock()
		var conn PacketChan
		c, ok := e.routeTable.c[payload.Addr]
		if ok {
			conn = c
		} else {
			c, err := e.addConn(payload.Addr)
			if err != nil {
				e.log.Warningf(e.ctx, "[RoutineRouteTableWriter] drop packet: %s, because %s", payload.Addr, err)
				e.routeTable.Unlock()
				continue
			}
			conn = c
		}
		e.routeTable.Unlock()

		select {
		case conn <- payload:
		default:
			e.log.Warningf(e.ctx, "[RoutineRouteTableWriter] drop packet: %s, because the sending queue is already full", payload.Addr)
		}
	}
}

func (e *Engine) addConn(dst netip.Addr) (PacketChan, error) {
	e.routeTable.Lock()
	for prefix, id := range e.routeTable.m {
		if prefix.Contains(dst) {
			peerChan := make(chan Payload, ChanSize)
			e.routeTable.c[dst] = peerChan
			dev := &devWrapper{r: e.devWriter, w: peerChan}
			go func() {
				peers, err := e.discovery.FindPeers(e.ctx, string(id))
				if err != nil {
					e.errChan <- err
				}

				for info := range peers {
					if info.ID != id || len(info.Addrs) <= 0 {
						continue
					}

					stream, err := e.host.NewStream(e.ctx, info.ID, VPNStreamProtocol)
					if err != nil {
						e.errChan <- err
					}
					e.log.Infof(e.ctx, "Peer [%s] connect success")

					go func() {
						_, err := io.Copy(stream, dev)
						if err != nil && err != io.EOF {
							e.log.Errorf(e.ctx, "Peer [%s] stream write error: %s", info.ID, err)
						}
					}()
					_, err = io.Copy(dev, stream)
					if err != nil && err != io.EOF {
						e.log.Errorf(e.ctx, "Peer [%s] stream read error: %s", info.ID, err)
						stream.Close()
						return
					}
					stream.Close()
					return
				}
			}()
			return peerChan, nil
		}
	}
	return nil, errors.New(fmt.Sprintf("unknown dst addr: %s", dst.String()))
}
