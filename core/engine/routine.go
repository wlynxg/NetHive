package engine

import (
	"fmt"
	"github.com/wlynxg/NetHive/core/protocol"
	"net/netip"
)

// RoutineTUNReader loop to read packets from TUN
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

		if (ip.Dst().IsLinkLocalMulticast() || ip.Dst().IsMulticast()) && !e.cfg.EnableBroadcast {
			e.log.Debugf("discard broadcast packets: %s -> %s", ip.Src(), ip.Dst())
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

// RoutineTUNWriter loop writing packets to TUN
func (e *Engine) RoutineTUNWriter() {
	var (
		payload Payload
		err     error
	)

	for payload = range e.devWriter {
		_, err = e.device.Write(payload.Data)
		if err != nil {
			e.log.Errorf("[RoutineTUNWriter]: %s", err)
			e.log.Errorf("[err packet]: %v", payload.Data)
		}
	}
}

// RoutineRouteTableWriter loop sending the data packet to the corresponding channel according to the routing table
func (e *Engine) RoutineRouteTableWriter() {
	var (
		payload Payload
		ok      bool
		conn    PacketChan
	)

	for payload = range e.devReader {
		if (payload.Dst.IsLinkLocalMulticast() || payload.Dst.IsMulticast()) && e.cfg.EnableBroadcast {
			e.routeTable.m.Range(func(key string, value netip.Prefix) bool {
				conn, ok := e.routeTable.id.Load(key)
				if !ok {
					conn := make(PacketChan, ChanSize)
					e.routeTable.id.Store(key, conn)
					e.routeTable.addr.Store(value.Addr(), conn)
					go func() {
						defer e.routeTable.id.Delete(key)
						defer e.routeTable.addr.Delete(value.Addr())
						e.addConn(conn, key)
					}()

				}
				select {
				case conn <- payload:
				default:
					e.log.Warnf("[RoutineRouteTableWriter] drop packet: %s, because the sending queue is already full", payload.Dst)
				}
				return true
			})
			continue
		}

		conn, ok = e.routeTable.addr.Load(payload.Dst)
		if !ok {
			c, err := e.addConnByDst(payload.Dst)
			if err != nil {
				e.log.Warnf("[RoutineRouteTableWriter] drop packet: %s, because %s", payload.Dst, err)
				continue
			}
			conn = c
		}

		if conn == nil {
			continue
		}

		select {
		case conn <- payload:
		default:
			e.log.Warnf("[RoutineRouteTableWriter] drop packet: %s, because the sending queue is already full", payload.Dst)
		}
	}
}
