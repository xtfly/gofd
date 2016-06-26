package client

import (
	"github.com/labstack/echo"
	"github.com/xtfly/gofd/common"
	"github.com/xtfly/gofd/p2p"
)

const (
	CXT_CLIENT = "_client"
)

type Client struct {
	common.BaseService
	// Session管理
	sessionMgnt *p2p.P2pSessionMgnt
}

func NewClient(cfg *common.Config) (*Client, error) {
	c := &Client{
		sessionMgnt: p2p.NewSessionMgnt(cfg),
	}
	c.BaseService = *common.NewBaseService(cfg, cfg.Name, c)
	return c, nil
}

func (c *Client) OnStart(cfg *common.Config, e *echo.Echo) error {
	go func() { c.sessionMgnt.Start() }()

	e.POST("/api/v1/client/tasks", c.CreateTask)
	e.POST("/api/v1/client/tasks/start", c.StartTask)
	e.DELETE("/api/v1/client/tasks/:id", c.CancelTask)

	return nil
}

func (c *Client) OnStop(cfg *common.Config, e *echo.Echo) {
	go func() { c.sessionMgnt.Stop() }()
}
