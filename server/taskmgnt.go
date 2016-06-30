package server

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	log "github.com/cihub/seelog"
	"github.com/xtfly/gofd/common"
	"github.com/xtfly/gofd/p2p"
)

type TaskClientRsp struct {
	IP      string
	Success bool
}

// 每一个Task，对应一个缓存对象，所有与它关联的操作都由一个Goroutine来处理
type CachedTaskInfo struct {
	s *Server

	id            string
	dispatchFiles []string
	destIPs       []string
	ti            *p2p.TaskInfo

	succCount int
	failCount int
	allCount  int

	stopChan     chan struct{}
	reportChan   chan *p2p.ClientStatusReport
	agentRspChan chan *TaskClientRsp
}

func NewCachedTaskInfo(s *Server, t *p2p.Task) *CachedTaskInfo {
	return &CachedTaskInfo{
		s:             s,
		id:            t.Id,
		dispatchFiles: t.DispatchFiles,
		destIPs:       t.DestIPs,
		ti:            newTaskInfo(t),

		stopChan:     make(chan struct{}),
		reportChan:   make(chan *p2p.ClientStatusReport, 10),
		agentRspChan: make(chan *TaskClientRsp, 10),
	}
}

func newTaskInfo(t *p2p.Task) *p2p.TaskInfo {
	init := p2p.TaskStatus_Init.String()
	ti := &p2p.TaskInfo{Id: t.Id, Status: init, StartedAt: time.Now()}
	ti.DispatchInfos = make(map[string]*p2p.DispatchInfo, len(t.DestIPs))
	for _, ip := range t.DestIPs {
		di := &p2p.DispatchInfo{Status: init, StartedAt: time.Now()}
		di.DispatchFiles = make([]*p2p.DispatchFile, len(t.DispatchFiles))
		ti.DispatchInfos[ip] = di
		for j, fn := range t.DispatchFiles {
			di.DispatchFiles[j] = &p2p.DispatchFile{FileName: fn}
		}
	}
	return ti
}

func createLinkChain(cfg *common.Config, ips []string, ti *p2p.TaskInfo) *p2p.LinkChain {
	lc := new(p2p.LinkChain)
	lc.ServerAddr = fmt.Sprintf("%s:%v", cfg.Net.IP, cfg.Net.MgntPort)
	lc.DispatchAddrs = make([]string, 1+len(ips))
	// 第一个节点为服务端
	lc.DispatchAddrs[0] = fmt.Sprintf("%s:%v", cfg.Net.IP, cfg.Net.DataPort)

	idx := 1
	for _, ip := range ips {
		if di, ok := ti.DispatchInfos[ip]; ok && di.Status == p2p.TaskStatus_InProgress.String() {
			lc.DispatchAddrs[idx] = fmt.Sprintf("%s:%v", ip, cfg.Net.AgentDataPort)
			idx++
		}
	}
	lc.DispatchAddrs = lc.DispatchAddrs[:idx]

	return lc
}

// 使用一个Goroutine来启动任务操作
func (ct *CachedTaskInfo) Start() {
	if ts := ct.createTask(); ts != p2p.TaskStatus_InProgress {
		ct.endTask(ts)
		return
	}

	for {
		select {
		case <-ct.stopChan:
			ct.ti.Status = p2p.TaskStatus_Failed.String()
			ct.stopAllClientTask()
			return
		case csr := <-ct.reportChan:
			ct.reportStatus(csr)
			if checkFinished(ct.ti) {
				ct.stopAllClientTask()
				return
			}
		}
	}
}

func (ct *CachedTaskInfo) endTask(ts p2p.TaskStatus) {
	log.Errorf("[%s] Task status changed, status=%v", ct.id, ts)
	ct.ti.Status = ts.String()
	ct.ti.FinishedAt = time.Now()
	ct.s.cache.Replace(ct.id, ct, 5*time.Minute)
	ct.s.sessionMgnt.StopTask(ct.id)
}

func (ct *CachedTaskInfo) createTask() p2p.TaskStatus {
	// 先产生任务元数据信息
	start := time.Now()
	mi, err := p2p.CreateFileMeta(ct.dispatchFiles, 256*1024)
	end := time.Now()
	if err != nil {
		log.Errorf("[%s] Create file meta failed, error=%v", ct.id, err)
		return p2p.TaskStatus_FileNotExist
	}
	log.Infof("[%s] Create metainfo: (%.2f seconds)", ct.id, end.Sub(start).Seconds())

	dt := &p2p.DispatchTask{
		TaskId:   ct.id,
		MetaInfo: mi,
		Speed:    int64(ct.s.Cfg.Control.Speed * 1024 * 1024),
	}
	dt.LinkChain = createLinkChain(ct.s.Cfg, []string{}, ct.ti) //

	dtbytes, err1 := json.Marshal(dt)
	if err1 != nil {
		return p2p.TaskStatus_Failed
	}
	log.Debugf("[%s] Create dispatch task, task=%v", ct.id, string(dtbytes))

	ct.allCount = len(ct.destIPs)
	// 提交到session管理中运行
	ct.s.sessionMgnt.CreateTask(dt)
	// 给各节点发送创建分发任务的Rest消息
	ct.sendReqToClients(ct.destIPs, "/api/v1/agent/tasks", dtbytes)

	for {
		select {
		case tdr := <-ct.agentRspChan:
			ct.checkAgentRsp(tdr)
			if ct.failCount == ct.allCount {
				return p2p.TaskStatus_Failed
			}
			if ct.succCount+ct.failCount == ct.allCount {
				if ts := ct.startTask(); ts != p2p.TaskStatus_InProgress {
					return ts
				}
				// 部分节点响应，则也继续
				return p2p.TaskStatus_InProgress
			}
		case <-time.After(5 * time.Second): // 等超时
			if ct.succCount == 0 {
				log.Errorf("[%s] Wait client response timeout.", ct.id)
				return p2p.TaskStatus_Failed
			}
		}
	}
}

