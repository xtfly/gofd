package p2p

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"time"

	log "github.com/cihub/seelog"
	"github.com/xtfly/gofd/common"
)

const (
	// 同一地址最大连接次数
	MAX_RETRY_CONNECT_TIMES = 10
)

type P2pSession struct {
	// 全局信息
	g *global

	// 任务信息
	taskId    string
	task      *DispatchTask
	fileStore FileStore

	// 下载过程中的Pieces信息
	pieceSet        *Bitset // 本节点已存在Piece
	totalPieces     int     // 整个Piece个数
	totalSize       int64   // 所有文件大小
	lastPieceLength int     // 最一块Piece的长度
	goodPieces      int     // 已下载的Piece个数

	// 正在下载的Piece
	activePieces map[int]*ActivePiece

	// Peer信息
	addPeerChan     chan *P2pConn
	startChan       chan *StartTask
	peers           map[string]*peer
	peerMessageChan chan peerMessage

	// 重新连接定时器
	retryConnTimeChan <-chan time.Time
	indexInChain      int
	connFailCount     int

	//
	quitChan     chan struct{}
	endedChan    chan struct{}
	stopSessChan chan string // sessionmgnt

	//
	initedAt   time.Time
	startAt    time.Time
	finishedAt time.Time
}

func NewP2pSession(g *global, dt *DispatchTask, stopSessChan chan string) (s *P2pSession, err error) {
	s = &P2pSession{
		g:      g,
		taskId: dt.TaskId,
		task:   dt,

		activePieces: make(map[int]*ActivePiece),
		peers:        make(map[string]*peer),

		addPeerChan:     make(chan *P2pConn, 5), // 不要阻塞
		startChan:       make(chan *StartTask),
		peerMessageChan: make(chan peerMessage, 5),

		quitChan:  make(chan struct{}),
		endedChan: make(chan struct{}),

		stopSessChan: stopSessChan,
	}
	return
}

func (s *P2pSession) init() error {
	log.Infof("[%s] Initing p2p session...", s.taskId)
	fileSystem, err := s.g.fsProvider.NewFS()
	if err != nil {
		return err
	}

	// 初始化存储
	m := s.task.MetaInfo
	s.fileStore, s.totalSize, err = NewFileStore(m, fileSystem)
	if err != nil {
		return err
	}

	s.totalPieces, s.lastPieceLength = countPieces(s.totalSize, m.PieceLen)
	return nil
}

func (s *P2pSession) initInServer() error {
	if err := s.init(); err != nil {
		return err
	}

	s.goodPieces = int(s.totalPieces)
	// 标识服务端都是下载完成的
	s.pieceSet = NewBitset(s.goodPieces)
	for index := 0; index < s.goodPieces; index++ {
		s.pieceSet.Set(index)
	}

	log.Infof("[%s] Inited p2p server session", s.taskId)
	s.initedAt = time.Now()
	return nil
}

func (s *P2pSession) initInClient() error {
	// 客户端与服务端的下载路径不同，修改路径
	for idx, _ := range s.task.MetaInfo.Files {
		s.task.MetaInfo.Files[idx].Path = s.g.cfg.DownDir
	}

	if err := s.init(); err != nil {
		return err
	}

	//计算已经下载的块信息
	{
		var err error
		start := time.Now()
		s.goodPieces, _, s.pieceSet, err = checkPieces(s.fileStore, s.totalSize, s.task.MetaInfo)
		end := time.Now()
		log.Infof("[%s] Computed missing pieces: total(%v), good(%v) (%.2f seconds)", s.taskId,
			s.totalPieces, s.goodPieces, end.Sub(start).Seconds())
		if err != nil {
			return err
		}
	}

	log.Infof("[%s] Inited p2p client session", s.taskId)
	s.initedAt = time.Now()
	return nil
}

func (s *P2pSession) initPeersBitset() {
	// Enlarge any existing peers piece maps
	for _, p := range s.peers {
		if p.have.n != s.totalPieces {
			if p.have.n != 0 {
				log.Error("Expected p.have.n == 0")
				panic("Expected p.have.n == 0")
			}
			p.have = NewBitset(s.totalPieces)
		}
	}
}

func (s *P2pSession) Start(st *StartTask) {
	s.startChan <- st
}

