package server

import "github.com/xtfly/gofd/p2p"

type Task struct {
	p2p.TaskInfo
}

func NewTaskInfo(t *p2p.Task) *p2p.TaskInfo {
	ti := &p2p.TaskInfo{}
	ti.Id = t.Id
	init := p2p.TaskStatus_Init.String()
	ti.Status = init
	ti.DispatchInfos = make([]p2p.DispatchInfo, len(t.DestIPs))
	for i, ip := range t.DestIPs {
		ti.DispatchInfos[i].IP = ip
		ti.DispatchInfos[i].Status = init
		ti.DispatchInfos[i].DispatchFiles = make([]p2p.DispatchFile, len(t.DispatchFiles))
		for j, fn := range t.DispatchFiles {
			ti.DispatchInfos[i].DispatchFiles[j].FileName = fn
			ti.DispatchInfos[i].DispatchFiles[j].Status = init
		}
	}
	return ti
}
