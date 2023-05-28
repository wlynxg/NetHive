package device

import (
	"NetHive/pkgs/command"
	"bytes"
	"fmt"
	"net/netip"
	"os"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

// https://man7.org/linux/man-pages/man7/netdevice.7.html
type ifReq [40]byte

// https://man7.org/linux/man-pages/man2/ioctl.2.html
func ioctl(fd uintptr, request uintptr, argp uintptr) error {
	_, _, err := unix.Syscall(unix.SYS_IOCTL, fd, request, argp)
	if err != 0 {
		return os.NewSyscallError("ioctl: ", err)
	}
	return nil
}

// compilation time interface check
var _ Device = new(tun)

const (
	cloneDevicePath = "/dev/net/tun"
)

type tun struct {
	name      string
	mtu       int
	cacheTime time.Time
	index     int32

	// flock is used for file usage locks
	flock   sync.Mutex
	tunFile *os.File
}

func (t *tun) Read(buff []byte) (int, error) {
	t.flock.Lock()
	defer t.flock.Unlock()
	return t.tunFile.Read(buff)
}

func (t *tun) Write(buff []byte) (int, error) {
	t.flock.Lock()
	defer t.flock.Unlock()
	return t.tunFile.Write(buff)
}

func (t *tun) Close() error {
	return t.tunFile.Close()
}

func (t *tun) MTU() (int, error) {
	if time.Now().After(t.cacheTime) {
		return t.getMTUFromSys()
	}
	return t.mtu, nil
}

func (t *tun) Name() (string, error) {
	if time.Now().After(t.cacheTime) {
		return t.getNameFromSys()
	}
	return t.name, nil
}

func (t *tun) AddAddress(addr netip.Addr) error {
	return command.IPAddr(addr, t.name)
}

func (t *tun) getNameFromSys() (string, error) {
	t.flock.Lock()
	defer t.flock.Unlock()

	conn, err := t.tunFile.SyscallConn()
	if err != nil {
		return "", err
	}

	var ifr ifReq
	var errno syscall.Errno
	err = conn.Control(func(fd uintptr) {
		ioctl(fd, unix.TUNGETIFF, uintptr(unsafe.Pointer(&ifr[0])))
	})
	if err != nil || errno != 0 {
		return "", fmt.Errorf("failed to get name of TUN device: %w", err)
	}

	name := ifr[:]
	if i := bytes.IndexByte(name, 0); i != -1 {
		name = name[:i]
	}
	t.name = string(name[:])
	return t.name, nil
}

func (t *tun) getMTUFromSys() (int, error) {
	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_DGRAM|unix.SOCK_CLOEXEC, 0)
	if err != nil {
		return 0, err
	}
	defer unix.Close(fd)

	var ifr ifReq
	copy(ifr[:], t.name)
	err = ioctl(uintptr(fd), unix.SIOCGIFMTU, uintptr(unsafe.Pointer(&ifr[0])))
	if err != nil {
		return -1, err
	}
	return int(*(*int32)(unsafe.Pointer(&ifr[unix.IFNAMSIZ]))), nil
}

func (t *tun) setMTU(n int) error {
	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_DGRAM|unix.SOCK_CLOEXEC, 0)
	if err != nil {
		return err
	}
	defer unix.Close(fd)

	var ifr ifReq
	copy(ifr[:], t.name)
	*(*uint32)(unsafe.Pointer(&ifr[unix.IFNAMSIZ])) = uint32(n)
	err = ioctl(uintptr(fd), unix.SIOCSIFMTU, uintptr(unsafe.Pointer(&ifr[0])))
	if err != nil {
		return err
	}
	return nil
}

func (t *tun) getIFIndex() (int32, error) {
	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_DGRAM|unix.SOCK_CLOEXEC, 0)
	if err != nil {
		return 0, err
	}
	defer unix.Close(fd)

	var ifr ifReq
	copy(ifr[:], t.name)
	err = ioctl(uintptr(fd), unix.SIOCGIFINDEX, uintptr(unsafe.Pointer(&ifr[0])))
	if err != nil {
		return 0, err
	}
	return *(*int32)(unsafe.Pointer(&ifr[unix.IFNAMSIZ])), nil
}

func CreateTUN(name string, mtu int) (Device, error) {
	tfd, err := unix.Open(cloneDevicePath, unix.O_RDWR|unix.O_CLOEXEC, 0)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("CreateTUN(%q) failed; %s does not exist", name, cloneDevicePath)
		}
		return nil, err
	}

	ifreq, err := unix.NewIfreq(name)
	if err != nil {
		return nil, err
	}

	// unix.IFF_TUN: TUN device
	// unix.IFF_NO_PI: no need to provide package information
	ifreq.SetUint16(unix.IFF_TUN | unix.IFF_NO_PI)
	err = unix.IoctlIfreq(tfd, unix.TUNSETIFF, ifreq)
	if err != nil {
		return nil, err
	}

	// set the current file descriptor to non-blocking status to improve concurrency
	err = unix.SetNonblock(tfd, true)
	if err != nil {
		return nil, err
	}

	file := os.NewFile(uintptr(tfd), cloneDevicePath)

	d := &tun{
		tunFile:   file,
		cacheTime: time.Now(),
	}

	_, err = d.getNameFromSys()
	if err != nil {
		return nil, err
	}

	d.index, err = d.getIFIndex()
	if err != nil {
		return nil, err
	}

	err = d.setMTU(mtu)
	if err != nil {
		return nil, err
	}

	d.mtu, err = d.getMTUFromSys()
	if err != nil {
		return nil, err
	}
	return d, nil
}
