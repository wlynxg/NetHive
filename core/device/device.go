package device

import (
	"net/netip"
)

type Device interface {
	// Read data packets from network device
	Read([]byte) (int, error)
	// Write data packets to network device
	Write([]byte) (int, error)
	// Close device
	Close() error
	// MTU return device's mtu
	MTU() (int, error)
	// Name return device's name
	Name() (string, error)
	// AddAddress add a address
	AddAddress(addr netip.Prefix) error
	// FlushAddress clear all address
	FlushAddress() error
}
