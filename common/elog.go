package common

import (
	"encoding/json"
	"io"

	rlog "github.com/Sirupsen/logrus"
	"github.com/labstack/echo/log"
	glog "github.com/labstack/gommon/log"
)

type elog struct {
	rlog.FieldLogger
}

func toString(j glog.JSON) string {
	b, _ := json.Marshal(j)
	return string(b)
}

func NewEchoLogger() log.Logger {
	return &elog{FieldLogger: rlog.StandardLogger()}
}
func (l *elog) SetOutput(io.Writer) {}
func (l *elog) SetLevel(glog.Lvl)   {}
func (l *elog) Printj(j glog.JSON)  { rlog.Print(toString(j)) }
func (l *elog) Debugj(j glog.JSON)  { rlog.Debug(toString(j)) }
func (l *elog) Infoj(j glog.JSON)   { rlog.Info(toString(j)) }
func (l *elog) Warnj(j glog.JSON)   { rlog.Warn(toString(j)) }
func (l *elog) Errorj(j glog.JSON)  { rlog.Error(toString(j)) }
func (l *elog) Fatalj(j glog.JSON)  { rlog.Fatal(toString(j)) }
