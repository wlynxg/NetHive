package win

import "unsafe"

// MibUnicastIPAddressRow structure stores information about a unicast IP address.
// https://learn.microsoft.com/en-us/windows/win32/api/netioapi/ns-netioapi-mib_unicastipaddress_row
type MibUnicastIPAddressRow struct {
	Address            SockAddrInet
	InterfaceLUID      uint64
	InterfaceIndex     uint32
	PrefixOrigin       uint32
	SuffixOrigin       uint32
	ValidLifetime      uint32
	PreferredLifetime  uint32
	OnLinkPrefixLength uint8
	SkipAsSource       bool
	DadState           NLDadState
	ScopeID            uint32
	CreationTimeStamp  int64
}

func (m *MibUnicastIPAddressRow) Init() error {
	return initializeUnicastIPAddressEntry(m)
}

func (m *MibUnicastIPAddressRow) Delete() error {
	return deleteUnicastIPAddressEntry(m)
}

type MibUnicastIPAddressTable struct {
	NumEntries uint32
	Table      [1]MibUnicastIPAddressRow
}

func (m *MibUnicastIPAddressTable) Init() error {
	return getUnicastIPAddressTable(m)
}

func (m *MibUnicastIPAddressTable) Rows() []MibUnicastIPAddressRow {
	return unsafe.Slice(&m.Table[0], m.NumEntries)
}

func (m *MibUnicastIPAddressTable) Free() {
	freeMibTable(unsafe.Pointer(m))
}
