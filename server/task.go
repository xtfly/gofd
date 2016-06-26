package server

import (
	"fmt"

	"github.com/xtfly/gofd/common"
	"github.com/xtfly/gofd/p2p"
)

type TaskClientRsp struct {
	IP      string
	Success bool
}

func NewTaskInfo(t *p2p.Task) *p2p.TaskInfo {
	ti := &p2p.TaskInfo{}
	ti.Id = t.Id
	init := p2p.TaskStatus_Init.String()
	ti.Status = init
	ti.DispatchInfos = make(map[string]*p2p.DispatchInfo, len(t.DestIPs))
	for _, ip := range t.DestIPs {
		di := &p2p.DispatchInfo{}
		ti.DispatchInfos[ip] = di
		di.Status = init
		di.DispatchFiles = make([]*p2p.DispatchFile, len(t.DispatchFiles))
		for j, fn := range t.DispatchFiles {
			df := &p2p.DispatchFile{FileName: fn, Status: init}
			di.DispatchFiles[j] = df
		}
	}
	return ti
}

func createLinkChain(cfg *common.Config, t *p2p.Task, ti *p2p.TaskInfo) *p2p.LinkChain {
	lc := new(p2p.LinkChain)
	lc.ServerAddr = fmt.Sprintf("%s:%v", cfg.Net.IP, cfg.Net.MgntPort)
	lc.DispatchAddrs = make([]string, 1+len(t.DestIPs))
	// 第一个节点为服务端
	lc.DispatchAddrs[0] = fmt.Sprintf("%s:%v", cfg.Net.IP, cfg.Net.DataPort)

	idx := 1
	for _, ip := range t.DestIPs {
		if ti != nil {
			if di, ok := ti.DispatchInfos[ip]; ok && di.Status == p2p.TaskStatus_InProgress.String() {
				lc.DispatchAddrs[idx] = fmt.Sprintf("%s:%v", ip, cfg.Net.ClientDataPort)
				idx++
			}
		}
	}
	lc.DispatchAddrs = lc.DispatchAddrs[:idx]

	return lc
}
