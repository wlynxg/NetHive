package win

import (
	"net/netip"
	"strconv"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

type SockAddrInet struct {
	Family uint16
	data   [26]byte
}

func (s *SockAddrInet) SetAddrPort(ap netip.AddrPort) error {
	if ap.Addr().Is4() {
		addr4 := (*syscall.RawSockaddrInet4)(unsafe.Pointer(s))
		addr4.Family = syscall.AF_INET
		addr4.Addr = ap.Addr().As4()
		addr4.Port = be2se(ap.Port())
		for i := 0; i < 8; i++ {
			addr4.Zero[i] = 0
		}
		return nil
	} else if ap.Addr().Is6() {
		addr6 := (*windows.RawSockaddrInet6)(unsafe.Pointer(s))
		addr6.Family = syscall.AF_INET6
		addr6.Addr = ap.Addr().As16()
		addr6.Port = be2se(ap.Port())
		addr6.Flowinfo = 0
		scopeId := uint32(0)
		if z := ap.Addr().Zone(); z != "" {
			if s, err := strconv.ParseUint(z, 10, 32); err == nil {
				scopeId = uint32(s)
			}
		}
		addr6.Scope_id = scopeId
		return nil
	}
	return windows.ERROR_INVALID_PARAMETER
}
