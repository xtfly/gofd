package agent

import (
	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
	"github.com/xtfly/gofd/common"
	"github.com/xtfly/gofd/p2p"
)

type Agent struct {
	common.BaseService
	// Session管理
	sessionMgnt *p2p.P2pSessionMgnt
}

func NewAgent(cfg *common.Config) (*Agent, error) {
	c := &Agent{
		sessionMgnt: p2p.NewSessionMgnt(cfg),
	}
	c.BaseService = *common.NewBaseService(cfg, cfg.Name, c)
	return c, nil
}

func (c *Agent) OnStart(cfg *common.Config, e *echo.Echo) error {
	go func() { c.sessionMgnt.Start() }()

	e.Use(middleware.BasicAuth(c.Auth))
	e.POST("/api/v1/agent/tasks", c.CreateTask)
	e.POST("/api/v1/agent/tasks/start", c.StartTask)
	e.DELETE("/api/v1/agent/tasks/:id", c.CancelTask)

	return nil
}

func (c *Agent) OnStop(cfg *common.Config, e *echo.Echo) {
	go func() { c.sessionMgnt.Stop() }()
}
