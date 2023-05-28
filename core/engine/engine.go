package engine

import (
	"NetHive/core/conn"
	"NetHive/core/control"
	"NetHive/core/device"
	"NetHive/core/info"
	"context"
	"fmt"
	"net/netip"
	"time"
)

const (
	BuffSize = 1024
)

type Engine struct {
	ctx    context.Context
	option Option
	// tun device
	device device.Device
	// udp conn
	udpConn *conn.UdpConn
	// control
	control *control.Client

	devWriter  chan Payload
	connWriter chan Payload
	errChan    chan error
}

func (e *Engine) Start() error {
	ctx, cancel := context.WithCancel(context.Background())
	e.ctx = ctx
	defer cancel()

	opt := e.option

	// create tun
	tun, err := device.CreateTUN(opt.Device.TUNName, opt.Device.MTU)
	if err != nil {
		return err
	}
	e.device = tun

	// create udp connection
	addr, err := netip.ParseAddrPort(opt.UDPAddr)
	if err != nil {
		return err
	}
	udpConn, err := conn.NewUDPConn(addr)
	if err != nil {
		return err
	}
	e.udpConn = udpConn

	e.control = control.New(opt.Server)

	e.devWriter = make(chan Payload, 100)
	e.connWriter = make(chan Payload, 100)

	go e.RoutineTUNReader()
	go e.RoutineTUNWriter()
	go e.RoutineConnReader()
	go e.RoutineConnWriter()
	go e.RoutineConnect()

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

func (e *Engine) RoutineConnect() {
	var (
		nodes []info.NodeInfo
		err   error
	)

	interval := e.option.Interval
	timer := time.NewTimer(interval)
	for {
		select {
		case <-e.ctx.Done():
			return
		case <-timer.C:
			nodes, err = e.control.Connect(e.ctx, *info.New())
			if err != nil {
				return
			}
			fmt.Printf("%+v\n", nodes)
		}
	}

}
