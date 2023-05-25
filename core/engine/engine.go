package engine

import (
	"NetHive/core/conn"
	"NetHive/core/device"
	"fmt"
	"net/netip"
)

const (
	BuffSize = 1024
)

type Engine struct {
	option     Option
	device     device.Device
	udpConn    *conn.UdpConn
	devWriter  chan Payload
	connWriter chan Payload
	errChan    chan error
}

func (e *Engine) Start() error {
	// create tun
	tun, err := device.CreateTUN(e.option.Device.TUNName, e.option.Device.MTU)
	if err != nil {
		return err
	}
	e.device = tun

	// create udp connection
	addr, err := netip.ParseAddrPort(e.option.UDPAddr)
	if err != nil {
		return err
	}
	udpConn, err := conn.NewUDPConn(addr)
	if err != nil {
		return err
	}
	e.udpConn = udpConn

	e.devWriter = make(chan Payload, 100)
	e.connWriter = make(chan Payload, 100)

	go e.RoutineTUNReader()
	go e.RoutineTUNWriter()
	go e.RoutineConnReader()
	go e.RoutineConnWriter()

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
		e.connWriter <- payload
	}
}

func (e *Engine) RoutineTUNWriter() {
	var (
		payload Payload
		err     error
	)

	for payload = range e.connWriter {
		_, err = e.device.Write(payload.Data)
		if err != nil {
			e.errChan <- fmt.Errorf("[RoutineTUNWriter]: %s", err)
			return
		}
	}
}

func (e *Engine) RoutineConnReader() {
	var (
		payload Payload
		buff    = make([]byte, BuffSize)
		err     error
		n       int
		addr    netip.AddrPort
	)

	for {
		n, addr, err = e.udpConn.ReadFrom(buff)
		if err != nil {
			e.errChan <- fmt.Errorf("[RoutineConnReader]: %s", err)
			return
		}
		copy(payload.Data, buff[:n])
		payload.Addr = addr
		e.devWriter <- payload
	}
}

func (e *Engine) RoutineConnWriter() {
	var (
		payload Payload
		err     error
	)

	for payload = range e.devWriter {
		_, err = e.udpConn.WriteTo(payload.Data, payload.Addr)
		if err != nil {
			e.errChan <- fmt.Errorf("[RoutineTUNWriter]: %s", err)
			return
		}
	}
}
