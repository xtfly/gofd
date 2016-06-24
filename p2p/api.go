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

	DispatchInfos map[string]DispatchInfo `json:"dispatchInfos,omitempty"`
}

// 单个IP的分发信息
type DispatchInfo struct {
	Status string `json:"status"`

	StartedAt  time.Time `json:"startedAt"`
	FinishedAt time.Time `json:"finishedAt"`

	DispatchFiles []DispatchFile `json:"dispatchFiles"`
}

// 单个文件分发状态
type DispatchFile struct {
	FileName string `json:"filename"`
	Status   string `json:"status"`
}

// 任务状态
type TaskStatus int

const (
	// task is not existed
	TaskStatus_TaskNotExist TaskStatus = iota
	// task has existed
	TaskStatus_TaskExist
	// task is initing
	TaskStatus_Init
	// task is failed
	TaskStatus_Failed
	// task is completed
	TaskStatus_Completed
	// files in the task are dispatching
	TaskStatus_InProgress
	// few files in the taks are not existed
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
	Length   int64      `json:"length"`
	Sum      string     `json:"sum"`
	PieceLen int64      `json:"PieceLen"`
	Pieces   string     `json:"pieces"`
	Files    []FileDict `json:"files"` // Multiple File
}

// 下发给客户端的分发任务
type DispatchTask struct {
	TaskId    string     `json:"taskId"`
	LinkChain *LinkChain `json:"linkChain"`
	MetaInfo  *MetaInfo  `json:"metaInfo"`
	Speed     int64      `json:"speed"`
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
	Event  string
	TaskId string
	IP     string
}
