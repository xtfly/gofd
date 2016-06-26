package p2p

import (
	"time"

	log "github.com/cihub/seelog"
	"github.com/xtfly/gofd/common"
)

type global struct {
	cfg *common.Config // 全局配置

	fsProvider FsProvider    // 读取文件
	cacher     CacheProvider // 用于缓存块信息
}

type P2pSessionMgnt struct {
	g *global //

	quitChan chan struct{} // 退出

	createSessChan chan *DispatchTask     // 要创建的Task
	startSessChan  chan *StartTask        //
	stopSessChan   chan string            // 要关闭的Task
	sessions       map[string]*P2pSession //
}

func NewSessionMgnt(cfg *common.Config) *P2pSessionMgnt {
	return &P2pSessionMgnt{
		g: &global{
			cfg:        cfg,
			fsProvider: OsFsProvider{},
			cacher:     NewRamCacheProvider(cfg.Control.CacheSize),
		},
		quitChan:       make(chan struct{}, 1),
		createSessChan: make(chan *DispatchTask, cfg.Control.MaxActive),
		startSessChan:  make(chan *StartTask, cfg.Control.MaxActive),
		stopSessChan:   make(chan string, 1),
		sessions:       make(map[string]*P2pSession, 10),
	}
}

// 启动监控
func (sm *P2pSessionMgnt) Start() error {
	conChan, listener, err := StartListen(sm.g.cfg)
	if err != nil {
		log.Error("Couldn't listen for peers connection: ", err)
		return err
	}
	defer listener.Close()

	checkSessChan := time.Tick(60 * time.Second) //每一分钟检查任务Session是否要清理
	for {
		select {
		case <-checkSessChan:
			for _, ts := range sm.sessions {
				if ts.Timeout() {
					log.Infof("[%s] P2p session is timeout, will be clean", ts.taskId)
					delete(sm.sessions, ts.taskId)
					ts.Quit()
				} else {
					log.Infof("[%s] Cached p2p task session", ts.taskId)
				}
			}
		case task := <-sm.createSessChan:
			if ts, err := NewP2pSession(sm.g, task); err != nil {
				log.Error("Could not create p2p task session.", err)
			} else {
				log.Infof("[%s] Created p2p task session", task.TaskId)
				sm.sessions[ts.taskId] = ts
				go func(s *P2pSession) {
					s.Init()
				}(ts)
			}
		case task := <-sm.startSessChan:
			if ts, ok := sm.sessions[task.TaskId]; ok {
				ts.Start(task)
			} else {
				log.Errorf("[%s] Not find p2p task session", task.TaskId)
			}
		case taskId := <-sm.stopSessChan:
			log.Infof("[%s] Stop p2p task session", taskId)
			if ts, ok := sm.sessions[taskId]; ok {
				delete(sm.sessions, taskId)
				ts.Quit()
			}
		case <-sm.quitChan:
			for _, ts := range sm.sessions {
				go ts.Quit()
			}
			log.Info("Closed all sessiong")
			return nil
		case c := <-conChan:
			log.Infof("[%s] New p2p connection, peer addr %s", c.taskId, c.remoteAddr.String())
			if ts, ok := sm.sessions[c.taskId]; ok {
				ts.AcceptNewPeer(c)
			} else {
				log.Errorf("[%s] Not find p2p task session", c.taskId)
				c.conn.Close() // TODO让客户端重连
			}
		}
	}
}

// 停止所有的任务，并退出监控
func (sm *P2pSessionMgnt) Stop() {
	sm.quitChan <- struct{}{}
}

// 创建一个任务
func (sm *P2pSessionMgnt) CreateTask(dt *DispatchTask) {
	go func(dt *DispatchTask) {
		sm.createSessChan <- dt
	}(dt)
}

// 启动一个任务
func (sm *P2pSessionMgnt) StartTask(st *StartTask) {
	go func(st *StartTask) {
		sm.startSessChan <- st
	}(st)
}

// 停止一下任务
func (sm *P2pSessionMgnt) StopTask(taskId string) {
	go func(taskId string) {
		sm.stopSessChan <- taskId
	}(taskId)
}
