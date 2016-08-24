package p2p

import (
	"encoding/json"
	"net/http"

	"github.com/xtfly/gofd/common"
)

type reportInfo struct {
	serverAddr      string
	percentComplete float32
}

type reporter struct {
	taskID string
	cfg    *common.Config
	client *http.Client

	reportChan chan *reportInfo
}

func newReporter(taskID string, cfg *common.Config) *reporter {
	r := &reporter{
		taskID:     taskID,
		cfg:        cfg,
		client:     common.CreateHTTPClient(cfg),
		reportChan: make(chan *reportInfo, 20),
	}

	go r.run()
	return r
}

func (r *reporter) run() {
	for rc := range r.reportChan {
		r.reportImp(rc)
	}
}

func (r *reporter) DoReport(serverAddr string, pecent float32) {
	r.reportChan <- &reportInfo{serverAddr: serverAddr, percentComplete: pecent}
}

func (r *reporter) Close() {
	close(r.reportChan)
}

func (r *reporter) reportImp(ri *reportInfo) {
	if int(ri.percentComplete) == 100 {
		common.LOG.Infof("[%s] Report session status... completed", r.taskID)
	}
	csr := &StatusReport{
		TaskID:          r.taskID,
		IP:              r.cfg.Net.IP,
		PercentComplete: ri.percentComplete,
	}
	bs, err := json.Marshal(csr)
	if err != nil {
		common.LOG.Errorf("[%s] Report session status failed. error=%v", r.taskID, err)
		return
	}

	_, err = common.SendHTTPReq(r.cfg, "POST",
		ri.serverAddr, "/api/v1/server/tasks/status", bs)
	if err != nil {
		common.LOG.Errorf("[%s] Report session status failed. error=%v", r.taskID, err)
	}
	return
}
