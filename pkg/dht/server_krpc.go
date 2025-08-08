package dht

import (
	"context"
	"errors"
	"net"

	"github.com/lyyyuna/bencode"
	"github.com/lyyyuna/zhongzi-go/pkg/log"
	"github.com/lyyyuna/zhongzi-go/pkg/types"
)

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

type findNodeQuery struct {
	T string `bencode:"t"`
	Y string `bencode:"y"`
	Q string `bencode:"q"`
	A struct {
		ID     []byte `bencode:"id"`
		Target []byte `bencode:"target"`
	} `bencode:"a"`
}

type findNodeResponse struct {
	T string `bencode:"t"`
	Y string `bencode:"y"`
	R struct {
		ID    []byte `bencode:"id"`
		Nodes []byte `bencode:"nodes"`
	} `bencode:"r,omitempty"`
	E []string `bencode:"e,omitempty"`
}

func (d *DHTServer) findNode(ctx context.Context, targetAddr *net.UDPAddr, targetNode *types.Infohash) (*findNodeResponse, error) {
	query := findNodeQuery{
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

	var response findNodeResponse
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
