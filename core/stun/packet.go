package netcheck

import (
	"bytes"
	"encoding/binary"
	"math/rand"
	"net"
	"time"
	"unsafe"
)

func buildRequestPacket(transactionID []byte, action ChangeRequestAction) []byte {
	var (
		req  = STUNRequestPacket{}
		buff = make([]byte, 1024)
	)

	// set the message type
	binary.BigEndian.PutUint16(buff, STUNBindingRequest)
	copy(req.MessageType[:], buff)

	// set the message length
	if action != NoAction {
		binary.BigEndian.PutUint16(buff, 8)
	} else {
		binary.BigEndian.PutUint16(buff, 0)
	}
	copy(req.MessageLength[:], buff)

	// set a magic cookie to be compatible with RFC3489
	binary.BigEndian.PutUint32(buff, STUNMagicCookie)
	copy(req.MagicCookie[:], buff)

	// set the TransactionID
	if len(transactionID) != 12 {
		return nil
	} else {
		copy(req.TransactionID[:], transactionID)
	}

	// convert a structure to a byte array
	*(*STUNRequestPacket)(unsafe.Pointer(&buff[0])) = req
	offset := unsafe.Sizeof(STUNRequestPacket{})

	switch action {
	case ChangePort:
		// set ChangeRequest
		binary.BigEndian.PutUint16(buff[offset:offset+2], uint16(ChangeRequest))
		offset += 2

		binary.BigEndian.PutUint16(buff[offset:offset+2], 4)
		offset += 2

		binary.BigEndian.PutUint32(buff[offset:offset+4], uint32(ChangePort))
		offset += 4
		return buff[:offset]
	case ChangeIPAndPort:
		// set ChangeRequest
		binary.BigEndian.PutUint16(buff[offset:offset+2], uint16(ChangeRequest))
		offset += 2

		binary.BigEndian.PutUint16(buff[offset:offset+2], 4)
		offset += 2

		binary.BigEndian.PutUint32(buff[offset:offset+4], uint32(ChangeIPAndPort))
		offset += 4
		return buff[:offset]
	default:
		return buff[:offset]
	}
}

func parseResponsePacket(buff []byte) *STUNResponse {
	var (
		resp   = &STUNResponse{Attributes: map[AttributeType]Attribute{}}
		offset = 0
	)

	// set the MessageType
	if binary.BigEndian.Uint16(buff[offset:offset+2]) == STUNBindingResponse {
		resp.MessageType = STUNBindingResponse
	}
	offset += 2

	// set the MessageLength
	resp.MessageLength = int(binary.BigEndian.Uint16(buff[offset : offset+2]))
	offset += 2

	// set the MagicCookie
	resp.MagicCookie = binary.BigEndian.Uint32(buff[offset : offset+4])
	offset += 4

	// set the TransactionID
	resp.TransactionID = make([]byte, 12)
	copy(resp.TransactionID, buff[offset:offset+12])
	offset += 12

	for i := 0; i < resp.MessageLength; i += AttributeSize {
		attribute := Attribute{}

		// set AttributeType
		attribute.Type = AttributeType(binary.BigEndian.Uint16(buff[offset : offset+2]))
		offset += 2

		// set AttributeLength
		attribute.Length = int(binary.BigEndian.Uint16(buff[offset : offset+2]))
		offset += 2

		// set AttributeReserved
		attribute.Reserved = int(buff[offset])
		offset += 1

		// set ProtocolFamily
		attribute.ProtocolFamily = ProtocolFamily(buff[offset])
		offset += 1

		// set Port
		attribute.Port = int(binary.BigEndian.Uint16(buff[offset : offset+2]))
		offset += 2

		// set IP
		attribute.IP = net.IPv4(buff[offset], buff[offset+1], buff[offset+2], buff[offset+3])
		offset += 4

		resp.Attributes[attribute.Type] = attribute
	}

	return resp
}

func sendAndRecv(udp *net.UDPConn, action ChangeRequestAction, remote *net.UDPAddr) (*STUNResponse, error) {
	buff := make([]byte, 1024)

	transactionID := make([]byte, 12)
	// set the TransactionID
	for i := 0; i < 12; i++ {
		transactionID[i] = byte(rand.Int())
	}

	_, err := udp.WriteToUDP(buildRequestPacket(transactionID, action), remote)
	if err != nil {
		return nil, err
	}
	udp.SetReadDeadline(time.Now().Add(ReadTimeout))
	n, _, err := udp.ReadFromUDP(buff)
	if err != nil {
		netErr, ok := err.(*net.OpError)
		if ok && netErr.Timeout() {
			return nil, nil
		} else {
			return nil, err
		}
	} else {
		resp := parseResponsePacket(buff[:n])
		if !bytes.Equal(resp.TransactionID, transactionID) {
			return nil, nil
		}
		return resp, nil
	}
}
