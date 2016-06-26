package server

import (
	"strings"
	"time"

	"net/http"

	"encoding/json"

	log "github.com/cihub/seelog"
	"github.com/labstack/echo"
	"github.com/xtfly/gofd/p2p"
	"github.com/xtfly/gokits"
)

// POST /api/v1/server/tasks
func (s *Server) CreateTask(c echo.Context) (err error) {
	//  获取Body
	t := new(p2p.Task)
	if err = c.Bind(t); err != nil {
		log.Errorf("Recv [%s] request, decode body failed. %v", c.Request().URL(), err)
		return
	}

	// 检查任务是否存在
	if _, ok := s.cache.Get(t.Id); ok {
		log.Debugf("[%s] Recv task, task is existed", t.Id)
		return c.String(http.StatusBadRequest, p2p.TaskStatus_TaskExist.String())
	}

	log.Infof("[%s] Recv task, file=%v, ips=%v", t.Id, t.DispatchFiles, t.DestIPs)
	ti := NewTaskInfo(t)
	s.cache.Set(ti.Id, ti, gokits.NoExpiration) // 任务完成之后再刷新过期时间

	go s.doTask(t, ti)
	return nil
}

func (s *Server) updateTask(ti *p2p.TaskInfo, ts p2p.TaskStatus) {
	log.Errorf("[%s] Task status changed, status=%v", ti.Id, ts)
	ti.Status = ts.String()
	// 设置缓存超时间
	s.cache.UpdateExpiration(ti.Id, time.Now().Add(5*time.Minute).UnixNano())
	s.sessionMgnt.StopTask(ti.Id)
}

func (s *Server) doTask(t *p2p.Task, ti *p2p.TaskInfo) {
	// 先产生任务元数据信息
	mi, err := p2p.CreateFileMeta(t.DispatchFiles, 512*1024)
	if err != nil {
		log.Errorf("[%s] Create file meta failed, error=%v", ti.Id, err)
		s.updateTask(ti, p2p.TaskStatus_FileNotExist)
		return
	}

	dt := &p2p.DispatchTask{
		TaskId:   t.Id,
		MetaInfo: mi,
		Speed:    int64(s.Cfg.Control.Speed * 1024 * 1024),
	}

	//  先假定所有节点都能连上
	dt.LinkChain = createLinkChain(s.Cfg, t, nil)

	rspChan := make(chan *TaskClientRsp)
	defer close(rspChan)

	{
		dtbytes, err1 := json.Marshal(dt)
		if err1 != nil {
			s.updateTask(ti, p2p.TaskStatus_Failed)
			return
		}
		log.Debugf("[%s] Create dispatch task, task=%v", ti.Id, string(dtbytes))

		// 提交到session管理中运行
		s.sessionMgnt.CreateTask(dt)
		// 给各节点发送创建分发任务的Rest消息
		s.sendReqToClients(t.Id, t.DestIPs, "/api/v1/client/tasks", dtbytes, rspChan)
	}

	// 等所有响应
	allCount := len(t.DestIPs)
	rspCount := s.waitClientRsp(ti, allCount, rspChan)

	// 生成新的分发路径
	if rspCount == allCount && ti.Status != p2p.TaskStatus_Failed.String() {
		log.Infof("[%s] Recv all client response, will send start command to clients", t.Id)
		st := &p2p.StartTask{TaskId: t.Id}
		//log.Debugf("[%s] Create link chain, info=%v", ti.Id, ti)
		st.LinkChain = createLinkChain(s.Cfg, t, ti)

		stbytes, err1 := json.Marshal(st)
		if err1 != nil {
			s.updateTask(ti, p2p.TaskStatus_Failed)
			return
		}
		log.Debugf("[%s] Create start task, task=%v", ti.Id, string(stbytes))

		// 第一个是Server，不用发送启动
		allCount = len(st.LinkChain.DispatchAddrs) - 1
		s.sessionMgnt.StartTask(st)

		// 给其它各节点发送启支分发任务的Rest消息
		s.sendReqToClients(t.Id, st.LinkChain.DispatchAddrs[1:], "/api/v1/client/tasks/start", stbytes, rspChan)
		s.waitClientRsp(ti, allCount, rspChan)
	}
}

