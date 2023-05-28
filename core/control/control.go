package control

import (
	"github.com/gogf/gf/v2/net/gclient"
)

const (
	ConnectURL = "/api/v1/connect"
)

type Client struct {
	server string
	client *gclient.Client
}

func New(server string) *Client {
	c := &Client{
		server: server,
		client: gclient.New(),
	}
	return c
}
