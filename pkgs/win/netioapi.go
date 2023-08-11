package win

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

func (m *MibUnicastIPAddressRow) Init() {
	initializeUnicastIPAddressEntry(m)
}
