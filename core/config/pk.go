package config

import (
	"crypto/rand"
	"encoding/hex"

	"github.com/libp2p/go-libp2p/core/crypto"
)

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

func (p *PrivateKey) UnmarshalText(data []byte) error {
	decodeString, err := hex.DecodeString(string(data))
	if err != nil {
		return err
	}
	p.key = decodeString
	return nil
}

func (p *PrivateKey) MarshalText() ([]byte, error) {
	return []byte(hex.EncodeToString(p.key)), nil
}

func (p *PrivateKey) PrivKey() (crypto.PrivKey, error) {
	key, err := crypto.UnmarshalPrivateKey(p.key)
	if err != nil {
		return nil, err
	}
	return key, nil
}
