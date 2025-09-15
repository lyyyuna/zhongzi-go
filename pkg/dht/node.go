package dht

import (
	"encoding/binary"
	"fmt"
	"net"
	"time"

	. "github.com/lyyyuna/zhongzi-go/pkg/types"
)

type Node struct {
	Id NodeId

	Addr *net.UDPAddr

	created time.Time

	modified time.Time
}

func NewNode(id NodeId, addr *net.UDPAddr) *Node {
	return &Node{
		Id:   id,
		Addr: addr,
	}
}

func (n Node) DistanceTo(target NodeId) XORDistance {
	var distance XORDistance
	for i := range 20 {
		distance[i] = n.Id[i] ^ target[i]
	}
	return distance
}

func NewNodesFromBytes(raw []byte) ([]Node, error) {
	if len(raw)%26 != 0 {
		return nil, fmt.Errorf("invalid raw bytes, cannot divide by 26")
	}

	nodes := make([]Node, 0)
	for i := 0; i < len(raw); i += 26 {
		rawId := raw[i : i+20]
		rawAddr := raw[i+20 : i+26]

		ip := net.IPv4(rawAddr[0], rawAddr[1], rawAddr[2], rawAddr[3])
		port := binary.BigEndian.Uint16(rawAddr[4:6])

		addr := &net.UDPAddr{
			IP:   ip,
			Port: int(port),
		}

		var id NodeId
		copy(id[:], rawId)
		nodes = append(nodes, *NewNode(id, addr))
	}

	return nodes, nil
}

func NewAddrFromByte(raw []byte) net.TCPAddr {
	ip := net.IPv4(raw[0], raw[1], raw[2], raw[3])
	port := binary.BigEndian.Uint16(raw[4:6])

	addr := net.TCPAddr{
		IP:   ip,
		Port: int(port),
	}

	return addr
}
