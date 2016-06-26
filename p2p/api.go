package p2p

import (
	"time"
)

//----------------------------------------
// 创建分发任务
type Task struct {
	Id            string   `json:"id"`
	DispatchFiles []string `json:"dispatchFiles"`
	DestIPs       []string `json:"destIPs"`
}

// 查询分发任务
type TaskInfo struct {
	Id     string `json:"id"`
	Status string `json:"status"`

	StartedAt  time.Time `json:"startedAt"`
	FinishedAt time.Time `json:"finishedAt"`

	DispatchInfos map[string]*DispatchInfo `json:"dispatchInfos,omitempty"`
}

// 单个IP的分发信息
type DispatchInfo struct {
	Status string `json:"status"`

	StartedAt  time.Time `json:"startedAt"`
	FinishedAt time.Time `json:"finishedAt"`

	DispatchFiles []*DispatchFile `json:"dispatchFiles"`
}

// 单个文件分发状态
type DispatchFile struct {
	FileName string `json:"filename"`
	Status   string `json:"status"`
}

func (ti *TaskInfo) IsFinished() bool {
	completed := 0
	failed := 0
	for _, v := range ti.DispatchInfos {
		if v.Status == TaskStatus_Completed.String() {
			completed++
		}
		if v.Status == TaskStatus_Failed.String() {
			failed++
		}
	}

	if completed == len(ti.DispatchInfos) {
		ti.Status = TaskStatus_Completed.String()
		return true
	}

	if completed+failed == len(ti.DispatchInfos) {
		ti.Status = TaskStatus_Failed.String()
		return true
	}

	return false
}

// 任务状态
type TaskStatus int

const (
	TaskStatus_TaskNotExist TaskStatus = iota
	TaskStatus_TaskExist
	TaskStatus_Init
	TaskStatus_Failed
	TaskStatus_Completed
	TaskStatus_InProgress
	TaskStatus_FileNotExist
)

// convert task status to a string
func (ts TaskStatus) String() string {
	switch ts {
	case TaskStatus_TaskNotExist:
		return "TASK_NOT_EXISTED"
	case TaskStatus_TaskExist:
		return "TASK_EXISTED"
	case TaskStatus_Init:
		return "INIT"
	case TaskStatus_Failed:
		return "FAILED"
	case TaskStatus_Completed:
		return "COMPLETED"
	case TaskStatus_InProgress:
		return "INPROGESS"
	case TaskStatus_FileNotExist:
		return "FILE_NOT_EXISTED"
	default:
		return "TASK_NOT_EXISTED"
	}
}

//----------------------------------------
// 一个文件的元数据信息
type FileDict struct {
	Length int64  `json:"length"`
	Path   string `json:"path"`
	Name   string `json:"name"`
	Sum    string `json:"sum"`
}

// 一个任务内所有文件的元数据信息
type MetaInfo struct {
	Length   int64       `json:"length"`
	PieceLen int64       `json:"PieceLen"`
	Pieces   []byte      `json:"pieces"`
	Files    []*FileDict `json:"files"` // Multiple File
}

// 下发给客户端的分发任务
type DispatchTask struct {
	TaskId    string     `json:"taskId"`
	MetaInfo  *MetaInfo  `json:"metaInfo"`
	LinkChain *LinkChain `json:"linkChain"`
	Speed     int64      `json:"speed"`
}

// 下发给客户端的分发任务
type StartTask struct {
	TaskId    string     `json:"taskId"`
	LinkChain *LinkChain `json:"linkChain"`
}

// 分发路径
type LinkChain struct {
	// 软件分发的路径，要求服务端的地址排在第一个
	DispatchAddrs []string `json:"dispatchAddrs"`
	// 服务端管理接口，用于上报状态
	ServerAddr string `json:"serverAddr"`
}

// 连接认证消息头
type Header struct {
	Len      int32
	TaskId   string
	Username string
	Passowrd string
	Seed     string
}

// TODO
type ClientStatusReport struct {
	TaskId          string  `json:"taskId"`
	IP              string  `json:"ip"`
	PercentComplete float32 `json:"percentComplete"`
}