func (s *P2pSession) startImp(st *StartTask) {
	if s.g.cfg.Server {
		s.startAt = time.Now()
		return
	}

	if s.totalPieces == s.goodPieces {
		// 本地文件的Piece与Block都下载完成，不再需要下载
		log.Infof("[%s] All piece has already download.", s.taskId)
		go s.reportStatus(float32(100))
		return
	}

	log.Infof("[%s] Starting p2p session...", s.taskId)
	// 更新路径
	s.task.LinkChain = st.LinkChain

	// 找到分发路径中位置
	net := s.g.cfg.Net
	self := fmt.Sprintf("%s:%v", net.IP, net.DataPort)
	addrs := s.task.LinkChain.DispatchAddrs
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
	s.startAt = time.Now()
	log.Infof("[%s] Started p2p client session", s.taskId)
}

// 寻找可用的地址并连接
func (s *P2pSession) tryNewPeer() {
	addrs := s.task.LinkChain.DispatchAddrs
	if s.connFailCount >= MAX_RETRY_CONNECT_TIMES {
		s.indexInChain--
	}
	if s.indexInChain < 0 {
		s.indexInChain = 0
	}
	peer := addrs[s.indexInChain]
	s.connectToPeer(peer)
}

// 连接其它的Peer
func (s *P2pSession) connectToPeer(peer string) error {
	log.Debugf("[%s] Try connect to peer[%s]", s.taskId, peer)
	conn, err := net.DialTimeout("tcp", peer, 1*time.Second)
	if err != nil {
		log.Errorf("[%s] Failed to connect to peer[%s], error=%v", s.taskId, peer, err)
		conn.Close()
		s.connFailCount++
		s.retryConnTimeChan = time.After(50 * time.Microsecond)
		return err
	}

	// 发送消息头，用于认证
	err = writeHeader(conn, s.taskId, s.g.cfg)
	if err != nil {
		log.Errorf("[%s] Failed to send header to peer[%s], error=%v", s.taskId, peer, err)
		conn.Close()
		s.indexInChain-- //连接下一个
		s.retryConnTimeChan = time.After(50 * time.Microsecond)
		return err
	}

	// 阻塞接收响应
	bs := make([]byte, 1)
	_, err = conn.Read(bs)
	if err != nil {
		// 认证通过了，但没有返回正确的响应，Peer还没创建对应Task的Session
		log.Errorf("[%s] Failed to reading header from peer[%s], error=%v", s.taskId, peer, err)
		conn.Close()
		s.retryConnTimeChan = time.After(50 * time.Microsecond)
		return err
	}

	s.connFailCount = 0
	log.Infof("[%s] Success to connect to peer[%s]", s.taskId, peer)
	p2pconn := &P2pConn{
		conn:       conn,
		client:     false, // 对端是Server
		remoteAddr: conn.RemoteAddr(),
		taskId:     s.taskId,
	}

	s.addPeerImp(p2pconn)
	return nil
}

// 接入其它的Peer连接
func (s *P2pSession) AcceptNewPeer(c *P2pConn) {
	// 先回一个连接响应
	_, err := c.conn.Write([]byte{byte(0xFF)})
	if err != nil {
		log.Errorf("[%s] Write connection init response to peer[%s] failed", s.taskId, c.remoteAddr.String())
		return
	}
	s.addPeerChan <- c
}

// 处理连接到其它成功的Peer，或者是其它Peer的接入
func (s *P2pSession) addPeerImp(c *P2pConn) {
	peerAddr := c.remoteAddr.String()
	log.Infof("[%s] Add new peer, peer[%s]", c.taskId, peerAddr)
	// 创建一个Peer对象
	ps := NewPeer(c, s.task.Speed)

	// 位图
	ps.have = NewBitset(s.totalPieces)
	s.peers[peerAddr] = ps

	// 一个从连接上写消息，或读消息
	go ps.peerWriter(s.peerMessageChan)
	go ps.peerReader(s.peerMessageChan)

	// 连接建立之后， 把自己的位置信息给对端
	if s.pieceSet != nil {
		ps.SendBitfield(s.pieceSet)
	}
}

