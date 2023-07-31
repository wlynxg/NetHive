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

		m      map[peer.ID][]netip.Prefix
		prefix map[netip.Prefix]peer.ID
		id     map[peer.ID]PacketChan
		addr   map[netip.Addr]PacketChan
	}
}

func New(opt *Option) (*Engine, error) {
	var (
		e = new(Engine)
	)
	e.opt = opt
	e.devWriter = make(PacketChan, ChanSize)
	e.devReader = make(PacketChan, ChanSize)
	e.relayChan = make(chan peer.AddrInfo, ChanSize)
	e.routeTable.m = opt.PeersRouteTable
	e.routeTable.prefix = make(map[netip.Prefix]peer.ID)
	e.routeTable.id = make(map[peer.ID]PacketChan)
	e.routeTable.addr = make(map[netip.Addr]PacketChan)
	node, err := libp2p.New(
		libp2p.Identity(opt.PrivateKey),
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

	e.host.SetStreamHandler(VPNStreamProtocol, func(stream network.Stream) {
		e.routeTable.Lock()
		id := peer.ID(stream.ID())
		prefixs, ok := e.routeTable.m[id]
		if !ok {
			stream.Close()
			return
		}

		peerChan := make(PacketChan, ChanSize)
		e.routeTable.id[id] = peerChan
		for _, prefix := range prefixs {
			e.routeTable.prefix[prefix] = id
		}

		dev := &devWrapper{w: e.devWriter, r: peerChan}
		go func() {
			_, err := io.Copy(stream, dev)
			if err != nil && err != io.EOF {
				e.log.Errorf(e.ctx, "Peer [%s] stream write error: %s", id, err)
				stream.Close()
				return
			}
		}()
		_, err = io.Copy(dev, stream)
		if err != nil && err != io.EOF {
			e.log.Errorf(e.ctx, "Peer [%s] stream read error: %s", id, err)
		}
		stream.Close()
		return
	})

	util.Advertise(e.ctx, e.discovery, e.host.ID().String())

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
		fmt.Println(buff[:n])
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
		c, ok := e.routeTable.addr[payload.Addr]
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
	for prefix, id := range e.routeTable.prefix {
		if prefix.Contains(dst) {
			if conn, ok := e.routeTable.id[id]; ok {
				return conn, nil
			} else {
				peerChan := make(chan Payload, ChanSize)
				e.routeTable.addr[dst] = peerChan
				dev := &devWrapper{w: e.devWriter, r: peerChan}
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
	}
	return nil, errors.New(fmt.Sprintf("unknown dst addr: %s", dst.String()))
}
