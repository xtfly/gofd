package p2p

import (
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"time"

	log "github.com/Sirupsen/logrus"
)

const (
	// 同一地址最大连接次数
	MAX_RETRY_CONNECT_TIMES = 10
)

const (
	// 每当客户端下载了一个piece，即将该piece的下标作为have消息的负载构造have消息，并把该消息发送给所有与客户端建立连接的peer
	HAVE = iota

	// 交换位图
	BITFIELD

	// 向该peer发送数据请求
	REQUEST

	// 当客户端收到某个peer的request消息后,客户端已经下载，则发送piece消息将文件数据上传给该peer。
	PIECE
)

type P2pSession struct {
	// 全局信息
	g *global

	// 任务信息
	taskId    string
	m         *MetaInfo
	fileStore FileStore

	// 下载过程中的Pieces信息
	pieceSet        *Bitset // The pieces we have
	totalPieces     int     // 整个Piece个数
	totalSize       int64   // 所有文件大小
	lastPieceLength int     // 最一块Piece的长度
	goodPieces      int     // 已下载的Piece个数

	//
	activePieces map[int]*ActivePiece

	//
	Uploaded   int64 // 已发送的大小
	Downloaded int64 // 已下载的大小
	Left       int64 // 剩余的Piece个数

	//
	trackerReportChan chan ClientStatusReport

	//
	addPeerChan     chan *P2pConn
	peers           map[string]*peer
	peerMessageChan chan peerMessage

	// 重新连接定时器
	retryConnTimeChan <-chan time.Time
	indexInChain      int
	connFailCount     int

	//
	quitChan  chan struct{}
	endedChan chan struct{}
}

func NewP2pSession(g *global, mi *MetaInfo) (s *P2pSession, err error) {
	s = &P2pSession{
		g:            g,
		taskId:       mi.TaskId,
		m:            mi,
		activePieces: make(map[int]*ActivePiece),

		addPeerChan:     make(chan *P2pConn),
		peers:           make(map[string]*peer),
		peerMessageChan: make(chan peerMessage),

		quitChan:  make(chan struct{}),
		endedChan: make(chan struct{}),
	}

	if g.cfg.Server {
		s.initInServer()
	} else {
		s.initInClient()
	}

	return
}

func (s *P2pSession) init() {
	fileSystem, err := s.g.fsProvider.NewFS()
	if err != nil {
		return
	}

	// 初始化存储
	s.fileStore, err = NewFileStore(s.m, fileSystem)
	if err != nil {
		return
	}

	s.totalSize = s.m.Length
	s.totalPieces = int(s.totalSize / s.m.PieceLen)
	s.lastPieceLength = int(s.totalSize % s.m.PieceLen)
	if s.lastPieceLength == 0 { // last piece is a full piece
		s.lastPieceLength = int(s.m.PieceLen)
	} else {
		s.totalPieces++
	}

	if s.pieceSet != nil {
		s.pieceSet = NewBitset(s.totalPieces)
	}
}

func (s *P2pSession) initInServer() {
	// init
	s.init()

	s.goodPieces = int(s.totalPieces)
	s.Left = 0
	s.Downloaded = s.totalSize

	s.initPeersBitset()
}

func (s *P2pSession) initInClient() {
	// init
	s.init()

	// 客户端与服务端的下载路径不同，修改路径
	for _, v := range s.m.Files {
		v.Path = s.g.cfg.DownDir
	}

	//计算已经下载的块信息
	{
		var err error
		start := time.Now()
		s.goodPieces, _, s.pieceSet, err = checkPieces(s.fileStore, s.totalSize, s.m)
		end := time.Now()
		log.Infof("[ %s ] Computed missing pieces (%.2f seconds)\n", s.taskId, end.Sub(start).Seconds())
		if err != nil {
			return
		}
	}

	// 计算剩余块信息
	{
		bad := s.totalPieces - s.goodPieces
		left := int64(bad) * int64(s.m.PieceLen)
		if !s.pieceSet.IsSet(s.totalPieces - 1) {
			left = left - int64(s.m.PieceLen) + int64(s.lastPieceLength)
		}
		s.Left = left
	}

	// 找到分发路径中位置
	self := fmt.Sprintf("%s:%v", s.g.cfg.Net.IP, s.g.cfg.Net.DataPort)
	addrs := s.m.LinkChain.DispatchAddrs
	count := len(addrs)
	for idx := count - 1; idx > 0; idx-- {
		if self == addrs[idx] {
			s.indexInChain = idx - 1
			break
		}
	}
	// 尝试与上一个节点建立连接
	s.tryNewPeer()

	s.initPeersBitset()
}

