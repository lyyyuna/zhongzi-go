package torrent

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/lyyyuna/zhongzi-go/pkg/log"
	"github.com/lyyyuna/zhongzi-go/pkg/types"
)

const (
	Choke = iota
	Unchoke
	Interested
	NotInterested
	Have
	Bitfield
	Request
	Piece
	Cancel
)

const (
	StateRunning = 1 << 1
	StateChoked  = 1 << 2
)

type Peer struct {
	peerId   *types.Infohash
	infoHash *types.Infohash
	peerAddr *net.TCPAddr

	conn net.Conn

	state int

	remotePieces map[int]bool

	piecePendings  map[string]chan []byte
	piecePendingsL sync.Mutex
}

func newPeer(peerId *types.Infohash, infoHash *types.Infohash, peerAddr *net.TCPAddr, pieceCnt int) *Peer {
	return &Peer{
		peerId:        peerId,
		infoHash:      infoHash,
		peerAddr:      peerAddr,
		remotePieces:  make(map[int]bool, pieceCnt),
		piecePendings: make(map[string]chan []byte),
	}
}

func (p *Peer) connect(ctx context.Context) error {
	conn, err := net.DialTimeout("tcp", p.peerAddr.String(), 3*time.Second)
	if err != nil {
		log.Errorf("connect to peer %s failed: %v", p.peerAddr.String(), err)
		return err
	}
	log.Infof("connected to peer %v", p.peerAddr.String())

	p.conn = conn

	err = p.sendHandshake()
	if err != nil {
		log.Errorf("send handshake to peer %s failed: %v", p.peerAddr.String(), err)
		return err
	}

	err = p.sendInterested()
	if err != nil {
		log.Errorf("send interested to peer %s failed: %v", p.peerAddr.String(), err)
		return err
	}

	p.stateStarted()
	p.stateChoked()

	go p.heartbeatLoop(ctx)

	return nil
}

type Handshake struct {
	PstrLen  uint8
	Pstr     [19]byte
	Reserved [8]byte
	InfoHash [20]byte
	PeerID   [20]byte
}

func (p *Peer) sendHandshake() error {
	log.Infof("handshaking with peer %v", p.peerAddr.String())

	handshake := Handshake{
		PstrLen: 19,
		Pstr:    [19]byte{'B', 'i', 't', 'T', 'o', 'r', 'r', 'e', 'n', 't', ' ', 'p', 'r', 'o', 't', 'o', 'c', 'o', 'l'},
	}
	copy(handshake.InfoHash[:], p.infoHash[:])
	copy(handshake.PeerID[:], p.peerId[:])

	var buf bytes.Buffer
	// 按大端序写入所有字段
	binary.Write(&buf, binary.BigEndian, handshake.PstrLen)
	binary.Write(&buf, binary.BigEndian, handshake.Pstr)
	binary.Write(&buf, binary.BigEndian, handshake.Reserved)
	binary.Write(&buf, binary.BigEndian, handshake.InfoHash)
	binary.Write(&buf, binary.BigEndian, handshake.PeerID)

	_, err := p.conn.Write(buf.Bytes())
	if err != nil {
		log.Errorf("send handshake to peer %s failed: %v", p.peerAddr.String(), err)
		return err
	}

	recvData := make([]byte, 68)
	_, err = io.ReadFull(p.conn, recvData)
	if err != nil {
		log.Errorf("recv handshake from peer %s failed: %v", p.peerAddr.String(), err)
		return err
	}

	// 解析接收到的握手消息
	// parts = struct.unpack('>B19s8x20s20s', data)
	if len(recvData) != 68 {
		log.Errorf("handshake response length mismatch, expected 68 bytes, got %d", len(recvData))
		return fmt.Errorf("handshake response length mismatch")
	}

	// 检查协议字符串长度
	pstrLen := recvData[0]
	if pstrLen != 19 {
		log.Errorf("protocol string length mismatch, expected 19, got %d", pstrLen)
		return fmt.Errorf("protocol string length mismatch")
	}

	// 检查协议字符串
	expectedPstr := []byte("BitTorrent protocol")
	if !bytes.Equal(recvData[1:20], expectedPstr) {
		log.Errorf("protocol string mismatch")
		return fmt.Errorf("protocol string mismatch")
	}

	// 提取 info hash (跳过 8 字节保留字段)
	recvInfoHash := recvData[28:48]

	// 检查 info hash
	if !bytes.Equal(recvInfoHash, p.infoHash[:]) {
		log.Error("info hash mismatch")
		return fmt.Errorf("info hash mismatch")
	}

	return nil
}

func (p *Peer) sendInterested() error {
	log.Infof("sending Interested to peer %s", p.peerAddr.String())

	buf := make([]byte, 5) // 4字节长度 + 1字节消息类型

	// 写入长度 (4字节大端序)
	binary.BigEndian.PutUint32(buf[0:4], 1)

	// 写入消息类型 (1字节)
	buf[4] = byte(Interested)

	_, err := p.conn.Write(buf)
	if err != nil {
		log.Errorf("send Interested to peer %s failed: %v", p.peerAddr.String(), err)
		return err
	}

	return nil
}

