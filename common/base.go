package common

import (
	"fmt"
	"sync/atomic"

	log "github.com/Sirupsen/logrus"
	"github.com/labstack/echo"
	"github.com/labstack/echo/engine/standard"
	"github.com/xtfly/loghook/rfile"
)

type Service interface {
	Start() error
	Stop() bool
	OnStart(c *Config, e *echo.Echo) error
	OnStop(c *Config, e *echo.Echo)
	IsRunning() bool
}

type BaseService struct {
	running uint32 // atomic
	cfg     *Config
	echo    *echo.Echo
	svc     Service
}

func NewBaseService(cfg *Config, svc Service) *BaseService {
	return &BaseService{
		running: 0,
		cfg:     cfg,
		echo:    echo.New(),
		svc:     svc,
	}
}

// init log by config
func (s *BaseService) initlog() {
	cfglog := s.cfg.Log
	if cfglog.File != "" {
		log.AddHook(rfile.NewHook(cfglog.File, cfglog.FileSize, cfglog.FileCount))
	}

	if lvl, err := log.ParseLevel(cfglog.Level); err != nil {
		log.SetLevel(lvl)
	}

	// init echo log
	s.echo.SetLogger(NewEchoLogger())
}

func (s *BaseService) runEcho() error {
	net := s.cfg.Net
	var sr *standard.Server
	if net.Tls != nil {
		sr = standard.WithTLS(fmt.Sprintf("%s:%v", net.IP, net.MgntPort),
			net.Tls.Cert,
			net.Tls.Key,
		)
	} else {
		sr = standard.New(fmt.Sprintf("%s:%v", net.IP, net.MgntPort))
	}
	sr.SetHandler(s.echo)
	sr.SetLogger(s.echo.Logger())
	return sr.Start()
}

func (s *BaseService) Start() error {
	if atomic.CompareAndSwapUint32(&s.running, 0, 1) {
		s.initlog()
		if err := s.svc.OnStart(s.cfg, s.echo); err != nil {
			return err
		}
		return s.runEcho()
	} else {
		return nil
	}
}

func (s *BaseService) OnStart(c *Config, e *echo.Echo) error { return nil }

func (s *BaseService) Stop() bool {
	if atomic.CompareAndSwapUint32(&s.running, 1, 0) {
		s.svc.OnStop(s.cfg, s.echo)
		// TODO echo server to stop
		return true
	} else {
		return false
	}
}

// Implements Service
func (s *BaseService) OnStop(c *Config, e *echo.Echo) {}

// Implements Service
func (s *BaseService) IsRunning() bool {
	return atomic.LoadUint32(&s.running) == 1
}