func (s *P2pSession) initPeersBitset() {
	// Enlarge any existing peers piece maps
	for _, p := range s.peers {
		if p.have.n != s.totalPieces {
			if p.have.n != 0 {
				panic("Expected p.have.n == 0")
			}
			p.have = NewBitset(s.totalPieces)
		}
	}
}

// 连接
func (s *P2pSession) tryNewPeer() {
	addrs := s.m.LinkChain.DispatchAddrs
	if s.connFailCount >= MAX_RETRY_CONNECT_TIMES {
		s.indexInChain--
	}
	if s.indexInChain < 0 {
		s.indexInChain = 0
	}
	peer := addrs[s.indexInChain]
	go s.connectToPeer(peer)
}

func (s *P2pSession) closeConnAndReConn(conn net.Conn) {
	conn.Close()
	s.indexInChain--
	s.tryNewPeer()
}

// 连接其它的Peer
func (s *P2pSession) connectToPeer(peer string) error {
	conn, err := net.DialTimeout("tcp", peer, 1*time.Second)
	if err != nil {
		log.Error("[", s.taskId, "] Failed to connect to", peer, err)
		conn.Close()
		s.connFailCount++
		s.retryConnTimeChan = time.After(50 * time.Microsecond)
		return err
	}

	// 发送消息头，用于认证
	err = writeHeader(conn, s.taskId, s.g.cfg)
	if err != nil {
		log.Error("[", s.taskId, "] Failed to send header to", peer, err)
		s.closeConnAndReConn(conn)
		return err
	}

	// 阻塞接收响应
	bs := make([]byte, 1)
	_, err = conn.Read(bs)
	if err != nil {
		log.Error("[", s.taskId, "] Error reading header: ", err)
		s.closeConnAndReConn(conn)
		return err
	}

	s.connFailCount = 0
	log.Info("[", s.taskId, "] Success to connect to", peer)
	p2pconn := &P2pConn{
		conn:       conn,
		upstream:   true,
		remoteAddr: conn.RemoteAddr(),
		taskId:     s.taskId,
	}

	// 异步加入
	s.AddPeer(p2pconn)
	return nil
}

//  增加其它的Peer
func (s *P2pSession) AddPeer(conn *P2pConn) {
	select {
	case s.addPeerChan <- conn:
	case <-s.endedChan:
	}
}

// 接入其它的Peer连接
func (s *P2pSession) AcceptNewPeer(conn *P2pConn) {
	// 先回一个连接响应
	_, err := conn.conn.Write([]byte{byte(0xFF)})
	if err != nil {
		return
	}
	s.AddPeer(conn)
}

// 处理连接到其它成功的Peer，或者是其它Peer的接入
func (s *P2pSession) addPeerImp(conn *P2pConn) {
	// 创建一个Peer对象
	peerAddr := conn.remoteAddr.String()
	ps := NewPeer(conn)

	// 位图
	ps.have = NewBitset(s.totalPieces)
	s.peers[peerAddr] = ps

	// 一个从连接上写消息，或读消息
	go ps.peerWriter(s.peerMessageChan)
	go ps.peerReader(s.peerMessageChan)

	// 连接建立之后，发送已有的位图信息
	if s.pieceSet != nil {
		ps.SendBitfield(s.pieceSet)
	}
}

// 关闭Peer
func (ts *P2pSession) closePeerAndTryReconn(peer *peer) {
	ts.ClosePeer(peer)
	if peer.upstream {
		ts.tryNewPeer()
	}
}

// 关闭Peer
func (ts *P2pSession) ClosePeer(peer *peer) {
	peer.Close()
	ts.removeRequests(peer)
	delete(ts.peers, peer.address)
}

