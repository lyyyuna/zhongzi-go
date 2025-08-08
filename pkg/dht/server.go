package dht

import (
	"context"
	"crypto/sha1"
	"encoding/binary"
	"errors"
	"math/rand"
	"net"
	"sync"
	"sync/atomic"

	"github.com/lyyyuna/bencode"
	"github.com/lyyyuna/zhongzi-go/pkg/log"
	. "github.com/lyyyuna/zhongzi-go/pkg/types"
)

type FindNodeQuery struct {
	T string `bencode:"t"`
	Y string `bencode:"y"`
	Q string `bencode:"q"`
	A struct {
		ID     []byte `bencode:"id"`
		Target []byte `bencode:"target"`
	} `bencode:"a"`
}

type FindNodeResponse struct {
	T string `bencode:"t"`
	Y string `bencode:"y"`
	R struct {
		ID    []byte `bencode:"id"`
		Nodes []byte `bencode:"nodes"`
	} `bencode:"r,omitempty"`
	E []string `bencode:"e,omitempty"`
}

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

func (d *DHTServer) sendKrpcMsg(ctx context.Context, targetAddr *net.UDPAddr, msg any) ([]byte, error) {
	id := d.rpcIndex.Add(1)

	data, err := bencode.Marshal(msg)
	if err != nil {
		log.Errorf("failed to marshal krpc message: %v", err)
		return nil, err
	}

	d.rpcPendingsL.Lock()
	d.rpcPendings[id] = make(chan any)
	d.rpcPendingsL.Unlock()

	_, err = d.rpcLocalConn.WriteToUDP(data, targetAddr)
	if err != nil {
		log.Errorf("failed to send krpc message: %v", err)

		d.rpcPendingsL.Lock()
		delete(d.rpcPendings, id)
		d.rpcPendingsL.Unlock()
		return nil, err
	}

	select {
	case <-ctx.Done():
		d.rpcPendingsL.Lock()
		delete(d.rpcPendings, id)
		d.rpcPendingsL.Unlock()
		return nil, ctx.Err()
	case resp := <-d.rpcPendings[id]:
		if respBytes, ok := resp.([]byte); ok {
			return respBytes, nil
		}
		return nil, errors.New("invalid response type")
	}
}

func (d *DHTServer) findNode(ctx context.Context, targetAddr *net.UDPAddr, targetNode *Infohash) (*FindNodeResponse, error) {
	query := FindNodeQuery{
		T: string([]byte{byte(d.rpcIndex.Load())}),
		Y: "q",
		Q: "find_node",
		A: struct {
			ID     []byte `bencode:"id"`
			Target []byte `bencode:"target"`
		}{
			ID:     d.Id[:],
			Target: targetNode[:],
		},
	}

	respData, err := d.sendKrpcMsg(ctx, targetAddr, query)
	if err != nil {
		log.Errorf("find_node query failed: %v", err)
		return nil, err
	}

	var response FindNodeResponse
	err = bencode.Unmarshal(respData, &response)
	if err != nil {
		log.Errorf("failed to unmarshal find_node response: %v", err)
		return nil, err
	}

	if len(response.E) > 0 {
		return nil, errors.New("DHT error response: " + response.E[1])
	}

	log.Infof("find_node response: found %d bytes of node data from node %x", len(response.R.Nodes), response.R.ID)
	return &response, nil
}
