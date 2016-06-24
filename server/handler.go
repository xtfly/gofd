package server

import (
	"fmt"
	"time"

	"net/http"

	"encoding/json"

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
func (s *Server) CreateTask(c echo.Context) (err error) {
	//  获取Body
	t := new(p2p.Task)
	if err = c.Bind(t); err != nil {
		log.Errorf("Recv '%s' request, decode body failed. %v", c.Request().URL(), err)
		return
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

func (s *Server) updateTask(ti *p2p.TaskInfo, ts p2p.TaskStatus) {
	ti.Status = ts.String()
	// 设置缓存超时间
	s.cache.UpdateExpiration(ti.Id, time.Now().Add(5*time.Minute).UnixNano())
}

func (s *Server) doTask(t *p2p.Task, ti *p2p.TaskInfo) {
	// 先产生任务元数据信息
	mi, err := p2p.CreateFileMeta(t.DispatchFiles, 0)
	if err != nil {
		s.updateTask(ti, p2p.TaskStatus_FileNotExist)
		return
	}

	dt := &p2p.DispatchTask{
		TaskId:   t.Id,
		MetaInfo: mi,
		Speed:    int64(s.Cfg.Control.Speed * 1024 * 1024),
	}

	// TODO 假定所有节点都能连上，后续再优化
	dt.LinkChain = createLinkChain(s.Cfg, t)

	dtbytes, err1 := json.Marshal(dt)
	if err1 != nil {
		s.updateTask(ti, p2p.TaskStatus_Failed)
		return
	}

	// 提交到session管理中运行
	s.sessionMgnt.RunTask(dt)

	// 给各节点发送Rest消息
	response := make(chan *TaskDispatchRsp)
	for _, ip := range t.DestIPs {
		go func(ip string) {
			if _, err2 := s.HttpPost(ip, "/api/v1/client/tasks", dtbytes); err2 != nil {
				response <- &TaskDispatchRsp{IP: ip, Success: false}
			} else {
				response <- &TaskDispatchRsp{IP: ip, Success: true}
			}
		}(ip)
	}

	count := 0
	fc := true
	for fc {
		select {
		case tdr := <-response:
			if di, ok := ti.DispatchInfos[tdr.IP]; ok {
				if tdr.Success {
					di.Status = p2p.TaskStatus_InProgress.String()
				} else {
					di.Status = p2p.TaskStatus_Failed.String()
				}
				count++
				if count == len(t.DestIPs) {
					fc = false
				}
			}
		case <-time.After(5 * time.Second): // 超时没有响应的
			if count == 0 {
				s.updateTask(ti, p2p.TaskStatus_Failed)
			}
			fc = false
		}
	}
	close(response)
}

//------------------------------------------
// DELETE /api/v1/server/tasks/:id
func (s *Server) CancelTask(c echo.Context) error {
	return nil
}

// GET /api/v1/server/tasks/:id
func (s *Server) QueryTask(c echo.Context) error {
	id := c.QueryParam("id")

	if v, ok := s.cache.Get(id); !ok {
		return c.String(http.StatusBadRequest, p2p.TaskStatus_TaskNotExist.String())
	} else {
		return c.JSON(200, v)
	}
}

// POST /api/v1/server/tasks/:id/status
func (s *Server) ReportTask(c echo.Context) error {
	return nil
}
