package engine

import (
	"errors"
	"fmt"
	"io"
	"net/netip"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/mr-tron/base58/base58"
)

func (e *Engine) addConnByDst(dst netip.Addr) (PacketChan, error) {
	e.log.Debugf("Try to connect to the corresponding node of %s", dst)

	var conn PacketChan
	if c, ok := e.routeTable.addr.Load(dst); ok {
		conn = c
	} else {
		e.routeTable.m.Range(func(key string, value netip.Prefix) bool {
			if value.Addr().Compare(dst) == 0 {
				conn = make(PacketChan, ChanSize)
				e.routeTable.addr.Store(dst, conn)
				go func() {
					defer e.routeTable.id.Delete(key)
					defer e.routeTable.addr.Delete(dst)
					e.addConn(conn, key)
				}()
				return false
			}
			return true
		})
	}

	if conn != nil {
		return conn, nil
	}

	return nil, errors.New(fmt.Sprintf("the routing rule corresponding to %s was not found", dst.String()))
}

func (e *Engine) addConnByID(id string) (PacketChan, error) {
	e.log.Debugf("Try to connect to the corresponding node of %s", id)

	if conn, ok := e.routeTable.id.Load(id); ok {
		return conn, nil
	}

	peerChan := make(chan Payload, ChanSize)
	e.routeTable.id.Store(id, peerChan)

	go func() {
		defer e.routeTable.id.Delete(id)
		e.addConn(peerChan, id)
	}()

	return nil, errors.New(fmt.Sprintf("unknown dst addr: %s", id))
}

func (e *Engine) addConn(peerChan PacketChan, id string) {
	dev := &devWrapper{w: e.devWriter, r: peerChan}
	e.log.Infof("start find peer %s", id)

	var (
		stream network.Stream
		err    error
	)

	idr, err := base58.Decode(id)
	if err != nil {
		e.log.Infof("base58 decode failed: %s", err)
	}

	info := e.host.Peerstore().PeerInfo(peer.ID(idr))
	if len(info.Addrs) > 0 {
		stream, err = e.host.NewStream(e.ctx, info.ID, VPNStreamProtocol)
		if err != nil {
			peerc, err := e.discovery.FindPeers(e.ctx, id)
			if err != nil {
				e.log.Warnf("Finding node by dht %s failed because %s", id, err)
				return
			}

			for info := range peerc {
				if info.ID.String() == id && len(info.Addrs) > 0 {
					stream, err = e.host.NewStream(e.ctx, info.ID, VPNStreamProtocol)
					if err == nil {
						break
					}
				}
			}
			e.log.Warnf("Connection establishment with node %s failed", id)
			return
		}
	}

	if stream == nil {
		return
	}

	e.log.Infof("Peer [%s] connect success", id)
	defer stream.Close()

	go func() {
		defer stream.Close()
		_, err := io.Copy(stream, dev)
		if err != nil && err != io.EOF {
			e.log.Errorf("Peer [%s] stream write error: %s", id, err)
		}
	}()

	_, err = io.Copy(dev, stream)
	if err != nil && err != io.EOF {
		e.log.Errorf("Peer [%s] stream read error: %s", id, err)
	}
}
