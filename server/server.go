package server

import (
	"time"

	"github.com/labstack/echo"
	"github.com/xtfly/gofd/common"
	"github.com/xtfly/gofd/p2p"
	"github.com/xtfly/gokits"
)

const (
	CXT_SERVER = "_server"
)

type Server struct {
	common.BaseService
	// 用于缓存当前接收到任务
	cache *gokits.Cache
	// Session管理
	sessionMgnt *p2p.P2pSessionMgnt
}

func NewServer(cfg *common.Config) (*Server, error) {
	s := &Server{
		cache:       gokits.NewCache(5 * time.Minute),
		sessionMgnt: p2p.NewSessionMgnt(cfg),
	}
	s.BaseService = *common.NewBaseService(cfg, cfg.Name, s)
	return s, nil
}

func (s *Server) OnStart(c *common.Config, e *echo.Echo) error {
	go func() { s.sessionMgnt.Start() }()

	e.Use(authContext(s))
	e.POST("/api/v1/server/tasks", s.CreateTask)
	e.DELETE("/api/v1/server/tasks/:id", s.CancelTask)
	e.GET("/api/v1/server/tasks/:id", s.QueryTask)
	e.POST("/api/v1/server/tasks/status", s.ReportTask)

	return nil
}

func (s *Server) OnStop(c *common.Config, e *echo.Echo) {
	go func() { s.sessionMgnt.Stop() }()
}

func authContext(svc *Server) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// TODO 认证
			return next(c)
		}
	}
}