// 删除REQUEST信息
func (ts *P2pSession) removeRequests(p *peer) (err error) {
	for k := range p.ourRequests {
		piece := int(k >> 32)
		begin := int(k & 0xffffffff)
		block := begin / STANDARD_BLOCK_LENGTH
		log.Info("[", ts.taskId, "] Forgetting we requested block ", piece, ".", block)
		ts.removeRequest(piece, block)
	}
	p.ourRequests = make(map[uint64]time.Time, MAX_OUR_REQUESTS)
	return
}

// 删除REQUEST信息
func (ts *P2pSession) removeRequest(piece, block int) {
	v, ok := ts.activePieces[piece]
	if ok && v.downloaderCount[block] > 0 {
		v.downloaderCount[block]--
	}
}

// 产生并发送消息
func (ts *P2pSession) DoMessage(p *peer, message []byte) (err error) {
	if message == nil {
		return io.EOF // The reader or writer goroutine has exited
	}

	if len(message) == 0 { // keep alive
		return
	}

	err = ts.generalMessage(message, p)
	return
}

func (s *P2pSession) generalMessage(message []byte, p *peer) (err error) {
	messageID := message[0]

	switch messageID {
	case HAVE:
		if len(message) != 5 {
			return errors.New("Unexpected length")
		}
		n := bytesToUint32(message[1:])
		if n < uint32(p.have.n) {
			p.have.Set(int(n))
		} else {
			return errors.New("have index is out of range")
		}
	case BITFIELD:
		log.Info("[", s.taskId, "] bitfield", p.address)
		p.have = NewBitsetFromBytes(s.totalPieces, message[1:])
		if p.have == nil {
			return errors.New("Invalid bitfield data")
		}
	case REQUEST:
		log.Info("[", s.taskId, "] request", p.address)
		if len(message) != 13 {
			return errors.New("Unexpected message length")
		}
		index := bytesToUint32(message[1:5])
		begin := bytesToUint32(message[5:9])
		length := bytesToUint32(message[9:13])
		if index >= uint32(p.have.n) {
			return errors.New("piece out of range")
		}
		if !s.pieceSet.IsSet(int(index)) {
			return errors.New("we don't have that piece")
		}
		if int64(begin) >= s.m.PieceLen {
			return errors.New("begin out of range")
		}
		if int64(begin)+int64(length) > s.m.PieceLen {
			return errors.New("begin + length out of range")
		}
		// TODO: Asynchronous
		return s.sendRequest(p, index, begin, length)
	case PIECE:
		// piece
		if len(message) < 9 {
			return errors.New("unexpected message length")
		}
		index := bytesToUint32(message[1:5])
		begin := bytesToUint32(message[5:9])
		length := len(message) - 9
		if index >= uint32(p.have.n) {
			return errors.New("piece out of range")
		}
		if s.pieceSet.IsSet(int(index)) {
			break // We already have that piece, keep going
		}
		if int64(begin) >= s.m.PieceLen {
			return errors.New("begin out of range")
		}
		if int64(begin)+int64(length) > s.m.PieceLen {
			return errors.New("begin + length out of range")
		}
		if length > 128*1024 {
			return errors.New("Block length too large")
		}

		globalOffset := int64(index)*s.m.PieceLen + int64(begin)
		_, err = s.fileStore.WriteAt(message[9:], globalOffset)
		if err != nil {
			return err
		}

		s.RecordBlock(p, index, begin, uint32(length))
		err = s.RequestBlock(p)

	default:
		return fmt.Errorf("Uknown message id: %d\n", messageID)
	}

	return
}

// 给Peer发送块消息
func (s *P2pSession) sendRequest(peer *peer, index, begin, length uint32) (err error) {
	log.Debug("[", s.taskId, "] Sending block", index, begin, length)
	buf := make([]byte, length+9)
	buf[0] = PIECE
	uint32ToBytes(buf[1:5], index)
	uint32ToBytes(buf[5:9], begin)
	_, err = s.fileStore.ReadAt(buf[9:],
		int64(index)*s.m.PieceLen+int64(begin))
	if err != nil {
		return
	}
	peer.sendMessage(buf)
	s.Uploaded += int64(length)
	return
}

