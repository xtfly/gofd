package p2p

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
	Files    []*FileDict `json:"files"`
}

// 下发给Agent的分发任务
type DispatchTask struct {
	TaskId    string     `json:"taskId"`
	MetaInfo  *MetaInfo  `json:"metaInfo"`
	LinkChain *LinkChain `json:"linkChain"`
	Speed     int64      `json:"speed"`
}

// 下发给Agent的分发任务
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
	Salt     string
}

// Agent分发状态上报
type StatusReport struct {
	TaskId          string  `json:"taskId"`
	IP              string  `json:"ip"`
	PercentComplete float32 `json:"percentComplete"`
}
