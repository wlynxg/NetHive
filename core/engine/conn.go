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
			if value.Addr().Compare(dst) != 0 {
				return true
			}

			if c, ok := e.routeTable.id.Load(key); ok {
				conn = c
			} else {
				conn = make(PacketChan, ChanSize)
				e.routeTable.addr.Store(dst, conn)
				go func() {
					defer e.routeTable.id.Delete(key)
					defer e.routeTable.addr.Delete(dst)
					e.addConn(conn, key)
				}()
			}
			return false
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
		return
	}

	pch := e.SearchNode(e.ctx, peer.ID(idr))
	for info := range pch {
		err := e.host.Connect(e.ctx, info)
		if err != nil {
			continue
		}

		stream, err = e.host.NewStream(e.ctx, info.ID, VPNStreamProtocol)
		if err != nil || stream == nil {
			continue
		}
		break
	}

	if stream == nil {
		return
	}

	e.log.Infof("successfully connect [%s] by %s", id, stream.Conn().RemoteMultiaddr())
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
