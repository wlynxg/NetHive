package command

import (
	"fmt"
	"net/netip"
)

func IPAddr(addr netip.Addr, dev string) error {
	cmd := fmt.Sprintf("ip addr add %s dev %s", addr.String(), dev)
	_, _, err := Bash(cmd)
	return err
}
