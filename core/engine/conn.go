package engine

import (
	"errors"
	"fmt"
	"io"
	"net/netip"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/mr-tron/base58/base58"
	"go4.org/netipx"
)

func (e *Engine) addConnByDst(dst netip.Addr) (PacketChan, error) {
	e.log.Debugf(e.ctx, "Try to connect to the corresponding node of %s", dst)

	var conn PacketChan
	e.routeTable.set.Range(func(id peer.ID, prefix *netipx.IPSet) bool {
		if !prefix.Contains(dst) {
			return true
		}

		if c, ok := e.routeTable.id.Load(id); ok {
			conn = c
			e.routeTable.addr.Store(dst, c)
			return false
		}
		peerChan := make(chan Payload, ChanSize)
		conn = peerChan
		e.routeTable.addr.Store(dst, peerChan)

		go func() {
			defer e.routeTable.addr.Delete(dst)
			e.addConn(peerChan, id)
		}()
		return false
	})

	if conn != nil {
		return conn, nil
	}

	return nil, errors.New(fmt.Sprintf("the routing rule corresponding to %s was not found", dst.String()))
}

func (e *Engine) addConnByID(id peer.ID) (PacketChan, error) {
	e.log.Debugf(e.ctx, "Try to connect to the corresponding node of %s", string(id))

	if conn, ok := e.routeTable.id.Load(id); ok {
		return conn, nil
	}

	peerChan := make(chan Payload, ChanSize)
	e.routeTable.id.Store(id, peerChan)

	go func() {
		defer e.routeTable.id.Delete(id)
		e.addConn(peerChan, id)
	}()

	return nil, errors.New(fmt.Sprintf("unknown dst addr: %s", string(id)))
}

func (e *Engine) addConn(peerChan PacketChan, id peer.ID) {
	dev := &devWrapper{w: e.devWriter, r: peerChan}
	e.log.Infof(e.ctx, "start find peer %s", string(id))

	var (
		stream network.Stream
		err    error
	)

	idr, err := base58.Decode(string(id))
	if err != nil {
		e.log.Infof(e.ctx, "base58 decode failed: %s", err)
	}

	info := e.host.Peerstore().PeerInfo(peer.ID(idr))
	if len(info.Addrs) > 0 {
		stream, err = e.host.NewStream(e.ctx, info.ID, VPNStreamProtocol)
		if err != nil {
			peerc, err := e.discovery.FindPeers(e.ctx, string(id))
			if err != nil {
				e.log.Warningf(e.ctx, "Finding node by dht %s failed because %s", string(id), err)
				return
			}

			for info := range peerc {
				if info.ID.String() == string(id) && len(info.Addrs) > 0 {
					stream, err = e.host.NewStream(e.ctx, info.ID, VPNStreamProtocol)
					if err == nil {
						break
					}
				}
			}
			e.log.Warningf(e.ctx, "Connection establishment with node %s failed", string(id))
			return
		}
	}

	if stream == nil {
		return
	}

	e.log.Infof(e.ctx, "Peer [%s] connect success", string(id))
	defer stream.Close()

	go func() {
		defer stream.Close()
		_, err := io.Copy(stream, dev)
		if err != nil && err != io.EOF {
			e.log.Errorf(e.ctx, "Peer [%s] stream write error: %s", string(id), err)
		}
	}()

	_, err = io.Copy(dev, stream)
	if err != nil && err != io.EOF {
		e.log.Errorf(e.ctx, "Peer [%s] stream read error: %s", string(id), err)
	}
}