// 关闭Peer
func (ts *P2pSession) closePeerAndTryReconn(peer *peer) {
	ts.ClosePeer(peer)
	if !peer.client {
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
		log.Infof("[%s] Forgetting we requested block %v.%v", piece, block)
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

// 接收Peer消息并发送消息
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
	case HAVE: // 处理Peer发送过来的HAVE消息
		log.Debugf("[%s] Recv HAVE from peer[%s] ", p.taskId, p.address)
		if len(message) != 5 {
			return errors.New("Unexpected length")
		}
		n := bytesToUint32(message[1:])
		if n < uint32(p.have.n) {
			p.have.Set(int(n))
			s.RequestBlock(p) // 向请此Peer上请求发送块
		} else {
			return errors.New("have index is out of range")
		}
	case BITFIELD: // 处理Peer发送过来的BITFIELD消息
		log.Debugf("[%s] Recv BITFIELD from peer[%s] isclient=%v", p.taskId, p.address, p.client)
		p.have = NewBitsetFromBytes(s.totalPieces, message[1:])
		if p.have != nil {
			if !p.client {
				s.RequestBlock(p) // 向Server Peer请求发送块
			}
		} else {
			return errors.New("Invalid bitfield data")
		}
	case REQUEST: // 处理Peer发送过来的REQUEST消息
		log.Debugf("[%s] Recv REQUEST from peer[%s] ", p.taskId, p.address)
		index, begin, length, err := s.decodeRequest(message, p)
		if err != nil {
			return err
		}
		// TODO go sendPiece可能产生Race问题
		go s.sendPiece(p, index, begin, length)
	case PIECE: // 处理Peer发送过来的PIECE消息
		log.Debugf("[%s] Recv PIECE from peer[%s]", p.taskId, p.address)
		index, begin, length, err := s.decodePiece(message, p)
		if err != nil {
			return err
		}

		if s.pieceSet.IsSet(int(index)) {
			log.Debugf("[%s] Recv PIECE from peer[%s] is already", p.taskId, p.address)
			break //  本Peer已存在此Piece，则继续
		}

		globalOffset := int64(index)*s.task.MetaInfo.PieceLen + int64(begin)
		_, err = s.fileStore.WriteAt(message[9:], globalOffset)
		if err != nil {
			return err
		}

		// 存储块的信息
		s.RecordBlock(p, index, begin, uint32(length))
		err = s.RequestBlock(p) // 继续向此Peer请求发送块信息
	default:
		return fmt.Errorf("Uknown message id: %d\n", messageID)
	}

	return
}

func (s *P2pSession) decodeRequest(message []byte, p *peer) (index, begin, length uint32, err error) {
	if len(message) != 13 {
		err = errors.New("Unexpected message length")
		return
	}
	index = bytesToUint32(message[1:5])
	begin = bytesToUint32(message[5:9])
	length = bytesToUint32(message[9:13])
	if index >= uint32(p.have.n) {
		err = errors.New("piece out of range")
		return
	}
	if !s.pieceSet.IsSet(int(index)) {
		err = errors.New("we don't have that piece")
		return
	}
	if int64(begin) >= s.task.MetaInfo.PieceLen {
		err = errors.New("begin out of range")
		return
	}
	if int64(begin)+int64(length) > s.task.MetaInfo.PieceLen {
		err = errors.New("begin + length out of range")
		return
	}
	return
}

// 给Peer发送块消息
func (s *P2pSession) sendPiece(p *peer, index, begin, length uint32) (err error) {
	log.Debugf("[%s] Sending block to peer[%s], index=%v, begin=%v, length=%v",
		s.taskId, p.address, index, begin, length)
	buf := make([]byte, length+9)
	buf[0] = PIECE
	uint32ToBytes(buf[1:5], index)
	uint32ToBytes(buf[5:9], begin)
	_, err = s.fileStore.ReadAt(buf[9:],
		int64(index)*s.task.MetaInfo.PieceLen+int64(begin))
	if err != nil {
		return
	}
	p.sendMessage(buf)

	return
}

