package dht

import (
	"crypto/sha1"
	"encoding/binary"
	"math/rand"
	"net"
	"sync"
	"sync/atomic"

	"github.com/lyyyuna/zhongzi-go/pkg/log"
	. "github.com/lyyyuna/zhongzi-go/pkg/types"
)

type DHTServer struct {
	Id Infohash

	bootstrapNodes []net.UDPAddr

	rpcIndex atomic.Uint32

	rpcLocalConn *net.UDPConn

	rpcPendings  map[uint32]chan any
	rpcPendingsL sync.Mutex
}

func NewDHTServer() *DHTServer {
	data := make([]byte, 8)
	binary.LittleEndian.PutUint64(data, rand.Uint64())

	return &DHTServer{
		Id: sha1.Sum(data),
		bootstrapNodes: []net.UDPAddr{
			{
				IP:   net.IPv4(67, 215, 246, 10),
				Port: 6881,
			},
			{
				IP:   net.IPv4(87, 98, 162, 88),
				Port: 6881,
			},
			{
				IP:   net.IPv4(82, 221, 103, 244),
				Port: 6881,
			},
		},
		rpcPendings: make(map[uint32]chan any),
	}
}

func (d *DHTServer) Run() error {
	conn, err := net.ListenUDP("udp", &net.UDPAddr{
		IP:   net.IPv4zero,
		Port: 0,
	})
	if err != nil {
		return err
	}

	log.Infof("DHT server listening on %s", conn.LocalAddr())

	d.rpcLocalConn = conn

	return nil
}
