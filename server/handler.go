package server

import (
	"net/http"

	log "github.com/cihub/seelog"
	"github.com/labstack/echo"
	"github.com/xtfly/gofd/p2p"
	"github.com/xtfly/gokits"
)

//------------------------------------------
// POST /api/v1/server/tasks
func (s *Server) CreateTask(c echo.Context) (err error) {
	//  获取Body
	t := new(p2p.Task)
	if err = c.Bind(t); err != nil {
		log.Errorf("Recv [%s] request, decode body failed. %v", c.Request().URL(), err)
		return
	}

	// 检查任务是否存在
	if v, ok := s.cache.Get(t.Id); ok {
		cti := v.(*CachedTaskInfo)
		if cti.EqualCmp(t) {
			return c.String(http.StatusAccepted, "")
		} else {
			log.Debugf("[%s] Recv task, task is existed", t.Id)
			return c.String(http.StatusBadRequest, p2p.TaskStatus_TaskExist.String())
		}
	}

	log.Infof("[%s] Recv task, file=%v, ips=%v", t.Id, t.DispatchFiles, t.DestIPs)

	cti := NewCachedTaskInfo(s, t)
	s.cache.Set(t.Id, cti, gokits.NoExpiration)
	s.cache.OnEvicted(func(id string, v interface{}) {
		log.Infof("[%s] Remove task cache", t.Id)
		cti := v.(*CachedTaskInfo)
		cti.quitChan <- struct{}{}
	})
	go cti.Start()

	return c.String(http.StatusAccepted, "")
}

//------------------------------------------
// DELETE /api/v1/server/tasks/:id
func (s *Server) CancelTask(c echo.Context) error {
	id := c.Param("id")
	log.Infof("[%s] Recv cancel task", id)
	if v, ok := s.cache.Get(id); !ok {
		return c.String(http.StatusBadRequest, p2p.TaskStatus_TaskNotExist.String())
	} else {
		cti := v.(*CachedTaskInfo)
		cti.stopChan <- struct{}{}
		return c.JSON(http.StatusAccepted, "")
	}
}

//------------------------------------------
// GET /api/v1/server/tasks/:id
func (s *Server) QueryTask(c echo.Context) error {
	id := c.Param("id")
	log.Infof("[%s] Recv query task", id)
	if v, ok := s.cache.Get(id); !ok {
		return c.String(http.StatusBadRequest, p2p.TaskStatus_TaskNotExist.String())
	} else {
		cti := v.(*CachedTaskInfo)
		return c.JSON(http.StatusOK, cti.Query())
	}
}

//------------------------------------------
// POST /api/v1/server/tasks/status
func (s *Server) ReportTask(c echo.Context) (err error) {
	//  获取Body
	csr := new(p2p.StatusReport)
	if err = c.Bind(csr); err != nil {
		log.Errorf("Recv [%s] request, decode body failed. %v", c.Request().URL(), err)
		return
	}

	log.Debugf("[%s] Recv task report, ip=%v, pecent=%v", csr.TaskId, csr.IP, csr.PercentComplete)
	if v, ok := s.cache.Get(csr.TaskId); ok {
		cti := v.(*CachedTaskInfo)
		cti.reportChan <- csr
	}

	return c.String(http.StatusOK, "")
}