func (p *Peer) stateStopped() {
	log.Infof("peer %s stopped", p.peerAddr.String())
	p.state = 0
}

func (p *Peer) stateStarted() {
	log.Infof("peer %s started", p.peerAddr.String())
	p.state = StateRunning
}

func (p *Peer) stateChoked() {
	log.Infof("peer %s choked", p.peerAddr.String())
	p.state |= StateChoked
}

func (p *Peer) stateUnchoked() {
	log.Infof("peer %s unchoked", p.peerAddr.String())
	p.state &^= StateChoked
}

func (p *Peer) isStateRunning() bool {
	return p.state&StateRunning != 0
}

func (p *Peer) isStateChoked() bool {
	return p.state&StateChoked != 0
}

func (p *Peer) canDownload() bool {
	return p.isStateRunning() && !p.isStateChoked()
}

func (p *Peer) hasPiece(pieceIndex int) bool {
	_, ok := p.remotePieces[pieceIndex]
	return ok
}

func (p *Peer) heartbeatLoop(ctx context.Context) {
	for {
		if !p.isStateRunning() {
			time.Sleep(10 * time.Second)
			continue
		}

		select {
		case <-ctx.Done():
			log.Infof("heartbeat loop for peer %s ctx canceled: %v", p.peerAddr.String(), ctx.Err())
			return
		default:
		}

		err := p.sendKeepAlive()
		if err != nil {
			log.Errorf("send keep alive to peer %s failed: %v", p.peerAddr.String(), err)
			return
		}

		time.Sleep(60 * time.Second)
	}
}

func (p *Peer) sendKeepAlive() error {
	buf := make([]byte, 4)
	binary.BigEndian.PutUint32(buf, 0)

	_, err := p.conn.Write(buf)
	if err != nil {
		log.Errorf("send keep alive to peer %s failed: %v", p.peerAddr.String(), err)
		return err
	}

	return nil
}

func (p *Peer) run() {
	for {
		buf := make([]byte, 4)
		_, err := io.ReadFull(p.conn, buf)
		if err == io.EOF {
			log.Infof("peer %s closed", p.peerAddr.String())
			return
		} else if err != nil {
			log.Errorf("read peer message length failed: %v", err)
			continue
		}

		length := binary.BigEndian.Uint32(buf)
		if length == 0 {
			log.Infof("received peer %s keep-alive message", p.peerAddr.String())
			continue
		}

		buf = make([]byte, 1)
		_, err = io.ReadFull(p.conn, buf)
		if err == io.EOF {
			log.Infof("peer %s closed", p.peerAddr.String())
		} else if err != nil {
			log.Errorf("read peer message type failed: %v", err)
			continue
		}

		id := int(buf[0])

		data := make([]byte, length-1)
		if length > 1 {
			_, err = io.ReadFull(p.conn, data)
			if err == io.EOF {
				log.Infof("peer %s closed", p.peerAddr.String())
				return
			} else if err != nil {
				log.Errorf("read peer message data failed: %v", err)
				continue
			}
		}

		switch id {
		case Choke:
			log.Infof("received peer %s Choke message", p.peerAddr.String())
			p.stateChoked()
		case Unchoke:
			log.Infof("received peer %s Unchoke message", p.peerAddr.String())
			p.stateUnchoked()
		case Interested:
			log.Infof("received peer %s Interested message", p.peerAddr.String())
		case NotInterested:
			log.Infof("received peer %s NotInterested message", p.peerAddr.String())
		case Have:
			log.Infof("received peer %s Have message", p.peerAddr.String())
			hm := NewHaveMessage(data)
			p.remotePieces[hm.PieceIndex] = true
		case Bitfield:
			log.Infof("received peer %s Bitfield message", p.peerAddr.String())
			bm := NewBitfieldMessage(data)
			pieceIndex := 0
			for _, b := range bm.Bitfield {
				for i := 7; i >= 0; i-- {
					bit := (b >> i) & 1
					if bit == 1 {
						p.remotePieces[pieceIndex] = true
					}
					pieceIndex++
				}
			}
		case Request:
			log.Infof("received peer %s Request message", p.peerAddr.String())
		case Piece:
			log.Infof("received peer %s Piece message", p.peerAddr.String())
			pm := NewPieceMessage(data)
			index := pm.Index
			offset := pm.Begin
			key := fmt.Sprintf("%v-%v", index, offset)
			p.piecePendingsL.Lock()
			if ch, ok := p.piecePendings[key]; ok {
				log.Infof("received peer %s Piece message for piece %v, offset %v", p.peerAddr.String(), index, offset)
				ch <- pm.Block
			} else {
				log.Warnf("received peer %s Piece message for piece %v, offset %v, but no pending request", p.peerAddr.String(), index, offset)
			}
			p.piecePendingsL.Unlock()

		case Cancel:
			log.Infof("received peer %s Cancel message", p.peerAddr.String())
		default:
			log.Warnf("received peer %s unknown message", p.peerAddr.String())
			p.stateStopped()
		}
	}
}
