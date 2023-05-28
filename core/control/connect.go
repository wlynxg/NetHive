package control

import (
	"NetHive/core/info"
	"context"
	"net/url"

	"github.com/gogf/gf/v2/util/gconv"
)

type ConnectReq struct {
	Node info.NodeInfo
}

type ConnectRes struct {
	Nodes []info.NodeInfo
}

func (c *Client) Connect(ctx context.Context, req info.NodeInfo) ([]info.NodeInfo, error) {
	path, err := url.JoinPath(c.server, ConnectURL)
	if err != nil {
		return nil, err
	}

	var nodes []info.NodeInfo
	response, err := c.client.Post(ctx, path, req)
	if err != nil {
		return nil, err
	}

	err = gconv.Scan(response.ReadAll(), &nodes)
	if err != nil {
		return nil, err
	}
	return nodes, nil
}
