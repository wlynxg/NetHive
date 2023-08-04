package system

import (
	"os"
	"syscall"
)

func Ioctl(fd uintptr, request uintptr, argp uintptr) error {
	_, _, err := syscall.Syscall(syscall.SYS_IOCTL, fd, request, argp)
	if err != 0 {
		return os.NewSyscallError("ioctl: ", err)
	}
	return nil
}