func (s *Server) sendReqToClients(taskId string, ips []string, url string, body []byte, rspChan chan *TaskClientRsp) {
	for _, ip := range ips {
		if idx := strings.Index(ip, ":"); idx > 0 {
			ip = ip[:idx]
		}

		go func(ip string) {
			if _, err2 := s.HttpPost(ip, url, body); err2 != nil {
				log.Errorf("[%s] Send http request failed. POST, ip=%s, url=%s, error=%v", taskId, ip, url, err2)
				rspChan <- &TaskClientRsp{IP: ip, Success: false}
			} else {
				log.Debugf("[%s] Send http request success. POST, ip=%s, url=%s", taskId, ip, url)
				rspChan <- &TaskClientRsp{IP: ip, Success: true}
			}
		}(ip)
	}
}

func (s *Server) waitClientRsp(ti *p2p.TaskInfo, allCount int, rspChan chan *TaskClientRsp) int {
	succCount := 0
	failCount := 0
	fc := true
	for fc {
		select {
		case tdr := <-rspChan:
			if di, ok := ti.DispatchInfos[tdr.IP]; ok {
				if tdr.Success {
					di.Status = p2p.TaskStatus_InProgress.String()
					succCount++
				} else {
					di.Status = p2p.TaskStatus_Failed.String()
					failCount++
				}

				if succCount == allCount {
					fc = false
				}

				if failCount == allCount {
					s.updateTask(ti, p2p.TaskStatus_Failed)
					fc = false
					break
				}
			}
		case <-time.After(5 * time.Second): // 超时没有响应的
			if succCount == 0 {
				log.Errorf("[%s] Wait client response timeout.", ti.Id)
				s.updateTask(ti, p2p.TaskStatus_Failed)
			}
			fc = false
		}
	}
	return succCount + failCount
}

//------------------------------------------
// DELETE /api/v1/server/tasks/:id
func (s *Server) CancelTask(c echo.Context) error {
	return nil
}

//------------------------------------------
// GET /api/v1/server/tasks/:id
func (s *Server) QueryTask(c echo.Context) error {
	id := c.Param("id")

	if v, ok := s.cache.Get(id); !ok {
		return c.String(http.StatusBadRequest, p2p.TaskStatus_TaskNotExist.String())
	} else {
		return c.JSON(http.StatusOK, v)
	}
}

//------------------------------------------
// POST /api/v1/server/tasks/status
func (s *Server) ReportTask(c echo.Context) (err error) {
	//  获取Body
	csr := new(p2p.ClientStatusReport)
	if err = c.Bind(csr); err != nil {
		log.Errorf("Recv [%s] request, decode body failed. %v", c.Request().URL(), err)
		return
	}

	log.Infof("[%s] Recv task report, ip=%v, pecent=%v", csr.TaskId, csr.IP, csr.PercentComplete)
	// 检查任务是否存在
	if v, ok := s.cache.Get(csr.TaskId); ok {
		ti := v.(*p2p.TaskInfo)
		if di, ok := ti.DispatchInfos[csr.IP]; ok {
			if int(csr.PercentComplete) == 100 {
				di.Status = p2p.TaskStatus_Completed.String()
				di.FinishedAt = time.Now()
			}
		}

		if ti.IsFinished() {
			s.stopAllClientTask(ti)
		}
	}

	return c.String(http.StatusOK, "")
}

// 给所有客户端发送停止命令
func (s *Server) stopAllClientTask(ti *p2p.TaskInfo) {
	url := "/api/v1/client/tasks/" + ti.Id
	s.sessionMgnt.StopTask(ti.Id)
	s.cache.UpdateExpiration(ti.Id, time.Now().Add(5*time.Minute).UnixNano())
	for ip, _ := range ti.DispatchInfos {
		go func(ip string) {
			if err2 := s.HttpDelete(ip, url); err2 != nil {
				log.Errorf("[%s] Send http request failed. DELETE, ip=%s, url=%s, error=%v", ti.Id, ip, url, err2)
			} else {
				log.Debugf("[%s] Send http request success. DELETE, ip=%s, url=%s", ti.Id, ip, url)
			}
		}(ip)
	}
}
