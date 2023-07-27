package engine

import (
	"NetHive/core/device"
	"context"
	"fmt"

	"github.com/libp2p/go-libp2p/core/host"
)

const (
	BuffSize = 1024
)

type Engine struct {
	ctx    context.Context
	option Option
	// tun device
	device device.Device

	host host.Host

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

	e.devWriter = make(chan Payload, 100)
	e.connWriter = make(chan Payload, 100)

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
