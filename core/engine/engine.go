package engine

import (
	"NetHive/core/conn"
	"NetHive/core/device"
)

type Engine struct {
	device  device.Device
	udpConn conn.UdpConn
}
