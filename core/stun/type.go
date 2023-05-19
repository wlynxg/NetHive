package netcheck

import (
	"fmt"
	"net"
	"time"
)

type STUNRequest struct {
	MessageType   int16
	MessageLength int16
	MagicCookie   int32
	TransactionID [12]byte
}

type STUNRequestPacket struct {
	MessageType   [2]byte
	MessageLength [2]byte
	MagicCookie   [4]byte
	TransactionID [12]byte
}

type STUNResponse struct {
	MessageType   int
	MessageLength int
	MagicCookie   uint32
	TransactionID []byte
	Attributes    map[AttributeType]Attribute
}

type Attributes struct {
	MappedAddress    net.UDPAddr
	ResponseAddress  net.UDPAddr
	OtherAddress     net.UDPAddr
	XorMappedAddress net.UDPAddr
}

type Attribute struct {
	Type           AttributeType
	Length         int
	Reserved       int
	ProtocolFamily ProtocolFamily
	Port           int
	IP             net.IP
}

type AttributeType int

const (
	MappedAddress    AttributeType = 0x0001
	ChangeRequest    AttributeType = 0x0003
	SourceAddress    AttributeType = 0x0004
	ChangedAddress   AttributeType = 0x0005
	ResponseOrigin   AttributeType = 0x802b
	OtherAddress     AttributeType = 0x802c
	XorMappedAddress AttributeType = 0x0020
)

func (t AttributeType) String() string {
	switch t {
	case MappedAddress:
		return "MappedAddress"
	case ResponseOrigin:
		return "ResponseOrigin"
	case OtherAddress:
		return "OtherAddress"
	case XorMappedAddress:
		return "XorMappedAddress"
	default:
		return ""
	}
}

type ProtocolFamily int

const (
	ProtocolFamilyIPv4 ProtocolFamily = 1
)

func (p ProtocolFamily) String() string {
	switch p {
	case ProtocolFamilyIPv4:
		return "IPv4"
	default:
		return ""
	}
}

type ChangeRequestAction int

const (
	NoAction        ChangeRequestAction = -1
	ChangePort      ChangeRequestAction = 0x00000002
	ChangeIPAndPort ChangeRequestAction = 0x00000006
)

type NatType int

const (
	UnknownNatType NatType = iota
	NoNat
	UDPBlock
	FullConeNAT
	RestrictedConeNAT
	PortRestrictedConeNAT
	SymmetricNAT
)

func (t NatType) String() string {
	switch t {
	case UnknownNatType:
		return "UnknownNatType"
	case NoNat:
		return "NoNat"
	case UDPBlock:
		return "UDPBlock"
	case FullConeNAT:
		return "FullConeNAT"
	case RestrictedConeNAT:
		return "RestrictedConeNAT"
	case PortRestrictedConeNAT:
		return "PortRestrictedConeNAT"
	case SymmetricNAT:
		return "SymmetricNAT"
	default:
		return ""
	}
}

func (t NatType) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("\"%s\"", t.String())), nil
}

type MappingBehavior int

const (
	UnknownMappingBehavior MappingBehavior = iota
	NoMapping
	EndpointIndependentMapping
	AddressAndPortDependentMapping
	AddressDependentMapping
)

func (t MappingBehavior) String() string {
	switch t {
	case UnknownMappingBehavior:
		return "UnknownMappingBehavior"
	case NoMapping:
		return "NoMapping"
	case EndpointIndependentMapping:
		return "EndpointIndependentMapping"
	case AddressAndPortDependentMapping:
		return "AddressAndPortDependentMapping"
	case AddressDependentMapping:
		return "AddressDependentMapping"
	default:
		return ""
	}
}

func (t MappingBehavior) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("\"%s\"", t.String())), nil
}

type FilteringBehavior int

const (
	UnknownFilteringBehavior FilteringBehavior = iota
	EndpointIndependentFiltering
	AddressAndPortDependentFiltering
	AddressDependentFiltering
)

func (t FilteringBehavior) String() string {
	switch t {
	case UnknownFilteringBehavior:
		return "UnknownFilteringBehavior"
	case EndpointIndependentFiltering:
		return "EndpointIndependentFiltering"
	case AddressAndPortDependentFiltering:
		return "AddressAndPortDependentFiltering"
	case AddressDependentFiltering:
		return "AddressDependentFiltering"
	default:
		return ""
	}
}

func (t FilteringBehavior) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("\"%s\"", t.String())), nil
}

const (
	DefaultSTUNPort     = 3478
	STUNBindingRequest  = 0x0001
	STUNBindingResponse = 0x0101
	STUNMagicCookie     = 0x2112A442
	AttributeSize       = 12
	ReadTimeout         = 10 * time.Second
)
