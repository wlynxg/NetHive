package system

import (
	"os"
	"syscall"
)

// Ioctl https://man7.org/linux/man-pages/man2/ioctl.2.html
func Ioctl(fd uintptr, request uintptr, argp uintptr) error {
	_, _, err := syscall.Syscall(syscall.SYS_IOCTL, fd, request, argp)
	if err != 0 {
		return os.NewSyscallError("ioctl", err)
	}
	return nil
}
