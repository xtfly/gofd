package agent

import (
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/xtfly/gofd/common"
	"github.com/xtfly/gofd/p2p"
)

// Agent is the p2p client
type Agent struct {
	*common.BaseService
	sessionMgnt *p2p.TaskSessionMgnt
}

// NewAgent return created Agent instance
func NewAgent(cfg *common.Config) (*Agent, error) {
	c := &Agent{
		sessionMgnt: p2p.NewSessionMgnt(cfg),
	}
	c.BaseService = common.NewBaseService(cfg, cfg.Name, c)
	return c, nil
}

// OnStart implements the Service interface
func (c *Agent) OnStart(cfg *common.Config, e *echo.Echo) error {
	go func() { c.sessionMgnt.Start() }()

	e.Use(middleware.BasicAuth(c.Auth))
	e.POST("/api/v1/agent/tasks", c.CreateTask)
	e.POST("/api/v1/agent/tasks/start", c.StartTask)
	e.DELETE("/api/v1/agent/tasks/:id", c.CancelTask)

	return nil
}

// OnStop implements the Service interface
func (c *Agent) OnStop(cfg *common.Config, e *echo.Echo) {
	c.sessionMgnt.Stop()
}
