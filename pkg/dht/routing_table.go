package dht

import (
	"bytes"
	"sort"
	"time"

	"github.com/jellydator/ttlcache/v3"
	. "github.com/lyyyuna/zhongzi-go/pkg/types"
)

type RoutingTable struct {
	nodes *ttlcache.Cache[NodeId, Node]
}

func NewRoutingTable() *RoutingTable {
	r := &RoutingTable{
		nodes: ttlcache.New(
			ttlcache.WithTTL[NodeId, Node](time.Minute * 10),
		),
	}

	go r.nodes.Start()

	return r
}

func (r *RoutingTable) Add(newnode Node) {
	r.nodes.Set(newnode.Id, newnode, time.Minute*10)
}

func (r *RoutingTable) GetClosestNodes(targetId NodeId, count int) []Node {
	nodes := make([]Node, 0)

	for _, item := range r.nodes.Items() {
		node := item.Value()
		nodes = append(nodes, node)
	}

	sort.Slice(nodes, func(i, j int) bool {
		idistance := nodes[i].DistanceTo(targetId)
		jdistance := nodes[j].DistanceTo(targetId)

		return bytes.Compare(idistance[:], jdistance[:]) < 0
	})

	if count < len(nodes) {
		nodes = nodes[:count]
	}

	return nodes
}

func (r *RoutingTable) Remove(nodeId NodeId) {
	r.nodes.Delete(nodeId)
}
