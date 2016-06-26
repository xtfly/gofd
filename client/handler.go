package client

import (
	log "github.com/cihub/seelog"
	"github.com/labstack/echo"
	"github.com/xtfly/gofd/p2p"
)

// POST /api/v1/client/tasks
func (svc *Client) CreateTask(c echo.Context) (err error) {
	//  获取Body
	dt := new(p2p.DispatchTask)
	if err = c.Bind(dt); err != nil {
		log.Errorf("Recv '%s' request, decode body failed. %v", c.Request().URL(), err)
		return
	}

	log.Infof("[%s] Recv create task request", dt.TaskId)
	// 暂不检查任务是否重复下发
	svc.sessionMgnt.CreateTask(dt)
	return nil
}

// POST /api/v1/client/tasks/start
func (svc *Client) StartTask(c echo.Context) (err error) {
	//  获取Body
	st := new(p2p.StartTask)
	if err = c.Bind(st); err != nil {
		log.Errorf("Recv '%s' request, decode body failed. %v", c.Request().URL(), err)
		return
	}

	log.Infof("[%s] Recv start task request", st.TaskId)
	// 暂不检查任务是否重复下发
	svc.sessionMgnt.StartTask(st)
	return nil
}

//------------------------------------------
// DELETE /api/v1/client/tasks/:id
func (svc *Client) CancelTask(c echo.Context) error {
	id := c.Param("id")
	log.Infof("[%s] Recv cancel ask request", id)
	svc.sessionMgnt.StopTask(id)
	return nil
}
