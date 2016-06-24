package p2p

import (
	log "github.com/Sirupsen/logrus"
	"github.com/xtfly/gofd/common"
)

type global struct {
	cfg *common.Config // 全局配置

	fsProvider FsProvider    // 读取文件
	cacher     CacheProvider // 用于缓存块信息
}

type P2pSessionMgnt struct {
	g *global

	quitChan chan struct{} // 退出

	createSessChan chan *DispatchTask // 要创建的Task
	startSessChan  chan *P2pSession   // 要启动的Task
	stopSessChan   chan string        // 要关闭的Task
	sessions       map[string]*P2pSession
}

func NewSessionMgnt(cfg *common.Config) *P2pSessionMgnt {
	return &P2pSessionMgnt{
		g: &global{
			cfg:        cfg,
			fsProvider: OsFsProvider{},
			cacher:     NewRamCacheProvider(cfg.Control.CacheSize)},

		quitChan:       make(chan struct{}, 1),
		createSessChan: make(chan *DispatchTask, cfg.Control.MaxActive),
		startSessChan:  make(chan *P2pSession, 1),
		stopSessChan:   make(chan string, 1),
		sessions:       make(map[string]*P2pSession),
	}
}

// 启动监控
func (sm *P2pSessionMgnt) Start() error {
	conChan, _, err := StartListen(sm.g.cfg)
	if err != nil {
		log.Error("Couldn't listen for peers connection: ", err)
		return err
	}

	for {
		select {
		case task := <-sm.createSessChan:
			if ts, err := NewP2pSession(sm.g, task); err != nil {
				log.Error("Could not create torrent session.", err)
			} else {
				log.Infof("Created torrent session for %s", task.TaskId)
				sm.startSessChan <- ts
			}
		case ts := <-sm.startSessChan:
			sm.sessions[ts.taskId] = ts
			log.Infof("Starting p2p session for %s", ts.taskId)
			go func(s *P2pSession) {
				s.Start()
			}(ts)
		case taskId := <-sm.stopSessChan:
			if ts, ok := sm.sessions[taskId]; ok {
				delete(sm.sessions, taskId)
				ts.Quit()
			}
		case <-sm.quitChan:
			for _, ts := range sm.sessions {
				go ts.Quit()
			}
			return nil
		case c := <-conChan:
			log.Infof("New bt connection for ih %x", c.taskId)
			if ts, ok := sm.sessions[c.taskId]; ok {
				ts.AcceptNewPeer(c)
			}
		}
	}
}

// 停止所有的任务，并退出监控
func (sm *P2pSessionMgnt) Stop() {
	sm.quitChan <- struct{}{}
}

// 启动一个任务
func (sm *P2pSessionMgnt) RunTask(dt *DispatchTask) {
	go func(dt *DispatchTask) {
		sm.createSessChan <- dt
	}(dt)
}

// 停止一下任务
func (sm *P2pSessionMgnt) StopTask(taskId string) {
	go func(taskId string) {
		sm.stopSessChan <- taskId
	}(taskId)
}
