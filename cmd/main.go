package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	_ "net/http/pprof"

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

	go func() {
		http.ListenAndServe(":6060", nil)
	}()

	cfg, err := config.Load(params.config)
	if err != nil {
		log.Fatal(err)
	}

	e, err := engine.Run(context.Background(), cfg)
	if err != nil {
		log.Fatal(err)
	}

	err = e.Run()
	if err != nil {
		log.Fatal(err)
	}
}
