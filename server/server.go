package server

import (
	"github.com/labstack/echo"
	"github.com/xtfly/gofd/common"
	"github.com/xtfly/gokits"
)

import "time"

const (
	CXT_SERVER = "_server"
)

type Server struct {
	common.BaseService
	// cache the task
	cache *gokits.Cache
}

func NewServer(cfg *common.Config) (*Server, error) {
	s := &Server{
		cache: gokits.NewCache(5 * time.Minute),
	}
	s.BaseService = *common.NewBaseService(cfg, s)
	return s, nil
}

func (s *Server) OnStart(c *common.Config, e *echo.Echo) error {
	cmf := setServerToContext(s)
	e.POST("/api/v1/server/tasks", CreateTask, cmf)
	e.DELETE("/api/v1/server/tasks/:id", CancelTask, cmf)
	e.GET("/api/v1/server/tasks/:id", QueryTask, cmf)
	return nil
}

func setServerToContext(s *Server) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set(CXT_SERVER, s)
			return next(c)
		}
	}
}
