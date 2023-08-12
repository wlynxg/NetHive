package win

import (
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modiphlpapi = windows.NewLazySystemDLL("iphlpapi.dll")

	procInitializeUnicastIpAddressEntry = modiphlpapi.NewProc("InitializeUnicastIpAddressEntry")
	procGetUnicastIpAddressTable        = modiphlpapi.NewProc("GetUnicastIpAddressTable")
	procDeleteUnicastIpAddressEntry     = modiphlpapi.NewProc("DeleteUnicastIpAddressEntry")
	procFreeMibTable                    = modiphlpapi.NewProc("FreeMibTable")
)

func initializeUnicastIPAddressEntry(row *MibUnicastIPAddressRow) error {
	return SyscallOnlyReturnError(procInitializeUnicastIpAddressEntry.Addr(), 1, uintptr(unsafe.Pointer(row)))
}

func getUnicastIPAddressTable(table *MibUnicastIPAddressTable) error {
	return SyscallOnlyReturnError(procGetUnicastIpAddressTable.Addr(), 2, 0, uintptr(unsafe.Pointer(table)))
}

func deleteUnicastIPAddressEntry(row *MibUnicastIPAddressRow) (ret error) {
	return SyscallOnlyReturnError(procDeleteUnicastIpAddressEntry.Addr(), 1, uintptr(unsafe.Pointer(row)))
}

func freeMibTable(memory unsafe.Pointer) {
	syscall.SyscallN(procFreeMibTable.Addr(), 1, uintptr(memory))
}
