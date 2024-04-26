package engine

import (
	"fmt"

	"github.com/wlynxg/NetHive/core/protocol"
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
			e.log.Error(e.ctx, fmt.Errorf("[RoutineTUNWriter]: %s", err))
			return
		}
	}
}

// RoutineRouteTableWriter loop sending the data packet to the corresponding channel according to the routing table
func (e *Engine) RoutineRouteTableWriter() {
	var (
		payload Payload
	)

	for payload = range e.devReader {
		var conn PacketChan

		e.log.Debugf("%s -> %s", payload.Src, payload.Dst)

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
