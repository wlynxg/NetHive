package main

import (
	"NetHive/core/engine"
	"fmt"
	"net/netip"
	"os"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
)

func main() {
	file, err := os.ReadFile("private.key")
	if err != nil {
		return
	}
	key, err := crypto.UnmarshalPrivateKey(file)
	if err != nil {
		return
	}
	fmt.Println(peer.IDFromPrivateKey(key))

	opt := engine.Option{
		TUNName:    "nethive0",
		MTU:        1500,
		LocalAddr:  netip.MustParsePrefix("192.168.199.1/32"),
		PrivateKey: key,
		PeersRouteTable: map[peer.ID][]netip.Prefix{
			"12D3KooWFDbzFGc89W8ZbaedTVcD4YgGMaKzTa3kw9hcZrBUrdZt": {netip.MustParsePrefix("192.168.199.1/32")},
			"12D3KooWLYUNghjWRUXBrdLqP2E7qky8r6GzJ74sJS4ZXn7cddJF": {netip.MustParsePrefix("192.168.199.2/32")},
		},
	}

	e, err := engine.New(&opt)
	if err != nil {
		panic(e)
	}
	err = e.Start()
	if err != nil {
		panic(err)
	}
}
