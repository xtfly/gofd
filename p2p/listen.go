package p2p

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"net"

	"github.com/xtfly/gofd/common"
)

// P2pConn wraps an incoming network connection and contains metadata that helps
// identify which active p2pSession it's relevant for.
type P2pConn struct {
	conn       net.Conn
	upstream   bool //是否是上游节点，即连接到其它的节点，而不是其它节点连入本节点
	remoteAddr net.Addr
	taskId     string
}

// listenForPeerConnections listens on a TCP port for incoming connections and
// demuxes them to the appropriate active p2pSession based on the InfoHash
// in the header.
func StartListen(cfg *common.Config) (conChan chan *P2pConn, listenPort int, err error) {
	listener, err := CreateListener(cfg)
	if err != nil {
		return
	}
	conChan = make(chan *P2pConn)
	go func(cfg *common.Config, conChan chan *P2pConn) {
		for {
			var conn net.Conn
			conn, err := listener.Accept()
			if err != nil {
				log.Println("Listener accept failed:", err)
				continue
			}
			h, err := readHeader(conn)
			if err != nil {
				log.Println("Error reading header: ", err)
				continue
			}

			if err := h.validate(cfg); err != nil {
				log.Println("header auth failed:", err)
				continue
			}

			conChan <- &P2pConn{
				conn:       conn,
				upstream:   false,
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
		log.Fatal("Listen failed:", err)
	}

	log.Println("Listening for peers on port:", cfg.Net.DataPort)
	return
}

// reading header info
func readHeader(conn net.Conn) (h *Header, err error) {
	h = &Header{}

	lenbs := make([]byte, 4)
	_, err = conn.Read(lenbs)
	if err != nil {
		err = fmt.Errorf("Couldn't read 4st byte: %v", err)
		return
	}
	lenbuf := bytes.NewBuffer(lenbs)
	var len int32
	err = binary.Read(lenbuf, binary.BigEndian, &len)
	if err != nil {
		err = fmt.Errorf("Read length error: %v", err)
		return
	}
	if len <= 0 || len > 200 {
		err = fmt.Errorf("read length is invalid: %v", len)
		return
	}

	bs := make([]byte, len)
	_, err = conn.Read(bs)
	if err != nil {
		err = fmt.Errorf("Couldn't read auth info: %v", err)
		return
	}

	h.Len = len
	buf := bytes.NewBuffer(bs)

	// taskId
	if h.TaskId, err = buf.ReadString(byte(0x00)); err != nil {
		err = fmt.Errorf("Read taskId error: %v", err)
		return
	}

	// username
	if h.Username, err = buf.ReadString(byte(0x00)); err != nil {
		err = fmt.Errorf("Read username error: %v", err)
		return
	}

	// password
	if h.Passowrd, err = buf.ReadString(byte(0x00)); err != nil {
		err = fmt.Errorf("Read password error: %v", err)
		return
	}

	// seed
	if h.Seed, err = buf.ReadString(byte(0x00)); err != nil {
		err = fmt.Errorf("Read password error: %v", err)
		return
	}

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

	binary.Write(buf, binary.BigEndian, blen)
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
