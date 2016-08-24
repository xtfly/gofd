package server

import (
	"github.com/labstack/gommon/log"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/xtfly/gofd/common"
	"github.com/xtfly/gofd/p2p"
	"github.com/xtfly/gokits/gcache"
)

// CreateTask POST /api/v1/server/tasks
func (s *Server) CreateTask(c echo.Context) (err error) {
	//  获取Body
	t := new(CreateTask)
	if err = c.Bind(t); err != nil {
		common.LOG.Errorf("Recv [%s] request, decode body failed. %v", c.Request().URL, err)
		return
	}

	// 检查任务是否存在
	v, ok := s.cache.Get(t.ID)
	if ok {
		cti := v.(*CachedTaskInfo)
		if cti.EqualCmp(t) {
			return c.String(http.StatusAccepted, "")
		}
		common.LOG.Debugf("[%s] Recv task, task is existed", t.ID)
		return c.String(http.StatusBadRequest, TaskExist.String())
	}

	common.LOG.Infof("[%s] Recv task, file=%v, ips=%v", t.ID, t.DispatchFiles, t.DestIPs)

	cti := NewCachedTaskInfo(s, t)
	s.cache.Set(t.ID, cti, gcache.NoExpiration)
	s.cache.OnEvicted(func(id string, v interface{}) {
		common.LOG.Infof("[%s] Remove task cache", t.ID)
		cti := v.(*CachedTaskInfo)
		cti.quitChan <- struct{}{}
	})
	go cti.Start()

	return c.String(http.StatusAccepted, "")
}

// CancelTask DELETE /api/v1/server/tasks/:id
func (s *Server) CancelTask(c echo.Context) error {
	id := c.Param("id")
	common.LOG.Infof("[%s] Recv cancel task", id)
	v, ok := s.cache.Get(id)
	if !ok {
		return c.String(http.StatusBadRequest, TaskNotExist.String())
	}
	cti := v.(*CachedTaskInfo)
	cti.stopChan <- struct{}{}
	return c.JSON(http.StatusAccepted, "")
}

// QueryTask GET /api/v1/server/tasks/:id
func (s *Server) QueryTask(c echo.Context) error {
	id := c.Param("id")
	log.Infof("[%s] Recv query task", id)
	v, ok := s.cache.Get(id)
	if !ok {
		return c.String(http.StatusBadRequest, TaskNotExist.String())
	}
	cti := v.(*CachedTaskInfo)
	return c.JSON(http.StatusOK, cti.Query())

}

// ReportTask POST /api/v1/server/tasks/status
func (s *Server) ReportTask(c echo.Context) (err error) {
	//  获取Body
	csr := new(p2p.StatusReport)
	if err = c.Bind(csr); err != nil {
		common.LOG.Errorf("Recv [%s] request, decode body failed. %v", c.Request().URL, err)
		return
	}

	common.LOG.Debugf("[%s] Recv task report, ip=%v, percent=%v", csr.TaskID, csr.IP, csr.PercentComplete)
	if v, ok := s.cache.Get(csr.TaskID); ok {
		cti := v.(*CachedTaskInfo)
		cti.reportChan <- csr
	}

	return c.String(http.StatusOK, "")
}