// 接收块消息
func (s *P2pSession) RecordBlock(p *peer, piece, begin, length uint32) (err error) {
	block := begin / STANDARD_BLOCK_LENGTH
	log.Debugf("[%s] Received block from peer[%s] %v.%v", s.taskId, p.address, piece, block)

	requestIndex := (uint64(piece) << 32) | uint64(begin)
	delete(p.ourRequests, requestIndex)
	v, ok := s.activePieces[int(piece)]
	if !ok {
		log.Debugf("[%s] Received a block we already have from peer[%], piece=%v.%v", s.taskId, p.address, piece, block)
		return
	}

	v.recordBlock(int(block))
	if !v.isComplete() {
		return
	}

	// Piece完成下载，清理资源，提交文件
	delete(s.activePieces, int(piece))
	var pieceBytes []byte
	ok, err, pieceBytes = checkPiece(s.fileStore, s.totalSize, s.task.MetaInfo, int(piece))
	if !ok || err != nil {
		log.Errorf("[%s] Closing peer[%s] that sent a bad piece=%v, error=%v", s.taskId, p.address, piece, err)
		go s.reportStatus(float32(-1))
		p.Close()
		return
	}

	// 提交文件存储
	s.fileStore.Commit(int(piece), pieceBytes, s.task.MetaInfo.PieceLen*int64(piece))
	s.pieceSet.Set(int(piece))
	s.goodPieces++

	var percentComplete float32
	if s.totalPieces > 0 {
		percentComplete = float32(s.goodPieces*100) / float32(s.totalPieces)
	}
	log.Debugf("[%s] Have %v of %v pieces %v%% complete", s.taskId, s.goodPieces, s.totalPieces,
		percentComplete)
	if s.goodPieces == s.totalPieces {
		s.finishedAt = time.Now() // 下载完成
	}
	go s.reportStatus(percentComplete)

	// 每当客户端下载了一个piece，即将该piece的下标作为have消息的负载构造have消息，
	// 并把该消息发送给所有建立连接的Client Peer。
	for _, p := range s.peers {
		if p.have != nil && p.client &&
			(int(piece) >= p.have.n || !p.have.IsSet(int(piece))) {
			p.SendHave(piece)
		}
	}

	return
}

func (s *P2pSession) decodePiece(message []byte, p *peer) (index, begin, length uint32, err error) {
	if len(message) < 9 {
		err = errors.New("unexpected message length")
		return
	}
	index = bytesToUint32(message[1:5])
	begin = bytesToUint32(message[5:9])
	length = uint32(len(message) - 9)

	if index >= uint32(p.have.n) {
		err = errors.New("piece out of range")
		return
	}

	if int64(begin) >= s.task.MetaInfo.PieceLen {
		err = errors.New("begin out of range")
		return
	}
	if int64(begin)+int64(length) > s.task.MetaInfo.PieceLen {
		err = errors.New("begin + length out of range")
		return
	}
	if length > MAX_BLOCK_LENGTH {
		err = errors.New("Block length too large")
		return
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
		// 本Peer没有，但其它Peer存在时
		if (!s.pieceSet.IsSet(i)) && p.have.IsSet(i) {
			if _, ok := s.activePieces[i]; !ok {
				return i
			}
		}
	}
	return -1
}

// 构建请求块（本Peer缺失）信息
func (s *P2pSession) RequestBlock(p *peer) (err error) {
	for k := range s.activePieces {
		if p.have.IsSet(k) {
			err = s.requestBlock2(p, k, false)
			if err != io.EOF {
				return
			}
		}
	}

	// No active pieces. (Or no suitable active pieces.) Pick one
	piece := s.ChoosePiece(p)
	if piece < 0 {
		for k := range s.activePieces {
			if p.have.IsSet(k) {
				err = s.requestBlock2(p, k, true)
				if err != io.EOF {
					return
				}
			}
		}
	}

	// 所有piece与block都下载完成了
	if piece < 0 {
		return
	}

	pieceLength := s.pieceLength(piece)
	pieceCount := (pieceLength + STANDARD_BLOCK_LENGTH - 1) / STANDARD_BLOCK_LENGTH
	s.activePieces[piece] = &ActivePiece{make([]int, pieceCount), pieceLength}
	return s.requestBlock2(p, piece, false)

}

