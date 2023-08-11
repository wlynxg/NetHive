package win

import (
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modiphlpapi = windows.NewLazySystemDLL("iphlpapi.dll")

	procInitializeUnicastIpAddressEntry = modiphlpapi.NewProc("InitializeUnicastIpAddressEntry")
)

func initializeUnicastIPAddressEntry(row *MibUnicastIPAddressRow) {
	syscall.SyscallN(procInitializeUnicastIpAddressEntry.Addr(), 1, uintptr(unsafe.Pointer(row)), 0, 0)
}
