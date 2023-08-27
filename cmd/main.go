package main

import (
	"NetHive/core/engine"
	"flag"
	"log"
	"path"

	"github.com/gogf/gf/v2/encoding/gjson"
	"github.com/gogf/gf/v2/os/gfile"
)

func main() {
	dir := flag.String("config-dir", "", `configuration file path`)
	if dir == nil || *dir == "" || !gfile.Exists(*dir) || !gfile.IsDir(*dir) {
		log.Fatal("config dir must be a valid folder path")
		return
	}

	var opt *engine.Option
	cfg := path.Join(*dir, "config.json")
	if gfile.Exists(cfg) {
		load, err := gjson.Load(cfg)
		if err != nil {
			log.Fatal(err)
			return
		}

		if err := load.Scan(&opt); err != nil {
			log.Fatal(err)
			return
		}
	} else {
		key, err := engine.NewPrivateKey()
		if err != nil {
			log.Fatal(err)
			return
		}
		opt = &engine.Option{
			TUNName:    "hive0",
			MTU:        1500,
			PrivateKey: key,
		}

		if err := gfile.PutBytes(cfg, gjson.New(opt).MustToJson()); err != nil {
			log.Fatal(err)
			return
		}
	}

	e, err := engine.New(opt)
	if err != nil {
		panic(e)
	}
	err = e.Start()
	if err != nil {
		panic(err)
	}
}
