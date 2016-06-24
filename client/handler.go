package client

import (
	"github.com/labstack/echo"
	"github.com/labstack/gommon/log"
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

	// 暂不检查任务是否重复下发
	svc.sessionMgnt.RunTask(dt)
	return nil
}

//------------------------------------------
// DELETE /api/v1/client/tasks/:id
func (svc *Client) CancelTask(c echo.Context) error {
	id := c.QueryParam("id")

	svc.sessionMgnt.StopTask(id)
	return nil
}
