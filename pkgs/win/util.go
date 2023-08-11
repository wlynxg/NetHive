package win

func be2se(i uint16) uint16 {
	return (i>>8)&0xFF | (i&0xFF)<<8
}

func se2be(i uint16) uint16 {
	return (i>>8)&0xFF | (i&0xFF)<<8
}
