package common

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/xtfly/log4g"
)

// Service is a common service interface
type Service interface {
	Start() error
	Stop() bool
	OnStart(c *Config, e *echo.Echo) error
	OnStop(c *Config, e *echo.Echo)
	IsRunning() bool
}

// BaseService is the basic service struct with config and status
type BaseService struct {
	name    string
	running uint32 // atomic
	Cfg     *Config
	echo    *echo.Echo
	svc     Service
}

// NewBaseService return created a basic service instance
func NewBaseService(cfg *Config, name string, svc Service) *BaseService {
	return &BaseService{
		name:    name,
		running: 0,
		Cfg:     cfg,
		echo:    echo.New(),
		svc:     svc,
	}
}

// init log by config
func (s *BaseService) initlog() {
	if s.Cfg.Log != "" {
		if err := log4g.GetManager().LoadConfigFile(s.Cfg.Log); err != nil {
			println("load log4g config failed")
		}
	}
}

func (s *BaseService) runEcho() (err error) {
	net := s.Cfg.Net
	addr := fmt.Sprintf("%s:%v", net.IP, net.MgntPort)
	LOG.Infof("Starting http server %s", addr)
	if net.TLS != nil {
		err = s.echo.StartTLS(addr, net.TLS.Cert, net.TLS.Key)
	} else {
		err = s.echo.Start(addr)
	}

	if err != nil {
		LOG.Infof("Start http server %s failed, %v", addr, err)
		return err
	}
	return nil
}

// Start the service
func (s *BaseService) Start() error {
	if atomic.CompareAndSwapUint32(&s.running, 0, 1) {
		s.initlog()
		LOG.Infof("Starting %s", s.name)
		if err := s.svc.OnStart(s.Cfg, s.echo); err != nil {
			return err
		}

		done := make(chan error)
		go func() {
			done <- s.runEcho()
		}()
		select {
		case err := <-done:
			return err
		case <-time.After(500 * time.Millisecond):
			return nil
		}
	}
	return errors.New("Started aleadry.")
}

// OnStart implements Service
func (s *BaseService) OnStart(c *Config, e *echo.Echo) error { return nil }

// Stop the service
func (s *BaseService) Stop() bool {
	if atomic.CompareAndSwapUint32(&s.running, 1, 0) {
		LOG.Infof("Stopping %s", s.name)
		s.svc.OnStop(s.Cfg, s.echo)
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := s.echo.Shutdown(ctx); err != nil {
			LOG.Error("shutdown echo server failed ", err)
		}
		return true
	}
	return false
}

// OnStop implements Service
func (s *BaseService) OnStop(c *Config, e *echo.Echo) {}

// IsRunning implements Service
func (s *BaseService) IsRunning() bool {
	return atomic.LoadUint32(&s.running) == 1
}

// Auth using basic authorization
func (s *BaseService) Auth(u, p string, ctx echo.Context) (bool, error) {
	if u == s.Cfg.Auth.Username && p == s.Cfg.Auth.Password {
		return true, nil
	}
	return false, nil
}
