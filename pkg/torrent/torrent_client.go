package torrent

import (
	"context"
	"math/rand"
	"net"
	"sync"
	"time"

	"github.com/lyyyuna/zhongzi-go/pkg/dht"
	"github.com/lyyyuna/zhongzi-go/pkg/log"
	"github.com/lyyyuna/zhongzi-go/pkg/types"
	"github.com/panjf2000/ants/v2"
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
	id := types.Infohash(calculatePeerID())
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

func calculatePeerID() [20]byte {
	var peerID [20]byte
	copy(peerID[:], "-PC0001-")

	for i := 8; i < 20; i++ {
		peerID[i] = byte('0' + rand.Intn(10))
	}

	return peerID
}

func (tc *TorrentClient) Start(ctx context.Context) {
	go tc.collectingPeers(ctx)

	go tc.download(ctx)

	tc.fileSaver(ctx)
}

func (tc *TorrentClient) collectingPeers(ctx context.Context) {
	dht := dht.NewDHTServer(dht.WithMaxBootstrapNodes(100))
	dht.Run()

	for {
		tc.availablePeersLock.Lock()
		peersCnt := len(tc.availablePeers)

		allRemotePieces := make(map[int]int)
		for _, peer := range tc.availablePeers {
			for pieceIndex := range peer.remotePieces {
				if allRemotePieces[pieceIndex] == 0 {
					allRemotePieces[pieceIndex] = 1
				} else {
					allRemotePieces[pieceIndex]++
				}
			}
		}

		needBootstrap := false
		for i := range tc.torrent.Pieces {
			if cnt, ok := allRemotePieces[i]; !ok {
				needBootstrap = true
				log.Infof("piece %v not available, need collecting new peers...", i)
			} else {
				if cnt == 1 {
					needBootstrap = true
					log.Infof("piece %v peer too little, only 1, need collecting new peers...", i)
				}
			}
		}

		tc.availablePeersLock.Unlock()

		if peersCnt > 15 && !needBootstrap {
			log.Infof("available peers is sufficient: %v, all piece available: %v, skipping dht bootstrap.", peersCnt, needBootstrap)
			time.Sleep(10 * time.Second)
			continue
		}

		log.Infof("collecting peers from DHT...")

		dht.Bootstrap(ctx)

		peers := dht.GetPeers(ctx, tc.infoHash)
		log.Infof("found %v peers from DHT", len(peers))

		diffs := make(map[string]*net.TCPAddr)
		for key, peerAddr := range peers {
			if tc.isPeerInAvailablePeers(peerAddr) {
				continue
			}
			diffs[key] = peerAddr
		}

		pool, _ := ants.NewPool(20)

		for _, peerAddr := range diffs {
			pool.Submit(func() {
				peer := newPeer(tc.id, tc.infoHash, peerAddr, len(tc.torrent.Pieces))

				err := peer.connect(ctx)
				if err != nil {
					log.Errorf("skip, connect to peer %s failed: %v", peerAddr.String(), err)
					return
				}

				go peer.run()

				tc.availablePeersLock.Lock()
				tc.availablePeers = append(tc.availablePeers, peer)
				tc.availablePeersLock.Unlock()
			})
		}
	}
}

func (tc *TorrentClient) isPeerInAvailablePeers(targetPeer *net.TCPAddr) bool {
	tc.availablePeersLock.Lock()
	defer tc.availablePeersLock.Unlock()

	for _, peer := range tc.availablePeers {
		if peer.peerAddr.String() == targetPeer.String() {
			return true
		}
	}
	return false
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
	for i := range 100 {
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
				if ipeer.peerAddr.String() == peer.peerAddr.String() {
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
				log.Infof("choose peer %v for piece %v", peer.peerAddr, pieceIndex)
				return peer
			}
		}

		tc.availablePeersLock.Unlock()

		log.Infof("no available peer for piece %v, retrying...", pieceIndex)
		time.Sleep(10 * time.Second)
	}
}
