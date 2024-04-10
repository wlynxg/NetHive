package config

import (
	"log"
	"net/netip"

	"github.com/gogf/gf/v2/encoding/gjson"
	"github.com/gogf/gf/v2/os/gfile"
	"github.com/libp2p/go-libp2p/core/peer"
)

type Config struct {
	path            string
	TUNName         string
	MTU             int
	PrivateKey      *PrivateKey
	PeerID          string
	PeersRouteTable map[peer.ID][]netip.Prefix
	LocalRoute      []netip.Prefix
	LocalAddr       netip.Prefix
	EnableMDNS      bool
}

func (c *Config) Save() error {
	if err := gfile.PutBytes(c.path, gjson.New(c).MustToJsonIndent()); err != nil {
		log.Fatal(err)
		return err
	}
	return nil
}

func Load(path string) (*Config, error) {
	cfg := &Config{}
	if gfile.Exists(path) {
		load, err := gjson.Load(path)
		if err != nil {
			return nil, err
		}

		if err := load.Scan(&cfg); err != nil {
			return nil, err
		}
	}

	cfg.path = path
	defaultConfig(cfg)

	err := cfg.Save()
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

func defaultConfig(cfg *Config) {
	if cfg.TUNName == "" {
		cfg.TUNName = "hive0"
	}

	if cfg.MTU == 0 {
		cfg.MTU = 1500
	}

	if cfg.PrivateKey == nil {
		cfg.PrivateKey, _ = NewPrivateKey()
		key, err := cfg.PrivateKey.PrivKey()
		if err != nil {
			return
		}
		id, err := peer.IDFromPrivateKey(key)
		if err != nil {
			panic(err)
		}
		cfg.PeerID = id.String()
	}

	if !cfg.LocalAddr.IsValid() {
		cfg.LocalAddr = netip.MustParsePrefix("192.168.168.1/24")
	}
}
