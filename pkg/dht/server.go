package dht

import (
	"crypto/sha1"
	"encoding/binary"
	"math/rand"
	"net"
	"sync"
	"sync/atomic"

	"github.com/lyyyuna/bencode"
	"github.com/lyyyuna/zhongzi-go/pkg/log"
	. "github.com/lyyyuna/zhongzi-go/pkg/types"
)

type DHTServer struct {
	Id Infohash

	bootstrapNodes []net.UDPAddr

	routingTable *RoutingTable

	maxBootstrapNode int
	maxPeers         int

	rpcIndex     atomic.Uint32
	rpcLocalConn *net.UDPConn
	rpcPendings  map[uint32]chan any
	rpcPendingsL sync.Mutex
}

func NewDHTServer(os ...option) *DHTServer {
	data := make([]byte, 8)
	binary.LittleEndian.PutUint64(data, rand.Uint64())

	d := &DHTServer{
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
		rpcPendings:      make(map[uint32]chan any),
		routingTable:     NewRoutingTable(),
		maxBootstrapNode: 20,
	}

	for _, o := range os {
		o(d)
	}

	return d
}

type option func(*DHTServer)

func WithMaxBootstrapNodes(max int) option {
	return func(d *DHTServer) {
		d.maxBootstrapNode = max
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

	go d.receiveLoop()

	return nil
}

func (d *DHTServer) receiveLoop() {
	buffer := make([]byte, 65536)

	for {
		n, remoteAddr, err := d.rpcLocalConn.ReadFromUDP(buffer)
		if err != nil {
			log.Errorf("Failed to read UDP message: %v", err)
			continue
		}

		log.Debugf("Received %d bytes from %s", n, remoteAddr)

		go d.handleKrpcMessage(buffer[:n], remoteAddr)
	}
}

func (d *DHTServer) handleKrpcMessage(data []byte, remoteAddr *net.UDPAddr) {
	var msg map[string]interface{}
	err := bencode.Unmarshal(data, &msg)
	if err != nil {
		log.Errorf("Failed to unmarshal KRPC message from %s: %v", remoteAddr, err)
		return
	}

	tidRaw, hasTid := msg["t"]
	if !hasTid {
		log.Errorf("KRPC message missing transaction ID from %s", remoteAddr)
		return
	}

	tidBytes, ok := tidRaw.(string)
	if !ok {
		log.Errorf("Invalid transaction ID type from %s", remoteAddr)
		return
	}

	if len(tidBytes) < 2 {
		log.Errorf("Transaction ID too short (need 2 bytes, got %d) from %s", len(tidBytes), remoteAddr)
		return
	}

	tid := uint32(tidBytes[0])<<8 | uint32(tidBytes[1])

	log.Debugf("Extracted transaction ID %d from message: %s", tid, tidBytes)

	d.rpcPendingsL.Lock()
	ch, exists := d.rpcPendings[tid]
	if exists {
		delete(d.rpcPendings, tid)
	}
	d.rpcPendingsL.Unlock()

	if exists {
		log.Debugf("Found pending request for tid %d, sending response to channel", tid)
		select {
		case ch <- msg:
			log.Debugf("Successfully sent response to channel for tid %d", tid)
		default:
			log.Errorf("Failed to send response to pending channel for tid %d", tid)
		}
	} else {
		log.Debugf("Received response for unknown transaction ID %d from %s", tid, remoteAddr)
	}
}
