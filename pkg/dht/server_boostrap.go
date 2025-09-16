package dht

import (
	"bytes"
	"context"
	"crypto/rand"
	"sort"
	"sync"
	"time"

	"github.com/lyyyuna/zhongzi-go/pkg/log"
	. "github.com/lyyyuna/zhongzi-go/pkg/types"
)

func (d *DHTServer) Bootstrap(ctx context.Context) {
	log.Info("Starting DHT bootstrap process")

	myid := generateRandomNodeId()

	peers := make([]*Node, 0)
	for _, addr := range d.bootstrapNodes {
		peers = append(peers, NewNode(myid, &addr))
	}

	knowns := make(map[NodeId]*Node)
	for {
		select {
		case <-ctx.Done():
			log.Info("Stopping DHT bootstrap")
			return
		default:
		}

		candidates := make(map[NodeId]*Node)
		var l sync.Mutex

		var wg sync.WaitGroup
		wg.Add(len(peers))
		for _, peer := range peers {
			go func() {
				defer wg.Done()
				ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
				defer cancel()

				infohash := Infohash(myid)
				nodes, err := d.findNode(ctx, peer.Addr, &infohash)
				if err != nil {
					log.Errorf("Failed to find node: %v", err)
					return
				}

				for _, node := range nodes {
					d.routingTable.Add(node)
					l.Lock()
					candidates[node.Id] = &node
					l.Unlock()
				}
			}()
		}

		wg.Wait()

		diffs := []*Node{}
		for id, node := range candidates {
			if _, ok := knowns[id]; !ok {
				diffs = append(diffs, node)
			}
		}

		sort.Slice(diffs, func(i, j int) bool {
			idistance := diffs[i].DistanceTo(NodeId(d.Id))
			jdistance := diffs[j].DistanceTo(NodeId(d.Id))

			return bytes.Compare(idistance[:], jdistance[:]) < 0
		})

		if len(diffs) > 20 {
			diffs = diffs[:20]
		}

		if len(diffs) != 0 {
			log.Infof("Found %d new nodes", len(diffs))
			for _, node := range diffs {
				knowns[node.Id] = node
			}
			peers = diffs
		} else {
			log.Infof("No new nodes found")
			break
		}

		if len(knowns) > d.maxBootstrapNode {
			log.Info("Reached maximum number of nodes")
			break
		}

		time.Sleep(time.Millisecond * 100)
	}

	log.Infof("DHT bootstrap process completed with total %d nodes", len(knowns))
}

func generateRandomNodeId() NodeId {
	var id NodeId
	rand.Read(id[:])
	return id
}