// 接收块消息
func (s *P2pSession) RecordBlock(p *peer, piece, begin, length uint32) (err error) {
	block := begin / STANDARD_BLOCK_LENGTH
	log.Debug("[", s.taskId, "] Received block", piece, ".", block)

	requestIndex := (uint64(piece) << 32) | uint64(begin)
	delete(p.ourRequests, requestIndex)
	v, ok := s.activePieces[int(piece)]
	if ok {
		s.Downloaded += int64(length)

		if v.isComplete() {
			delete(s.activePieces, int(piece))
			var pieceBytes []byte
			ok, err, pieceBytes = checkPiece(s.fileStore, s.totalSize, s.m, int(piece))
			if !ok || err != nil {
				log.Println("[", s.taskId, "] Closing peer that sent a bad piece", piece, err)
				p.Close()
				return
			}

			// 提交文件存储
			s.fileStore.Commit(int(piece), pieceBytes, s.m.PieceLen*int64(piece))
			s.Left -= int64(v.pieceLength)
			s.pieceSet.Set(int(piece))
			s.goodPieces++

			var percentComplete float32
			if s.totalPieces > 0 {
				percentComplete = float32(s.goodPieces*100) / float32(s.totalPieces)
			}
			log.Println("[", s.taskId, "] Have", s.goodPieces, "of", s.totalPieces,
				"pieces", percentComplete, "% complete")
			if s.goodPieces == s.totalPieces {
				// TODO 下载完成，上报状态
				// if !ts.trackerLessMode {
				// 	ts.fetchTrackerInfo("completed")
				// }
				// TODO: Drop connections to all seeders.
			}

			// 每当客户端下载了一个piece，即将该piece的下标作为have消息的负载构造have消息，
			// 并把该消息发送给所有与客户端建立连接的peer。
			for _, p := range s.peers {
				if p.have != nil {
					if int(piece) < p.have.n && p.have.IsSet(int(piece)) {
						// We don't do anything special. We rely on the caller
						// to decide if this peer is still interesting.
					} else {
						haveMsg := make([]byte, 5)
						haveMsg[0] = HAVE
						uint32ToBytes(haveMsg[1:5], piece)
						p.sendMessage(haveMsg)
					}
				}
			}
		}
	} else {
		log.Println("[", s.taskId, "] Received a block we already have.", piece, block, p.address)
	}

	return
}

// 请求下载时，选择一个可用的Piece
func (s *P2pSession) ChoosePiece(p *peer) (piece int) {
	n := s.totalPieces
	start := rand.Intn(n)
	piece = s.checkRange(p, start, n)
	if piece == -1 {
		piece = s.checkRange(p, 0, start)
	}
	return
}

func (s *P2pSession) checkRange(p *peer, start, end int) (piece int) {
	clampedEnd := min(end, min(p.have.n, s.pieceSet.n))
	for i := start; i < clampedEnd; i++ {
		if (!s.pieceSet.IsSet(i)) && p.have.IsSet(i) {
			return i
		}
	}
	return -1
}

// 构建请求块（本Peer缺失）信息
func (s *P2pSession) RequestBlock(p *peer) (err error) {
	for k := range s.activePieces {
		if p.have.IsSet(k) {
			err = s.RequestBlock2(p, k, false)
			if err != io.EOF {
				return
			}
		}
	}
	// No active pieces. (Or no suitable active pieces.) Pick one
	piece := s.ChoosePiece(p)
	if piece < 0 {
		// No unclaimed pieces. See if we can double-up on an active piece
		for k := range s.activePieces {
			if p.have.IsSet(k) {
				err = s.RequestBlock2(p, k, true)
				if err != io.EOF {
					return
				}
			}
		}
	}

	if piece < 0 {
		return
	}

	pieceLength := s.pieceLength(piece)
	pieceCount := (pieceLength + STANDARD_BLOCK_LENGTH - 1) / STANDARD_BLOCK_LENGTH
	s.activePieces[piece] = &ActivePiece{make([]int, pieceCount), pieceLength}
	return s.RequestBlock2(p, piece, false)

}