func (ct *CachedTaskInfo) checkAgentRsp(tcr *TaskClientRsp) {
	if di, ok := ct.ti.DispatchInfos[tcr.IP]; ok {
		di.StartedAt = time.Now()
		if tcr.Success {
			di.Status = p2p.TaskStatus_InProgress.String()
			ct.succCount++
		} else {
			di.Status = p2p.TaskStatus_Failed.String()
			di.FinishedAt = time.Now()
			ct.failCount++
		}
	}
}

func (ct *CachedTaskInfo) startTask() p2p.TaskStatus {
	log.Infof("[%s] Recv all client response, will send start command to clients", ct.id)
	st := &p2p.StartTask{TaskId: ct.id}
	st.LinkChain = createLinkChain(ct.s.Cfg, ct.destIPs, ct.ti)

	stbytes, err1 := json.Marshal(st)
	if err1 != nil {
		return p2p.TaskStatus_Failed
	}
	log.Debugf("[%s] Create start task, task=%v", ct.id, string(stbytes))

	// 第一个是Server，不用发送启动
	ct.allCount = len(st.LinkChain.DispatchAddrs) - 1
	ct.succCount, ct.failCount = 0, 0
	ct.s.sessionMgnt.StartTask(st)

	// 给其它各节点发送启支分发任务的Rest消息
	ct.sendReqToClients(st.LinkChain.DispatchAddrs[1:], "/api/v1/agent/tasks/start", stbytes)
	for {
		select {
		case tdr := <-ct.agentRspChan:
			ct.checkAgentRsp(tdr)
			if ct.failCount == ct.allCount {
				return p2p.TaskStatus_Failed
			}
			if ct.succCount+ct.failCount == ct.allCount {
				return p2p.TaskStatus_InProgress
			}
		case <-time.After(5 * time.Second): // 等超时
			if ct.succCount == 0 {
				log.Errorf("[%s] Wait client response timeout.", ct.id)
				return p2p.TaskStatus_Failed
			}
		}
	}
}

func (ct *CachedTaskInfo) sendReqToClients(ips []string, url string, body []byte) {
	for _, ip := range ips {
		if idx := strings.Index(ip, ":"); idx > 0 {
			ip = ip[:idx]
		}

		go func(ip string) {
			if _, err2 := ct.s.HttpPost(ip, url, body); err2 != nil {
				log.Errorf("[%s] Send http request failed. POST, ip=%s, url=%s, error=%v", ct.id, ip, url, err2)
				ct.agentRspChan <- &TaskClientRsp{IP: ip, Success: false}
			} else {
				log.Debugf("[%s] Send http request success. POST, ip=%s, url=%s", ct.id, ip, url)
				ct.agentRspChan <- &TaskClientRsp{IP: ip, Success: true}
			}
		}(ip)
	}
}

// 给所有客户端发送停止命令
func (ct *CachedTaskInfo) stopAllClientTask() {
	url := "/api/v1/agent/tasks/" + ct.id
	ct.s.sessionMgnt.StopTask(ct.id)
	ct.s.cache.Replace(ct.id, ct, 5*time.Minute)
	for _, ip := range ct.destIPs {
		go func(ip string) {
			if err2 := ct.s.HttpDelete(ip, url); err2 != nil {
				log.Errorf("[%s] Send http request failed. DELETE, ip=%s, url=%s, error=%v", ct.id, ip, url, err2)
			} else {
				log.Debugf("[%s] Send http request success. DELETE, ip=%s, url=%s", ct.id, ip, url)
			}
		}(ip)
	}
}

func (ct *CachedTaskInfo) reportStatus(csr *p2p.ClientStatusReport) {
	if di, ok := ct.ti.DispatchInfos[csr.IP]; ok {
		if int(csr.PercentComplete) == 100 {
			di.Status = p2p.TaskStatus_Completed.String()
			di.FinishedAt = time.Now()
		} else if int(csr.PercentComplete) == -1 {
			di.Status = p2p.TaskStatus_Failed.String()
			di.FinishedAt = time.Now()
		}
		di.PercentComplete = csr.PercentComplete
	}
}

func (ct *CachedTaskInfo) Query(qchan chan *p2p.TaskInfo) {
	qchan <- ct.ti
}

func checkFinished(ti *p2p.TaskInfo) bool {
	completed := 0
	failed := 0
	for _, v := range ti.DispatchInfos {
		if v.Status == p2p.TaskStatus_Completed.String() {
			completed++
		}
		if v.Status == p2p.TaskStatus_Failed.String() {
			failed++
		}
	}

	if completed == len(ti.DispatchInfos) {
		ti.Status = p2p.TaskStatus_Completed.String()
		return true
	}

	if completed+failed == len(ti.DispatchInfos) {
		ti.Status = p2p.TaskStatus_Failed.String()
		return true
	}

	return false
}
