package main

import (
	"flag"
	"log"

	"github.com/wlynxg/NetHive/core/config"
	"github.com/wlynxg/NetHive/core/engine"
)

type args struct {
	config string
}

func parse() *args {
	param := &args{}
	flag.StringVar(&param.config, "config", "/var/lib/NetHive/config.json", `configuration file path`)
	flag.Parse()
	return param
}

func main() {
	params := parse()

	cfg, err := config.Load(params.config)
	if err != nil {
		log.Fatal(err)
	}

	e, err := engine.Run(cfg)
	if err != nil {
		log.Fatal(err)
	}

	err = e.Run()
	if err != nil {
		log.Fatal(err)
	}
}
