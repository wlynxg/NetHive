package config

import (
	"net/netip"

	"github.com/gogf/gf/v2/encoding/gjson"
	"github.com/gogf/gf/v2/os/gfile"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/peer"
	mlog "github.com/wlynxg/NetHive/pkgs/log"
)

type Config struct {
	path string

	// tun
	TUNName   string
	MTU       int
	LocalAddr netip.Prefix

	// libp2p
	PrivateKey      *PrivateKey
	PeerID          string
	Bootstraps      []string
	PeersRouteTable map[string]netip.Prefix
	EnableMDNS      bool

	// log
	LogConfigs []mlog.CoreConfig
}

func (c *Config) Save() error {
	if err := gfile.PutBytes(c.path, gjson.New(c).MustToJsonIndent()); err != nil {
		return err
	}
	return nil
}

func Load(path string) (*Config, error) {
	cfg := &Config{path: path}
	if gfile.Exists(path) {
		load, err := gjson.Load(path)
		if err != nil {
			return nil, err
		}

		if err := load.Scan(&cfg); err != nil {
			return nil, err
		}
	}

	err := defaultConfig(cfg)
	if err != nil {
		return nil, err
	}

	err = cfg.Save()
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

func defaultConfig(cfg *Config) error {
	if cfg.TUNName == "" {
		cfg.TUNName = "hive0"
	}

	if cfg.MTU == 0 {
		cfg.MTU = 1500
	}

	if !cfg.LocalAddr.IsValid() {
		cfg.LocalAddr = netip.MustParsePrefix("192.168.168.1/24")
	}

	if cfg.PrivateKey == nil {
		cfg.PrivateKey, _ = NewPrivateKey()
		key, err := cfg.PrivateKey.PrivKey()
		if err != nil {
			return err
		}
		id, err := peer.IDFromPrivateKey(key)
		if err != nil {
			return err
		}
		cfg.PeerID = id.String()
	}

	if len(cfg.Bootstraps) == 0 {
		for _, n := range dht.DefaultBootstrapPeers {
			cfg.Bootstraps = append(cfg.Bootstraps, n.String())
		}
	}
	return nil
}
