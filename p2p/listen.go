package p2p

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"time"

	log "github.com/cihub/seelog"
	"github.com/xtfly/gofd/common"
)

// P2pConn wraps an incoming network connection and contains metadata that helps
// identify which active p2pSession it's relevant for.
type P2pConn struct {
	conn       net.Conn
	client     bool //  对端是否为客户端
	remoteAddr net.Addr
	taskId     string
}

// listenForPeerConnections listens on a TCP port for incoming connections and
// demuxes them to the appropriate active p2pSession based on the InfoHash
// in the header.
func StartListen(cfg *common.Config) (conChan chan *P2pConn, listener net.Listener, err error) {
	listener, err = CreateListener(cfg)
	if err != nil {
		return
	}

	conChan = make(chan *P2pConn)
	go func(cfg *common.Config, conChan chan *P2pConn) {
		var tempDelay time.Duration
		for {
			conn, e := listener.Accept()
			if e != nil {
				if ne, ok := e.(net.Error); ok && ne.Temporary() {
					if tempDelay == 0 {
						tempDelay = 5 * time.Millisecond
					} else {
						tempDelay *= 2
					}
					if max := 1 * time.Second; tempDelay > max {
						tempDelay = max
					}
					log.Infof("http: Accept error: %v; retrying in %v", e, tempDelay)
					time.Sleep(tempDelay)
					continue
				}
				return
			}
			tempDelay = 0

			h, err := readHeader(conn)
			if err != nil {
				log.Error("Error reading header: ", err)
				continue
			}

			if err := h.validate(cfg); err != nil {
				log.Error("header auth failed:", err)
				continue
			}

			conChan <- &P2pConn{
				conn:       conn,
				client:     true,
				remoteAddr: conn.RemoteAddr(),
				taskId:     h.TaskId,
			}
		}
	}(cfg, conChan)

	return
}

func CreateListener(cfg *common.Config) (listener net.Listener, err error) {
	listener, err = net.ListenTCP("tcp",
		&net.TCPAddr{
			IP:   net.ParseIP(cfg.Net.IP),
			Port: cfg.Net.DataPort,
		})

	if err != nil {
		log.Error("Listen failed:", err)
		return
	}

	log.Info("Listening for peers on port:", cfg.Net.DataPort)
	return
}

// reading header info
func readHeader(conn net.Conn) (h *Header, err error) {
	h = &Header{}

	var bslen int32
	err = binary.Read(conn, binary.BigEndian, &bslen)
	if err != nil {
		err = fmt.Errorf("Read length error: %v", err)
		return
	}

	if bslen <= 0 || bslen > 200 {
		err = fmt.Errorf("read length is invalid: %v", bslen)
		return
	}

	bs := make([]byte, bslen)
	_, err = conn.Read(bs)
	if err != nil {
		err = fmt.Errorf("Couldn't read auth info: %v", err)
		return
	}

	h.Len = bslen
	buf := bytes.NewBuffer(bs)

	// taskId
	if h.TaskId, err = buf.ReadString(byte(0x00)); err != nil {
		err = fmt.Errorf("Read taskId error: %v", err)
		return
	}
	h.TaskId = h.TaskId[:len(h.TaskId)-1]

	// username
	if h.Username, err = buf.ReadString(byte(0x00)); err != nil {
		err = fmt.Errorf("Read username error: %v", err)
		return
	}
	h.Username = h.Username[:len(h.Username)-1]

	// password
	if h.Passowrd, err = buf.ReadString(byte(0x00)); err != nil {
		err = fmt.Errorf("Read password error: %v", err)
		return
	}
	h.Passowrd = h.Passowrd[:len(h.Passowrd)-1]

	// seed
	if h.Seed, err = buf.ReadString(byte(0x00)); err != nil {
		err = fmt.Errorf("Read password error: %v", err)
		return
	}
	h.Seed = h.Seed[:len(h.Seed)-1]

	return
}

func writeHeader(conn net.Conn, taskId string, cfg *common.Config) (err error) {
	// TODO 安全编码
	all := [][]byte{[]byte(taskId),
		[]byte(cfg.Auth.Username),
		[]byte(cfg.Auth.Passowrd),
		[]byte("12345678")}

	buf := bytes.NewBuffer(make([]byte, 0))
	blen := 0
	for _, v := range all {
		blen += len(v) + 1
	}

	binary.Write(buf, binary.BigEndian, int32(blen))
	for _, v := range all {
		buf.Write(v)
		buf.WriteByte(0)
	}

	_, err = conn.Write(buf.Bytes())
	return
}

func (h *Header) validate(cfg *common.Config) error {
	return nil
}
