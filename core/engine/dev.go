package engine

import (
	"io"
)

var _ io.ReadWriteCloser = (*devWrapper)(nil)

type devWrapper struct {
	w PacketChan
	r PacketChan
}

func (c *devWrapper) Read(p []byte) (n int, err error) {
	packet := <-c.r
	copy(p, packet.Data)
	return len(packet.Data), nil
}

func (c *devWrapper) Write(p []byte) (n int, err error) {
	buff := make([]byte, len(p))
	copy(buff, p)
	c.w <- Payload{Data: buff}
	return len(buff), nil
}

func (c *devWrapper) Close() error {
	return nil
}
