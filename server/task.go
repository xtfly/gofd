package server

import (
	"fmt"

	"github.com/xtfly/gofd/common"
	"github.com/xtfly/gofd/p2p"
)

type TaskDispatchRsp struct {
	IP      string
	Success bool
}

func NewTaskInfo(t *p2p.Task) *p2p.TaskInfo {
	ti := &p2p.TaskInfo{}
	ti.Id = t.Id
	init := p2p.TaskStatus_Init.String()
	ti.Status = init
	ti.DispatchInfos = make(map[string]p2p.DispatchInfo, len(t.DestIPs))
	for _, ip := range t.DestIPs {
		di := p2p.DispatchInfo{}
		ti.DispatchInfos[ip] = di
		di.Status = init
		di.DispatchFiles = make([]p2p.DispatchFile, len(t.DispatchFiles))
		for j, fn := range t.DispatchFiles {
			di.DispatchFiles[j].FileName = fn
			di.DispatchFiles[j].Status = init
		}
	}
	return ti
}

func createLinkChain(cfg *common.Config, t *p2p.Task) *p2p.LinkChain {
	lc := new(p2p.LinkChain)
	lc.ServerAddr = fmt.Sprintf("%s:%v", cfg.Net.IP, cfg.Net.MgntPort)
	lc.DispatchAddrs = make([]string, 1+len(t.DestIPs))
	// 第一个节点为服务端
	lc.DispatchAddrs[0] = fmt.Sprintf("%s:%v", cfg.Net.IP, cfg.Net.DataPort)
	for idx, ip := range t.DestIPs {
		lc.DispatchAddrs[idx+1] = fmt.Sprintf("%s:%v", ip, cfg.Net.ClientDataPort)
	}

	return lc
}
