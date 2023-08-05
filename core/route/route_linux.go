package route

import (
	"NetHive/pkgs/system"
	"net"
	"net/netip"
	"syscall"
	"unsafe"
)

type RtEntry struct {
	_       uint64
	Dst     syscall.RawSockaddrInet4
	Gateway syscall.RawSockaddrInet4
	GenMask syscall.RawSockaddrInet4
	Flags   uint16
	_       uint16
	_       uint64
	Tos     byte
	Class   byte
	_       [3]uint16
	Metric  int16
	Dev     uintptr
	Mtu     uint64
	Window  uint64
	IRtt    uint16
}

func Add(dev string, target netip.Prefix) error {
	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_DGRAM, 0)
	if err != nil {
		return err
	}
	defer syscall.Close(fd)

	_, cidr, err := net.ParseCIDR(target.String())
	if err != nil {
		return err
	}
	mask := cidr.Mask

	rt := RtEntry{}
	rt.Dst = syscall.RawSockaddrInet4{Family: syscall.AF_INET, Addr: target.Addr().As4()}
	rt.GenMask = syscall.RawSockaddrInet4{Family: syscall.AF_INET, Addr: [4]byte{mask[0], mask[1], mask[2], mask[3]}}
	rt.Flags = syscall.RTF_UP
	devName := [syscall.IFNAMSIZ]byte{}
	copy(devName[:], dev)
	rt.Dev = uintptr(unsafe.Pointer(&devName))
	rtBytes := *(*[unsafe.Sizeof(rt)]byte)(unsafe.Pointer(&rt))

	err = system.Ioctl(uintptr(fd), syscall.SIOCADDRT, uintptr(unsafe.Pointer(&rtBytes[0])))
	if err != nil {
		return err
	}
	return nil
}

func Del(target netip.Prefix) error {
	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_DGRAM, 0)
	if err != nil {
		return err
	}
	defer syscall.Close(fd)

	_, cidr, err := net.ParseCIDR(target.String())
	if err != nil {
		return err
	}
	mask := cidr.Mask

	rt := RtEntry{}
	rt.Dst = syscall.RawSockaddrInet4{Family: syscall.AF_INET, Addr: target.Addr().As4()}
	rt.GenMask = syscall.RawSockaddrInet4{Family: syscall.AF_INET, Addr: [4]byte{mask[0], mask[1], mask[2], mask[3]}}
	rtBytes := *(*[unsafe.Sizeof(rt)]byte)(unsafe.Pointer(&rt))

	err = system.Ioctl(uintptr(fd), syscall.SIOCDELRT, uintptr(unsafe.Pointer(&rtBytes[0])))
	if err != nil {
		return err
	}
	return nil
}
