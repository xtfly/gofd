package p2p

import (
	"time"
)

//----------------------------------------
// create a task message body
type Task struct {
	Id            string   `json:"id"`
	DispatchFiles []string `json:"dispatchFiles"`
	DestIPs       []string `json:"destIPs"`
}

// query a task info message body
type TaskInfo struct {
	Id     string `json:"id"`
	Status string `json:"status"`

	StartedAt  time.Time `json:"startedAt"`
	FinishedAt time.Time `json:"finishedAt"`

	DispatchInfos []DispatchInfo `json:"dispatchInfos,omitempty"`
}

type DispatchInfo struct {
	IP     string `json:"ip"`
	Status string `json:"status"`

	StartedAt  time.Time `json:"startedAt"`
	FinishedAt time.Time `json:"finishedAt"`

	DispatchFiles []DispatchFile `json:"dispatchFiles"`
}

type DispatchFile struct {
	FileName string `json:"filename"`
	Status   string `json:"status"`
}

// Task status type
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
// one files meta info
type FileDict struct {
	Length int64  `json:"length"`
	Path   string `json:"path"`
	Name   string `json:"name"`
	Sum    string `json:"sum"`
}

// all files meta info
type MetaInfo struct {
	TaskId    string
	LinkChain LinkChain
	Length    int64      `json:"length"`
	Sum       string     `json:"sum"`
	PieceLen  int64      `json:"PieceLen"`
	Pieces    string     `json:"pieces"`
	Files     []FileDict `json:"files"` // Multiple File
}

type LinkChain struct {
	// 软件分发的路径，要求服务端的地址排在第一个
	DispatchAddrs []string `json:"dispatchAddrs"`
	// 服务端管理接口，用于上报状态
	ServerAddr string `json:"serverAddr"`
}

// data connection header
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
