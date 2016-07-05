package p2p

import (
	"encoding/json"
	"net/http"

	log "github.com/cihub/seelog"
	"github.com/xtfly/gofd/common"
)

type reportInfo struct {
	serverAddr      string
	percentComplete float32
}

type reportor struct {
	taskId string
	cfg    *common.Config
	client *http.Client

	reportChan chan *reportInfo
}

func NewReportor(taskId string, cfg *common.Config) *reportor {
	r := &reportor{
		taskId:     taskId,
		cfg:        cfg,
		client:     common.CreateHttpClient(cfg),
		reportChan: make(chan *reportInfo, 20),
	}

	go r.run()
	return r
}

func (r *reportor) run() {
	for rc := range r.reportChan {
		r.reportImp(rc)
	}
}

func (r *reportor) DoReport(serverAddr string, pecent float32) {
	r.reportChan <- &reportInfo{serverAddr: serverAddr, percentComplete: pecent}
}

func (r *reportor) Close() {
	close(r.reportChan)
}

func (r *reportor) reportImp(ri *reportInfo) {
	if int(ri.percentComplete) == 100 {
		log.Infof("[%s] Report session status... completed", r.taskId)
	}
	csr := &StatusReport{
		TaskId:          r.taskId,
		IP:              r.cfg.Net.IP,
		PercentComplete: ri.percentComplete,
	}
	bs, err := json.Marshal(csr)
	if err != nil {
		log.Errorf("[%s] Report session status failed. error=%v", r.taskId, err)
		return
	}

	_, err = common.SendHttpReq(r.cfg, "POST",
		ri.serverAddr, "/api/v1/server/tasks/status", bs)
	if err != nil {
		log.Errorf("[%s] Report session status failed. error=%v", r.taskId, err)
	}
	return
}
