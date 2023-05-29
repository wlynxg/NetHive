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
	// local node information
	Node info.NodeInfo
	// others node information
	Nodes []info.NodeInfo
}

func (c *Client) Connect(ctx context.Context, req info.NodeInfo) (*ConnectRes, error) {
	path, err := url.JoinPath(c.server, ConnectURL)
	if err != nil {
		return nil, err
	}

	var res *ConnectRes
	response, err := c.client.Post(ctx, path, req)
	if err != nil {
		return nil, err
	}

	err = gconv.Scan(response.ReadAll(), &res)
	if err != nil {
		return nil, err
	}
	return res, nil
}
