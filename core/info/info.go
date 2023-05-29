package info

import (
	"net/netip"
	"os"
	"runtime"
)

type NodeInfo struct {
	Endpoints []netip.AddrPort
	Hostname  string
	OS        string
	Arch      string

	Address netip.Prefix
}

func New() *NodeInfo {
	hostname, _ := os.Hostname()
	return &NodeInfo{
		Hostname: hostname,
		OS:       OS(),
		Arch:     runtime.GOARCH,
	}
}
