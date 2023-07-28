package engine

import (
	"NetHive/core/device"
	"context"
	"fmt"
	"io"
	"net/netip"
	"sync"

	"github.com/gogf/gf/v2/os/glog"
	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/routing"
)

const (
	BuffSize          = 1500
	ChanSize          = 1500
	VPNStreamProtocol = "/NetHive/vpn"
)

type Engine struct {
	log *glog.Logger
	ctx context.Context
	opt Option
	// tun device
	device device.Device

	host      host.Host
	dht       *dht.IpfsDHT
	discovery *routing.RoutingDiscovery

	relayChan chan peer.AddrInfo

	devWriter chan Payload
	devReader chan Payload
	errChan   chan error

	routeTable struct {
		sync.RWMutex
		m map[netip.Prefix]chan []byte
	}
}

func New(opt *Option) (*Engine, error) {
	var (
		e = new(Engine)
	)

	return e, nil
}

func (e *Engine) Start() error {
	ctx, cancel := context.WithCancel(context.Background())
	e.ctx = ctx
	defer cancel()

	opt := e.opt

	// create tun
	tun, err := device.CreateTUN(opt.Device.TUNName, opt.Device.MTU)
	if err != nil {
		return err
	}
	e.device = tun

	e.devWriter = make(chan Payload, ChanSize)
	e.devReader = make(chan Payload, ChanSize)
	e.relayChan = make(chan peer.AddrInfo, ChanSize)

	node, err := libp2p.New(libp2p.EnableAutoRelayWithPeerSource(func(ctx context.Context, num int) <-chan peer.AddrInfo { return e.relayChan }))
	if err != nil {
		return err
	}
	e.host = node

	e.dht, err = dht.New(e.ctx, e.host)
	if err != nil {
		return err
	}
	e.discovery = routing.NewRoutingDiscovery(e.dht)
	e.routeTable.m = make(map[netip.Prefix]chan []byte)

	wg := sync.WaitGroup{}
	errChan := make(chan error, len(opt.PeersRouteTable))
	for id, prefixes := range opt.PeersRouteTable {
		peerChan := make(chan []byte, ChanSize)
		e.routeTable.Lock()
		for _, prefix := range prefixes {
			e.routeTable.m[prefix] = peerChan
		}
		e.routeTable.Unlock()

		wg.Add(1)
		id := id
		go func() {
			peers, err := e.discovery.FindPeers(e.ctx, string(id))
			if err != nil {
				errChan <- err
			}

			for info := range peers {
				if info.ID != id || len(info.Addrs) <= 0 {
					continue
				}

				stream, err := e.host.NewStream(e.ctx, info.ID, VPNStreamProtocol)
				if err != nil {
					errChan <- err
				}

				// TODO: 封装 channel
				go func() {
					io.Copy(stream, nil)
				}()
				io.Copy(nil, stream)
			}
		}()
	}

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

chooseRoute:
	for payload = range e.devReader {
		e.routeTable.RLock()
		// TODO: 增加缓存机制
		for prefix, c := range e.routeTable.m {
			if prefix.Contains(payload.Addr) {
				c <- payload.Data
				continue chooseRoute
			}
		}
		e.log.Warningf(e.ctx, "[RoutineRouteTableWriter] drop packet: %s", payload.Addr)
		e.routeTable.RUnlock()
	}
}
