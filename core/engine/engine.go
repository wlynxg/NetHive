package engine

import (
	"NetHive/core/device"
	"NetHive/core/protocol"
	"NetHive/pkgs/xsync"
	"context"
	"errors"
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
	"github.com/mr-tron/base58"
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
		m      xsync.Map[peer.ID, []netip.Prefix]
		prefix xsync.Map[netip.Prefix, peer.ID]
		id     xsync.Map[peer.ID, PacketChan]
		addr   xsync.Map[netip.Addr, PacketChan]
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
	e.routeTable.prefix = xsync.Map[netip.Prefix, peer.ID]{}
	e.routeTable.id = xsync.Map[peer.ID, PacketChan]{}
	e.routeTable.addr = xsync.Map[netip.Addr, PacketChan]{}

	for id, prefixes := range e.opt.PeersRouteTable {
		e.routeTable.m.Store(id, prefixes)
		for _, prefix := range prefixes {
			e.routeTable.prefix.Store(prefix, id)
		}
	}

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
	e.mdns = mdns.NewMdnsService(e.host, "_net._hive", e)

	return e, nil
}

func (e *Engine) Start() error {
	defer e.cancel()
	opt := e.opt

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
		defer stream.Close()
		_, err = io.Copy(dev, stream)
		if err != nil && err != io.EOF {
			e.log.Errorf(e.ctx, "Peer [%s] stream read error: %s", string(id), err)
		}
	})

	util.Advertise(e.ctx, e.discovery, e.host.ID().String())
	go func() {
		for {
			peers, err := e.dht.GetClosestPeers(e.ctx, string(e.host.ID()))
			if err != nil {
				e.log.Warningf(e.ctx, "Failed to get nearest node: %s", err)
				continue
			}

			for _, id := range peers {
				e.relayChan <- e.host.Peerstore().PeerInfo(id)
			}
			time.Sleep(10 * time.Minute)
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
			Src: ip.Src(),
			Dst: ip.Dst(),
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
		fmt.Println(payload.Dst)
		var conn PacketChan
		c, ok := e.routeTable.addr.Load(payload.Dst)
		if ok {
			conn = c
		} else {
			c, err := e.addConn(payload.Dst)
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

func (e *Engine) addConn(dst netip.Addr) (PacketChan, error) {
	e.log.Debugf(e.ctx, "Try to connect to the corresponding node of %s", dst)

	var conn PacketChan
	e.routeTable.prefix.Range(func(prefix netip.Prefix, id peer.ID) bool {
		if !prefix.Contains(dst) {
			return true
		}

		if c, ok := e.routeTable.id.Load(id); ok {
			conn = c
			return false
		}
		peerChan := make(chan Payload, ChanSize)
		conn = peerChan
		e.routeTable.addr.Store(dst, peerChan)

		go func() {
			defer e.routeTable.addr.Delete(dst)

			dev := &devWrapper{w: e.devWriter, r: peerChan}
			e.log.Infof(e.ctx, "start find peer %s", string(id))
			peerc, err := e.discovery.FindPeers(e.ctx, string(id))
			if err != nil {
				e.log.Warningf(e.ctx, "Finding node by dht %s failed because %s", string(id), err)
			}

			var peers []peer.AddrInfo
			for info := range peerc {
				if info.ID.String() == string(id) && len(info.Addrs) > 0 {
					peers = append(peers, info)
				}
			}

			if idr, err := base58.Decode(string(id)); err == nil {
				info := e.host.Peerstore().PeerInfo(peer.ID(idr))
				if len(info.Addrs) > 0 {
					peers = append(peers, info)
				}
			}

			var stream network.Stream
			for _, info := range peers {
				e.log.Infof(e.ctx, "find peer: %s", info)
				stream, err = e.host.NewStream(e.ctx, info.ID, VPNStreamProtocol)
				if err == nil {
					break
				}
				e.log.Warningf(e.ctx, "Connection establishment with node %s failed due to %s", info, err)
			}
			e.log.Infof(e.ctx, "Peer [%s] connect success", string(id))

			go func() {
				defer stream.Close()
				_, err := io.Copy(stream, dev)
				if err != nil && err != io.EOF {
					e.log.Errorf(e.ctx, "Peer [%s] stream write error: %s", string(id), err)
				}
			}()

			defer stream.Close()
			_, err = io.Copy(dev, stream)
			if err != nil && err != io.EOF {
				e.log.Errorf(e.ctx, "Peer [%s] stream read error: %s", string(id), err)
			}
		}()
		return false
	})

	if conn != nil {
		return conn, nil
	}

	return nil, errors.New(fmt.Sprintf("unknown dst addr: %s", dst.String()))
}
