package server

import (
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/xtfly/gofd/common"
	"github.com/xtfly/gofd/p2p"
	"github.com/xtfly/gokits/gcache"
)

// Server ..
type Server struct {
	*common.BaseService
	// 用于缓存当前接收到任务
	cache *gcache.Cache
	// Session管理
	sessionMgnt *p2p.TaskSessionMgnt
}

// NewServer ..
func NewServer(cfg *common.Config) (*Server, error) {
	s := &Server{
		cache:       gcache.NewCache(5 * time.Minute),
		sessionMgnt: p2p.NewSessionMgnt(cfg),
	}
	s.BaseService = common.NewBaseService(cfg, cfg.Name, s)
	return s, nil
}

// OnStart ...
func (s *Server) OnStart(c *common.Config, e *echo.Echo) error {
	go func() { s.sessionMgnt.Start() }()

	e.Use(middleware.BasicAuth(s.Auth))
	e.POST("/api/v1/server/tasks", s.CreateTask)
	e.DELETE("/api/v1/server/tasks/:id", s.CancelTask)
	e.GET("/api/v1/server/tasks/:id", s.QueryTask)
	e.POST("/api/v1/server/tasks/status", s.ReportTask)

	return nil
}

// OnStop ...
func (s *Server) OnStop(c *common.Config, e *echo.Echo) {
	s.sessionMgnt.Stop()
}
