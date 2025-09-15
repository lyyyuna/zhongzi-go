package torrent

import (
	"context"
	"encoding/hex"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/lyyyuna/zhongzi-go/pkg/dht"
	"github.com/lyyyuna/zhongzi-go/pkg/log"
	"github.com/lyyyuna/zhongzi-go/pkg/types"
)

type TorrentClient struct {
	id *types.Infohash

	infoHash *types.Infohash

	torrent *Torrent

	availablePeersLock sync.Mutex
	availablePeers     []*Peer

	pieceDownloadQueue chan *TorrentPiece
	pieceFileQueue     chan *TorrentPiece

	downloadPath string
}

type o func(tc *TorrentClient)

func WithDownloadPath(path string) o {
	return func(tc *TorrentClient) {
		tc.downloadPath = path
	}
}

func NewTorrentClient(torrent *Torrent, opts ...o) *TorrentClient {
	digits := make([]string, 12)
	for i := range digits {
		digits[i] = strconv.Itoa(rand.Intn(10))
	}

	var result [20]byte
	hex.Decode(result[:], []byte("-PC0001-"+strings.Join(digits, "")))
	id := types.Infohash(result)
	tc := &TorrentClient{
		id:                 &id,
		infoHash:           &torrent.InfoHash,
		torrent:            torrent,
		availablePeers:     make([]*Peer, 0),
		pieceDownloadQueue: make(chan *TorrentPiece, len(torrent.Pieces)),
		pieceFileQueue:     make(chan *TorrentPiece, 1),
	}

	for _, o := range opts {
		o(tc)
	}

	return tc
}

func (tc *TorrentClient) Start(ctx context.Context) {
	go tc.collectingPeers(ctx)

	go tc.download(ctx)

	tc.fileSaver(ctx)
}

func (tc *TorrentClient) collectingPeers(ctx context.Context) {
	dht := dht.NewDHTServer(dht.WithMaxBootstrapNodes(20))
	dht.Run()

	for {
		tc.availablePeersLock.Lock()
		peersCnt := len(tc.availablePeers)
		tc.availablePeersLock.Unlock()
		if peersCnt > 15 {
			log.Infof("available peers is sufficient: %v, skipping dht bootstrap.", peersCnt)
			time.Sleep(10 * time.Second)
			continue
		}

		log.Infof("collecting peers from DHT...")

		dht.Bootstrap(ctx)

		peers := dht.GetPeers(ctx, tc.infoHash)
		log.Infof("found %v peers from DHT", len(peers))

		for _, peerAddr := range peers {
			peer := newPeer(tc.id, tc.infoHash, peerAddr, len(tc.torrent.Pieces))

			err := peer.connect(ctx)
			if err != nil {
				log.Errorf("skip, connect to peer %s failed: %v", peerAddr.String(), err)
				continue
			}

			go peer.run()

			tc.availablePeersLock.Lock()
			tc.availablePeers = append(tc.availablePeers, peer)
			tc.availablePeersLock.Unlock()
			log.Infof("try to connect to peer 2 %s", peerAddr.String())
		}
	}
}

func (tc *TorrentClient) pieceGenerator() {
	for _, piece := range tc.torrent.Pieces {
		tc.pieceDownloadQueue <- &piece
	}
}

func (tc *TorrentClient) download(ctx context.Context) {

	for {
		tc.availablePeersLock.Lock()
		peersCnt := len(tc.availablePeers)
		tc.availablePeersLock.Unlock()
		if peersCnt > 0 {
			break
		} else {
			time.Sleep(time.Second)
		}

	}

	go tc.pieceGenerator()

	var wg sync.WaitGroup
	for i := range 30 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tc.downloadPieceWorker(ctx, i)
		}()
	}
	wg.Wait()
}

func (tc *TorrentClient) downloadPieceWorker(ctx context.Context, workerIndex int) {
	for piece := range tc.pieceDownloadQueue {
		peer := tc.choosePeer(piece.Index)
		downloadData, err := peer.downloadPiece(ctx, piece)
		if err != nil {
			log.Errorf("peer %v download piece %v error: %v", peer.peerAddr, piece.Index, err)
			tc.availablePeersLock.Lock()
			for i, ipeer := range tc.availablePeers {
				if ipeer == peer {
					// 删除第i个元素
					tc.availablePeers = append(tc.availablePeers[:i], tc.availablePeers[i+1:]...)
					break
				}
			}
			tc.availablePeersLock.Unlock()
			tc.pieceDownloadQueue <- piece
			continue
		}

		log.Infof("[worker %v] downloaded piece %v from peer %v", workerIndex, piece.Index, peer.peerAddr)
		piece.Data = downloadData

		tc.pieceFileQueue <- piece
	}

	log.Infof("piece queue shutdown, worker %v stopped", workerIndex)
}

func (tc *TorrentClient) choosePeer(pieceIndex int) *Peer {
	for {
		tc.availablePeersLock.Lock()

		rand.Shuffle(len(tc.availablePeers), func(i, j int) {
			tc.availablePeers[i], tc.availablePeers[j] = tc.availablePeers[j], tc.availablePeers[i]
		})

		for _, peer := range tc.availablePeers {
			if !peer.canDownload() {
				continue
			}

			if peer.hasPiece(pieceIndex) {
				tc.availablePeersLock.Unlock()
				return peer
			}
		}

		tc.availablePeersLock.Unlock()

		log.Infof("no available peer for piece %v, retrying...", pieceIndex)
		time.Sleep(10 * time.Second)
	}
}

func (tc *TorrentClient) fileSaver(ctx context.Context) error {
	downloadedPiece := 0

	f, err := os.OpenFile(filepath.Join(tc.downloadPath, tc.torrent.Name), os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Errorf("open file failed: %v", err)
		return err
	}

	for piece := range tc.pieceFileQueue {
		select {
		case <-ctx.Done():
			log.Infof("file saver shutdown")
			return ctx.Err()
		default:
		}

		_, err := f.Seek(int64(piece.Index*tc.torrent.PieceLength), 0)
		if err != nil {
			log.Errorf("seek file failed: %v", err)
			return err
		}

		_, err = f.Write(piece.Data)
		if err != nil {
			log.Errorf("write file failed: %v", err)
			return err
		}

		err = f.Sync()
		if err != nil {
			log.Errorf("sync file failed: %v", err)
			return err
		}

		log.Infof("piece %v saved, %v/%v", piece.Index, downloadedPiece, len(tc.torrent.Pieces))

		downloadedPiece++
		if downloadedPiece == len(tc.torrent.Pieces) {
			log.Infof("download completed")
			return nil
		}
	}

	return nil
}
