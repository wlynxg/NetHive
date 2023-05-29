package command

import (
	"fmt"
	"net/netip"
)

func IP4AddAddr(addr netip.Prefix, dev string) error {
	cmd := fmt.Sprintf("ip -4 addr add %s dev %s", addr.String(), dev)
	_, _, err := Bash(cmd)
	return err
}

func IP4FlushAddr(dev string) error {
	cmd := fmt.Sprintf("ip -4 addr flush dev %s", dev)
	_, _, err := Bash(cmd)
	return err
}
