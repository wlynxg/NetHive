package engine

type Option struct {
	Device struct {
		TUNName string
		MTU     int
	}
	UDPAddr string
}
