package device

import (
	"NetHive/pkgs/win"
	"net/netip"
	"runtime"

	"github.com/pkg/errors"
	"golang.org/x/sys/windows"
	"golang.zx2c4.com/wintun"
)

const (
	WintunTunnelType = "NetHive"
)

type tun struct {
	adapter *wintun.Adapter
	session *wintun.Session
	name    string
	mtu     int
}

func (t *tun) Read(buff []byte) (int, error) {
	for {
		packet, err := t.session.ReceivePacket()
		switch err {
		case nil:
			size := len(packet)
			copy(buff, packet)
			t.session.ReleaseReceivePacket(packet)
			return size, nil
		case windows.ERROR_NO_MORE_FILES:
			runtime.Gosched()
			continue
		case windows.ERROR_INVALID_DATA:
			return 0, errors.New("Send ring corrupt")
		default:
			return 0, errors.Errorf("Read failed: %v", err)
		}
	}
}

func (t *tun) Write(bytes []byte) (int, error) {
	//TODO implement me
	panic("implement me")
}

func (t *tun) Close() error {
	return t.adapter.Close()
}

func (t *tun) MTU() (int, error) {
	return t.mtu, nil
}

func (t *tun) Name() (string, error) {
	return t.name, nil
}

func (t *tun) AddAddress(addr netip.Prefix) error {
	luid := t.adapter.LUID()
	itf := win.NetItf(luid)
	return errors.Wrap(itf.AddIPAddress(addr), "Error AddAddress:")
}

func (t *tun) FlushAddress() error {
	luid := t.adapter.LUID()
	itf := win.NetItf(luid)
	return errors.Wrap(itf.FlushAddress(), "Error FlushAddress:")
}

func (t *tun) Up() error {
	//TODO implement me
	panic("implement me")
}

func (t *tun) Down() error {
	//TODO implement me
	panic("implement me")
}

func (t *tun) State() bool {
	//TODO implement me
	panic("implement me")
}

func CreateTUN(name string, mtu int) (Device, error) {
	adapter, err := wintun.CreateAdapter(name, WintunTunnelType, nil)
	if err != nil {
		return nil, errors.Wrapf(err, "error creating interface: ")
	}

	session, err := adapter.StartSession(0x800000)
	if err != nil {
		adapter.Close()
		return nil, errors.Wrapf(err, "error start session: ")
	}

	if mtu <= 0 {
		mtu = 1420
	}

	t := &tun{
		adapter: adapter,
		session: &session,
		name:    name,
		mtu:     mtu,
	}

	return t, nil
}
