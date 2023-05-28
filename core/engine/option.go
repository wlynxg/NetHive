package engine

import "time"

type Option struct {
	Device struct {
		TUNName string
		MTU     int
	}
	UDPAddr  string
	Server   string
	Interval time.Duration
}
