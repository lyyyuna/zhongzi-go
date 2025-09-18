package torrent

import (
	"bytes"
	"context"
	"crypto/sha1"
	"errors"
	"fmt"
	"time"

	"github.com/lyyyuna/zhongzi-go/pkg/log"
)

func (p *Peer) IsBusy() bool {
	return len(p.downloadSema) > (10 - 1)
}

func (p *Peer) downloadPiece(ctx context.Context, piece *TorrentPiece) ([]byte, error) {
	var buf bytes.Buffer

	for _, block := range piece.Blocks {
		p.downloadSema <- struct{}{}
		ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
		data, err := p.getPiece(ctx, piece.Index, block.Offset, block.Length)
		<-p.downloadSema
		if err != nil {
			cancel()
			log.Errorf("get piece failed: %v", err)
			return nil, err
		}
		cancel()

		buf.Write(data)
	}

	pieceData := buf.Bytes()

	checksum := sha1.Sum(pieceData)

	if checksum != piece.Checksum {
		return nil, errors.New("checksum not match")
	}

	log.Infof("downloaded piece %v from %v", piece.Index, p.peerAddr.String())

	p.sendHaveMessage(piece.Index)

	return pieceData, nil
}

func (p *Peer) getPiece(ctx context.Context, pieceIndex int, offset int, length int) ([]byte, error) {
	data := MakeRequestMessage(pieceIndex, offset, length).Encode()

	ch := make(chan []byte, 1)
	p.piecePendingsL.Lock()
	index := fmt.Sprintf("%v-%v", pieceIndex, offset)
	p.piecePendings[index] = ch
	p.piecePendingsL.Unlock()

	log.Infof("send request message to peer %s, piece_index=%v, offset=%v, length=%v", p.peerAddr, pieceIndex, offset, length)

	_, err := p.conn.Write(data)
	if err != nil {
		log.Errorf("send request message failed: %v", err)

		p.piecePendingsL.Lock()
		delete(p.piecePendings, index)
		p.piecePendingsL.Unlock()
		return nil, err
	}

	select {
	case <-ctx.Done():
		p.piecePendingsL.Lock()
		delete(p.piecePendings, index)
		p.piecePendingsL.Unlock()
		return nil, ctx.Err()

	case <-p.closeCh:
		p.piecePendingsL.Lock()
		delete(p.piecePendings, index)
		p.piecePendingsL.Unlock()
		return nil, errors.New("peer connection closed")

	case resp := <-ch:
		return resp, nil
	}
}

func (p *Peer) sendHaveMessage(index int) error {
	log.Infof("sent have message to peer %v", p.peerAddr.String())

	hm := HaveMessage{PieceIndex: index}

	_, err := p.conn.Write(hm.Encode())
	if err != nil {
		log.Errorf("failed to send have message to peer %v", p.peerAddr.String())
		return err
	}

	return nil
}
