package common

import (
	"encoding/json"
	"io"

	slog "github.com/cihub/seelog"
	"github.com/labstack/echo/log"
	glog "github.com/labstack/gommon/log"
)

type elog struct {
}

func toString(j glog.JSON) string {
	b, _ := json.Marshal(j)
	return string(b)
}

func NewEchoLogger() log.Logger {
	return &elog{}
}
func (l *elog) SetOutput(io.Writer) {}
func (l *elog) SetLevel(glog.Lvl)   {}
func (l *elog) Printj(j glog.JSON)  { slog.Trace(toString(j)) }
func (l *elog) Debugj(j glog.JSON)  { slog.Debug(toString(j)) }
func (l *elog) Infoj(j glog.JSON)   { slog.Info(toString(j)) }
func (l *elog) Warnj(j glog.JSON)   { slog.Warn(toString(j)) }
func (l *elog) Errorj(j glog.JSON)  { slog.Error(toString(j)) }
func (l *elog) Fatalj(j glog.JSON)  { slog.Error(toString(j)); panic("toString(j)") }

func (l *elog) Print(a ...interface{})            { slog.Trace(a...) }
func (l *elog) Printf(f string, a ...interface{}) { slog.Tracef(f, a...) }

func (l *elog) Debug(a ...interface{})            { slog.Debug(a...) }
func (l *elog) Debugf(f string, a ...interface{}) { slog.Debugf(f, a...) }

func (l *elog) Info(a ...interface{})            { slog.Info(a...) }
func (l *elog) Infof(f string, a ...interface{}) { slog.Infof(f, a...) }

func (l *elog) Warn(a ...interface{})            { slog.Warn(a...) }
func (l *elog) Warnf(f string, a ...interface{}) { slog.Warnf(f, a...) }

func (l *elog) Error(a ...interface{})            { slog.Error(a...) }
func (l *elog) Errorf(f string, a ...interface{}) { slog.Errorf(f, a...) }

func (l *elog) Fatal(a ...interface{})            { slog.Error(a...); panic("") }
func (l *elog) Fatalf(f string, a ...interface{}) { slog.Errorf(f, a...) }
