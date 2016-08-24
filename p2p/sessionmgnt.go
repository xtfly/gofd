package p2p

import (
	"github.com/xtfly/gofd/common"
)

type global struct {
	cfg *common.Config // 全局配置

	fsProvider FsProvider    // 读取文件
	cache      CacheProvider // 用于缓存块信息
}

// TaskSessionMgnt ...
type TaskSessionMgnt struct {
	g *global //

	quitChan chan struct{} // 退出

	createSessChan chan *DispatchTask      // 要创建的Task
	startSessChan  chan *StartTask         //
	stopSessChan   chan string             // 要关闭的Task
	sessions       map[string]*TaskSession //
}

// NewSessionMgnt ...
func NewSessionMgnt(cfg *common.Config) *TaskSessionMgnt {
	return &TaskSessionMgnt{
		g: &global{
			cfg:        cfg,
			fsProvider: OsFsProvider{},
			cache:      NewRAMCacheProvider(cfg.Control.CacheSize),
		},
		quitChan:       make(chan struct{}, 1),
		createSessChan: make(chan *DispatchTask, cfg.Control.MaxActive),
		startSessChan:  make(chan *StartTask, cfg.Control.MaxActive),
		stopSessChan:   make(chan string, 1),
		sessions:       make(map[string]*TaskSession, 10),
	}
}

// Start 启动监控
func (sm *TaskSessionMgnt) Start() error {
	conChan, listener, err := StartListen(sm.g.cfg)
	if err != nil {
		common.LOG.Error("Couldn't listen for peers connection: ", err)
		return err
	}
	defer listener.Close()

	for {
		select {
		case task := <-sm.createSessChan:
			if ts, err := NewTaskSession(sm.g, task, sm.stopSessChan); err != nil {
				common.LOG.Error("Could not create p2p task session.", err)
			} else {
				common.LOG.Infof("[%s] Created p2p task session", task.TaskID)
				sm.sessions[ts.taskID] = ts
				go func(s *TaskSession) {
					s.Init()
				}(ts)
			}
		case task := <-sm.startSessChan:
			if ts, ok := sm.sessions[task.TaskID]; ok {
				ts.Start(task)
			} else {
				common.LOG.Errorf("[%s] Not find p2p task session", task.TaskID)
			}
		case taskID := <-sm.stopSessChan:
			common.LOG.Infof("[%s] Stop p2p task session", taskID)
			if ts, ok := sm.sessions[taskID]; ok {
				delete(sm.sessions, taskID)
				ts.Quit()
			}
		case <-sm.quitChan:
			for _, ts := range sm.sessions {
				go ts.Quit()
			}
			common.LOG.Info("Closed all sessions")
			return nil
		case c := <-conChan:
			common.LOG.Infof("[%s] New p2p connection, peer addr %s", c.taskID, c.remoteAddr.String())
			if ts, ok := sm.sessions[c.taskID]; ok {
				ts.AcceptNewPeer(c)
			} else {
				common.LOG.Errorf("[%s] Not find p2p task session", c.taskID)
				if err := c.conn.Close(); err != nil { // TODO让客户端重连
					common.LOG.Errorf("[%s] Close connection failed, connect %v", c.taskID, c.conn.RemoteAddr())
				}
			}
		}
	}
}

// Stop 停止所有的任务，并退出监控
func (sm *TaskSessionMgnt) Stop() {
	sm.quitChan <- struct{}{}
}

// CreateTask 创建一个任务
func (sm *TaskSessionMgnt) CreateTask(dt *DispatchTask) {
	go func(dt *DispatchTask) {
		sm.createSessChan <- dt
	}(dt)
}

// StartTask 启动一个任务
func (sm *TaskSessionMgnt) StartTask(st *StartTask) {
	go func(st *StartTask) {
		sm.startSessChan <- st
	}(st)
}

// StopTask 停止一下任务
func (sm *TaskSessionMgnt) StopTask(taskID string) {
	go func(taskID string) {
		sm.stopSessChan <- taskID
	}(taskID)
}
