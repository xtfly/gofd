package server

import (
	"fmt"
	"time"

	"net/http"

	log "github.com/Sirupsen/logrus"
	"github.com/labstack/echo"
	"github.com/xtfly/gofd/p2p"
	"github.com/xtfly/gokits"
)

func getServer(c echo.Context) (*Server, error) {
	s, ok := c.Get(CXT_SERVER).(*Server)
	if !ok {
		return nil, fmt.Errorf("Server errror.")
	}
	return s, nil
}

// POST /api/v1/server/tasks
func CreateTask(c echo.Context) (err error) {
	//  获取Body
	t := new(p2p.Task)
	if err = c.Bind(t); err != nil {
		log.Errorf("Recv '%s' request, decode body failed. %v", c.Request().URL(), err)
		return
	}

	// 获取Server对象
	s, err := getServer(c)
	if err != nil {
		return err
	}

	// 检查任务是否存在
	if _, ok := s.cache.Get(t.Id); ok {
		return c.String(http.StatusBadRequest, p2p.TaskStatus_TaskExist.String())
	}

	ti := NewTaskInfo(t)
	s.cache.Set(ti.Id, ti, gokits.NoExpiration) // 任务完成之后再刷新过期时间

	go s.doTask(t, ti)
	return nil
}

func (s *Server) doTask(t *p2p.Task, ti *p2p.TaskInfo) {
	// 先产生任务元数据信息
	mi, err := p2p.CreateFileMeta(t.DispatchFiles, 0)
	if err != nil {
		// 更新状态
		ti.Status = p2p.TaskStatus_FileNotExist.String()
		// 设置缓存超时间
		s.cache.UpdateExpiration(t.Id, time.Now().Add(5*time.Minute).UnixNano())
		return
	}

	// 否则提交到session管理中运行
	s.sessionMgnt.RunTask(mi)

	response := make(chan string)
	for {
		select {
		case addr := <-response:
			//
		case <-time.After(5 * time.Second):
			// 超时没有响应的，认为都是失败的
		}

	}

	// TODO给各节点发送Rest消息
}

//------------------------------------------
// DELETE /api/v1/server/tasks/:id
func CancelTask(c echo.Context) error {
	return nil
}

// GET /api/v1/server/tasks/:id
func QueryTask(c echo.Context) error {
	return nil
}
