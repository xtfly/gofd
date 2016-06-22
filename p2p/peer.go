package p2p

import (
	"io"
	"net"
	"time"

	log "github.com/Sirupsen/logrus"
)

const (
	// 最多从其它Peer发送请求
	MAX_OUR_REQUESTS = 5

	// 最多允许其它Peer向本Peer请求
	MAX_PEER_REQUESTS = 10
)

// 下载端
type peer struct {
	taskId   string   // 任务标识
	address  string   // 对端地址
	conn     net.Conn // 物理连接
	upstream bool     // 是否是上游节点，即连接到其它的节点，而不是其它节点连入本节点

	writeChan         chan []byte
	writeQueueingChan chan []byte

	lastReadTime time.Time
	have         *Bitset // 对端已有的Piece

	peerRequests map[uint64]bool
	ourRequests  map[uint64]time.Time // What we requested, when we requested it
}

type peerMessage struct {
	peer    *peer
	message []byte // nil means an error occurred
}

func NewPeer(conn *P2pConn) *peer {
	writeChan := make(chan []byte)
	writeQueueingChan := make(chan []byte)
	go queueingWriter(writeChan, writeQueueingChan)
	return &peer{
		conn:              conn.conn,
		upstream:          conn.upstream,
		address:           conn.remoteAddr.String(),
		writeChan:         writeChan,
		writeQueueingChan: writeQueueingChan,
		peerRequests:      make(map[uint64]bool, MAX_PEER_REQUESTS),
		ourRequests:       make(map[uint64]time.Time, MAX_OUR_REQUESTS),
	}
}

func queueingWriter(in, out chan []byte) {
	queue := make(map[int][]byte)
	head, tail := 0, 0
L:
	for {
		if head == tail {
			select {
			case m, ok := <-in:
				if !ok {
					break L
				}
				queue[head] = m
				head++
			}
		} else {
			select {
			case m, ok := <-in:
				if !ok {
					break L
				}
				queue[head] = m
				head++
			case out <- queue[tail]:
				delete(queue, tail)
				tail++
			}
		}
	}
	// We throw away any messages waiting to be sent, including the
	// nil message that is automatically sent when the in channel is closed
	close(out)
}

func (p *peer) Close() {
	log.Info("Closing connection to", p.address)
	p.conn.Close()
}

func (p *peer) sendMessage(b []byte) {
	p.writeChan <- b
}

func (p *peer) keepAlive(now time.Time) {
	p.sendMessage([]byte{})
}

// There's two goroutines per peer, one to read data from the peer, the other to
// send data to the peer.

func writeNBOUint32(conn net.Conn, n uint32) (err error) {
	var buf []byte = make([]byte, 4)
	uint32ToBytes(buf, n)
	_, err = conn.Write(buf[0:])
	return
}

func readNBOUint32(conn net.Conn) (n uint32, err error) {
	var buf [4]byte
	_, err = conn.Read(buf[0:])
	if err != nil {
		return
	}
	n = bytesToUint32(buf[0:])
	return
}

// This func is designed to be run as a goroutine. It
// listens for messages on a channel and sends them to a peer.

func (p *peer) peerWriter(errorChan chan peerMessage) {
	log.Info("[", p.taskId, "] Writing messages")
	var lastWriteTime time.Time

	for msg := range p.writeQueueingChan {
		now := time.Now()
		if len(msg) == 0 {
			// This is a keep-alive message.
			if now.Sub(lastWriteTime) < 2*time.Minute {
				// Don't need to send keep-alive because we have recently sent a
				// message to this peer.
				continue
			}
			log.Debug("[", p.taskId, "] Sending keep alive", p)
		}
		lastWriteTime = now

		log.Debug("[", p.taskId, "] Writing", uint32(len(msg)), p.conn.RemoteAddr())
		err := writeNBOUint32(p.conn, uint32(len(msg)))
		if err != nil {
			log.Println(err)
			break
		}
		_, err = p.conn.Write(msg)
		if err != nil {
			log.Error("[", p.taskId, "] Failed to write a message", p.address, len(msg), msg, err)
			break
		}
	}

	log.Info("[", p.taskId, "] peerWriter exiting")
	errorChan <- peerMessage{p, nil}
}

// This func is designed to be run as a goroutine. It
// listens for messages from the peer and forwards them to a channel.

func (p *peer) peerReader(msgChan chan peerMessage) {
	log.Debug("[", p.taskId, "] Reading messages")
	for {
		var n uint32
		n, err := readNBOUint32(p.conn)
		if err != nil {
			break
		}
		if n > 130*1024 {
			log.Println("[", p.taskId, "] Message size too large: ", n)
			break
		}

		var buf []byte
		if n == 0 {
			// keep-alive - we want an empty message
			buf = make([]byte, 1)
		} else {
			buf = make([]byte, n)
		}

		_, err = io.ReadFull(p.conn, buf)
		if err != nil {
			break
		}
		msgChan <- peerMessage{p, buf}
	}

	msgChan <- peerMessage{p, nil}
	log.Info("[", p.taskId, "] peerReader exiting")
}

// 发送位图
func (p *peer) SendBitfield(bs *Bitset) {
	msg := make([]byte, len(bs.Bytes())+1)
	msg[0] = BITFIELD
	copy(msg[1:], bs.Bytes())
	p.sendMessage(msg)
}