func (ts *P2pSession) requestBlock2(p *peer, piece int, endGame bool) (err error) {
	v := ts.activePieces[piece]
	block := v.chooseBlockToDownload(endGame)
	if block >= 0 {
		ts.requestBlockImp(p, piece, block)
	} else {
		//log.Debugf("[%s] Request block from peer[%s], EOF", ts.taskId, p.address)
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

	log.Debugf("[%s] Requesting block from peer[%s], piece=%v.%v, length=%v", ts.taskId, p.address, piece, block, length)
	p.SendRequest(piece, begin, length)
	return
}

func (ts *P2pSession) pieceLength(piece int) int {
	if piece < ts.totalPieces-1 {
		return int(ts.task.MetaInfo.PieceLen)
	}
	return ts.lastPieceLength
}

func (ts *P2pSession) Quit() (err error) {
	select {
	case ts.quitChan <- struct{}{}:
	case <-ts.endedChan: // 接收到close才算真正quit
	}
	return
}

func (ts *P2pSession) shutdown() (err error) {
	for _, peer := range ts.peers {
		ts.ClosePeer(peer)
	}

	if ts.fileStore != nil {
		err = ts.fileStore.Close()
		if err != nil {
			log.Errorf("[%s] Error closing filestore : %v", ts.taskId, err)
		}
	}
	close(ts.endedChan)
	return
}

// 初始化
func (ts *P2pSession) Init() {
	// 开启缓存
	if ts.fileStore != nil {
		cache := ts.g.cacher.NewCache(ts.taskId, ts.totalPieces, int(ts.task.MetaInfo.PieceLen), ts.totalSize)
		ts.fileStore.SetCache(cache)
	}

	if ts.g.cfg.Server {
		if err := ts.initInServer(); err != nil {
			log.Errorf("[%s] Init p2p server session failed, %v", ts.taskId, err)
		}
	} else {
		if err := ts.initInClient(); err != nil {
			log.Errorf("[%s] Init p2p client session failed, %v", ts.taskId, err)
		}
	}

	keepAliveChan := time.Tick(60 * time.Second)

	for {
		select {
		case conn := <-ts.addPeerChan:
			ts.addPeerImp(conn)
		case st := <-ts.startChan:
			ts.startImp(st)
		case pm := <-ts.peerMessageChan:
			peer, message := pm.peer, pm.message
			peer.lastReadTime = time.Now()
			err2 := ts.DoMessage(peer, message)
			if err2 != nil {
				if err2 != io.EOF {
					log.Error("[", ts.taskId, "] Closing peer[", peer.address, "] because ", err2)
					ts.closePeerAndTryReconn(peer)
				} else {
					ts.ClosePeer(peer)
				}
			}
		case <-keepAliveChan:
			if ts.timeout() {
				// Session超时没有启动，需要stop
				ts.stopSessChan <- ts.taskId
				log.Info("[", ts.taskId, "] P2p session is timeout")
			}
			ts.peersKeepAlive()
		case <-ts.retryConnTimeChan:
			ts.tryNewPeer()
		case <-ts.quitChan:
			log.Info("[", ts.taskId, "] Quit p2p session")
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
			log.Error("[", ts.taskId, "] Timing out request of ", piece, ".", block)
			ts.removeRequest(piece, block)
		}
	}
	return
}

func (ts *P2pSession) peersKeepAlive() {
	now := time.Now()
	for _, peer := range ts.peers {
		if peer.lastReadTime.Second() != 0 && now.Sub(peer.lastReadTime) > 3*time.Minute {
			log.Error("[", ts.taskId, "] Closing peer [", peer.address, "] because timed out")
			ts.ClosePeer(peer)
			continue
		}
		err2 := ts.doCheckRequests(peer)
		if err2 != nil {
			if err2 != io.EOF {
				log.Error("[", ts.taskId, "] Closing peer[", peer.address, "] because", err2)
			}
			ts.ClosePeer(peer)
			continue
		}
		peer.keepAlive(now)
	}
}

// 检查是否超时了
func (ts *P2pSession) timeout() bool {
	now := time.Now()
	if ts.startAt.IsZero() && now.Sub(ts.initedAt) >= 3*time.Minute {
		return true
	}

	if !ts.finishedAt.IsZero() && now.Sub(ts.finishedAt) >= 3*time.Minute {
		return true
	}
	return false
}

func (ts *P2pSession) reportStatus(pecent float32) {
	csr := ClientStatusReport{
		TaskId:          ts.taskId,
		IP:              ts.g.cfg.Net.IP,
		PercentComplete: pecent,
	}
	bytes, err := json.Marshal(csr)
	if err != nil {
		log.Errorf("[%s] Report session status failed. error=%v", ts.taskId, err)
		return
	}

	_, err = common.SendHttpReq(ts.g.cfg, "POST", ts.task.LinkChain.ServerAddr,
		"/api/v1/server/tasks/status", bytes)
	if err != nil {
		log.Errorf("[%s] Report session status failed. error=%v", ts.taskId, err)
		select {
		case <-time.After(1 * time.Second):
			ts.reportStatus(pecent)
		}
		return
	}
	return
}
