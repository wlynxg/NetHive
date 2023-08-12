package win

import (
	"net/netip"
)

// NetItf represents the network interface UUID of windows.
type NetItf uint64

func (i NetItf) AddIPAddress(address netip.Prefix) error {
	row := &MibUnicastIPAddressRow{}
	if err := row.Init(); err != nil {
		return err
	}
	if err := row.Address.SetAddrPort(netip.AddrPortFrom(address.Addr(), 0)); err != nil {
		return err
	}
	row.InterfaceLUID = uint64(i)
	row.ValidLifetime = 0xffffffff
	row.PreferredLifetime = 0xffffffff
	row.OnLinkPrefixLength = uint8(address.Bits())
	row.DadState = NldsPreferred
	return nil
}

func (i NetItf) FlushAddress() error {
	tab := &MibUnicastIPAddressTable{}

	if err := tab.Init(); err != nil {
		return err
	}

	for _, row := range tab.Rows() {
		if row.InterfaceLUID == uint64(i) {
			row.Delete()
		}
	}
	tab.Free()
	return nil
}
