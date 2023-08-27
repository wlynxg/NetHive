package engine

import (
	"crypto/rand"
	"encoding/hex"
	"net/netip"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
)

type Option struct {
	TUNName         string                     `json:"tun_name"`
	MTU             int                        `json:"mtu"`
	PrivateKey      *PrivateKey                `json:"private_key"`
	PeersRouteTable map[peer.ID][]netip.Prefix `json:"peers_route_table"`
	LocalRoute      []netip.Prefix             `json:"local_route"`
	LocalAddr       netip.Prefix               `json:"local_addr"`
}

type PrivateKey struct {
	key []byte
}

func NewPrivateKey() (*PrivateKey, error) {
	key, _, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		return nil, err
	}
	privateKey, err := crypto.MarshalPrivateKey(key)
	if err != nil {
		return nil, err
	}
	return &PrivateKey{key: privateKey}, nil
}

func (p *PrivateKey) MarshalJSON() ([]byte, error) {
	return []byte(hex.EncodeToString(p.key)), nil
}

func (p *PrivateKey) UnmarshalJSON(data []byte) error {
	decodeString, err := hex.DecodeString(string(data))
	if err != nil {
		return err
	}
	p.key = decodeString
	return nil
}

func (p *PrivateKey) PrivKey() (crypto.PrivKey, error) {
	key, err := crypto.UnmarshalPrivateKey(p.key)
	if err != nil {
		return nil, err
	}
	return key, nil
}
