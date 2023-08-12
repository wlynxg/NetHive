package win

import (
	"syscall"
)

func be2se(i uint16) uint16 {
	return (i>>8)&0xFF | (i&0xFF)<<8
}

func se2be(i uint16) uint16 {
	return (i>>8)&0xFF | (i&0xFF)<<8
}

func SyscallOnlyReturnError(trap uintptr, args ...uintptr) error {
	r0, _, _ := syscall.SyscallN(trap, args...)
	if r0 != 0 {
		return syscall.Errno(r0)
	}
	return nil
}
