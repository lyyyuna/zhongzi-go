package dht

import (
	"bytes"
	"context"
	"net"
	"sort"
	"sync"
	"time"

	"github.com/lyyyuna/zhongzi-go/pkg/log"
	"github.com/lyyyuna/zhongzi-go/pkg/types"
)

func (d *DHTServer) GetPeers(ctx context.Context, infoHash *types.Infohash) map[string]*net.TCPAddr {
	peernodes := d.routingTable.GetClosestNodes(types.NodeId(*infoHash), 20)

	results := make(map[string]*net.TCPAddr)
	knowns := make(map[types.NodeId]Node)

	for {
		select {
		case <-ctx.Done():
			log.Info("Stopping DHT bootstrap")
			return nil
		default:
		}

		nodeCandidates := make(map[types.NodeId]Node)
		var nodeCandidatesLock sync.Mutex

		var wg sync.WaitGroup
		wg.Add(len(peernodes))
		for _, peernode := range peernodes {
			go func() {
				defer wg.Done()
				ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
				defer cancel()

				nodes, addrs, err := d.getPeers(ctx, peernode.Addr, infoHash)
				if err != nil {
					log.Errorf("Failed to get peers: %v", err)
					return
				}

				if len(nodes) > 0 { // 如果返回的是 node
					for _, node := range nodes {
						nodeCandidatesLock.Lock()
						nodeCandidates[node.Id] = node
						d.routingTable.Add(node)
						nodeCandidatesLock.Unlock()
					}
				} else { // 如果返回的是 node
					for _, addr := range addrs {
						nodeCandidatesLock.Lock()
						results[addr.String()] = &addr
						nodeCandidatesLock.Unlock()
					}
				}

			}()
		}
		wg.Wait()

		diffs := []Node{}
		for id, node := range nodeCandidates {
			if _, ok := knowns[id]; !ok {
				diffs = append(diffs, node)
			}
		}

		sort.Slice(diffs, func(i, j int) bool {
			idistance := diffs[i].DistanceTo(types.NodeId(*infoHash))
			jdistance := diffs[j].DistanceTo(types.NodeId(*infoHash))

			return bytes.Compare(idistance[:], jdistance[:]) < 0
		})

		if len(diffs) > 20 {
			diffs = diffs[:20]
		}

		if d.maxPeers > 0 && len(results) > d.maxPeers {
			log.Info("Reached maximum number of peers")
			return results
		}

		if len(diffs) > 0 {
			log.Infof("Found %d new diff nodes", len(diffs))
			peernodes = diffs
			for _, node := range diffs {
				knowns[node.Id] = node
			}
		} else {
			log.Infof("got %v peers", len(results))
			return results
		}

		log.Infof("get_peers: found %v new nodes, %v peers", len(knowns), len(results))
	}
}