func (ts *P2pSession) RequestBlock2(p *peer, piece int, endGame bool) (err error) {
	v := ts.activePieces[piece]
	block := v.chooseBlockToDownload(endGame)
	if block >= 0 {
		ts.requestBlockImp(p, piece, block)
	} else {
		return io.EOF
	}
	return
}

// Request or cancel a block
func (ts *P2pSession) requestBlockImp(p *peer, piece int, block int) {
	begin := block * STANDARD_BLOCK_LENGTH

	length := STANDARD_BLOCK_LENGTH
	if piece == ts.totalPieces-1 {
		left := ts.lastPieceLength - begin
		if left < length {
			length = left
		}
	}

	log.Debug("[", ts.taskId, "] Requesting block", piece, ".", block, length)

	req := make([]byte, 13)
	req[0] = byte(REQUEST)
	uint32ToBytes(req[1:5], uint32(piece))
	uint32ToBytes(req[5:9], uint32(begin))
	uint32ToBytes(req[9:13], uint32(length))
	requestIndex := (uint64(piece) << 32) | uint64(begin)

	p.ourRequests[requestIndex] = time.Now()
	p.sendMessage(req)
	return
}
func (ts *P2pSession) pieceLength(piece int) int {
	if piece < ts.totalPieces-1 {
		return int(ts.m.PieceLen)
	}
	return ts.lastPieceLength
}

func (ts *P2pSession) Quit() (err error) {
	select {
	case ts.quitChan <- struct{}{}:
	case <-ts.endedChan:
	}
	return
}

func (ts *P2pSession) shutdown() (err error) {
	close(ts.endedChan)

	for _, peer := range ts.peers {
		ts.ClosePeer(peer)
	}
	if ts.fileStore != nil {
		err = ts.fileStore.Close()
		if err != nil {
			log.Println("[", ts.taskId, "] Error closing filestore:", err)
		}
	}
	return
}

// 启动入口
func (ts *P2pSession) Start() {
	// 开启缓存
	if ts.fileStore != nil {
		cache := ts.g.cacher.NewCache(ts.taskId, ts.totalPieces, int(ts.m.PieceLen), ts.totalSize)
		ts.fileStore.SetCache(cache)
	}

	keepAliveChan := time.Tick(60 * time.Second)

	for {
		select {
		case conn := <-ts.addPeerChan:
			ts.addPeerImp(conn)
		case pm := <-ts.peerMessageChan:
			peer, message := pm.peer, pm.message
			peer.lastReadTime = time.Now()
			err2 := ts.DoMessage(peer, message)
			if err2 != nil {
				if err2 != io.EOF {
					log.Error("[", ts.taskId, "] Closing peer", peer.address, "because", err2)
				}
				ts.closePeerAndTryReconn(peer)
			}
		case <-keepAliveChan:
			now := time.Now()
			for _, peer := range ts.peers {
				if peer.lastReadTime.Second() != 0 && now.Sub(peer.lastReadTime) > 3*time.Minute {
					log.Error("[", ts.m.TaskId, "] Closing peer", peer.address, "because timed out")
					ts.closePeerAndTryReconn(peer)
					continue
				}
				err2 := ts.doCheckRequests(peer)
				if err2 != nil {
					if err2 != io.EOF {
						log.Error("[", ts.taskId, "] Closing peer", peer.address, "because", err2)
					}
					ts.closePeerAndTryReconn(peer)
					continue
				}
				peer.keepAlive(now)
			}
		case <-ts.retryConnTimeChan:
			ts.tryNewPeer()
		case <-ts.quitChan:
			log.Println("[", ts.taskId, "] quit p2p session")
			ts.shutdown()
			return
		}
	}
}

func (ts *P2pSession) doCheckRequests(p *peer) (err error) {
	now := time.Now()
	for k, v := range p.ourRequests {
		if now.Sub(v).Seconds() > 30 {
			piece := int(k >> 32)
			block := int(k&0xffffffff) / STANDARD_BLOCK_LENGTH
			log.Error("[", ts.taskId, "] timing out request of", piece, ".", block)
			ts.removeRequest(piece, block)
		}
	}
	return
}
