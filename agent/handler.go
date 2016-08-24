package agent

import (
	"github.com/labstack/echo/v4"
	"github.com/labstack/gommon/log"
	"github.com/xtfly/gofd/common"
	"github.com/xtfly/gofd/p2p"
)

// CreateTask POST /api/v1/agent/tasks
func (svc *Agent) CreateTask(c echo.Context) (err error) {
	//  获取Body
	dt := new(p2p.DispatchTask)
	if err = c.Bind(dt); err != nil {
		log.Errorf("Recv '%s' request, decode body failed. %v", c.Request().URL, err)
		return
	}

	common.LOG.Infof("[%s] Recv create task request", dt.TaskID)
	// 暂不检查任务是否重复下发
	svc.sessionMgnt.CreateTask(dt)
	return nil
}

// StartTask POST /api/v1/agent/tasks/start
func (svc *Agent) StartTask(c echo.Context) (err error) {
	//  获取Body
	st := new(p2p.StartTask)
	if err = c.Bind(st); err != nil {
		common.LOG.Errorf("Recv '%s' request, decode body failed. %v", c.Request().URL, err)
		return
	}

	common.LOG.Infof("[%s] Recv start task request", st.TaskID)
	// 暂不检查任务是否重复下发
	svc.sessionMgnt.StartTask(st)
	return nil
}

// CancelTask DELETE /api/v1/agent/tasks/:id
func (svc *Agent) CancelTask(c echo.Context) error {
	id := c.Param("id")
	common.LOG.Infof("[%s] Recv cancel task request", id)
	svc.sessionMgnt.StopTask(id)
	return nil
}
