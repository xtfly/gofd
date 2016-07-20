package server

//----------------------------------------
import "time"

// 创建分发任务
type CreateTask struct {
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
	Status          string  `json:"status"`
	PercentComplete float32 `json:"percentComplete"`

	StartedAt  time.Time `json:"startedAt"`
	FinishedAt time.Time `json:"finishedAt"`

	DispatchFiles []*DispatchFile `json:"dispatchFiles"`
}

// 单个文件分发状态
type DispatchFile struct {
	FileName        string  `json:"filename"`
	PercentComplete float32 `json:"-"`
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
