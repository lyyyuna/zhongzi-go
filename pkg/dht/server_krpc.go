package dht

import (
	"context"
	"errors"
	"fmt"
	"net"

	"github.com/lyyyuna/bencode"
	"github.com/lyyyuna/zhongzi-go/pkg/log"
	"github.com/lyyyuna/zhongzi-go/pkg/types"
)

func (d *DHTServer) sendKrpcMsg(ctx context.Context, targetAddr *net.UDPAddr, tid uint32, msg any) ([]byte, error) {

	data, err := bencode.Marshal(msg)
	if err != nil {
		log.Errorf("failed to marshal krpc message: %v", err)
		return nil, err
	}

	log.Debugf("Sending KRPC message to %s with tid %d, data: %s", targetAddr, tid, string(data))

	ch := make(chan any, 1)

	d.rpcPendingsL.Lock()
	d.rpcPendings[tid] = ch
	d.rpcPendingsL.Unlock()

	_, err = d.rpcLocalConn.WriteToUDP(data, targetAddr)
	if err != nil {
		log.Errorf("failed to send krpc message: %v", err)

		d.rpcPendingsL.Lock()
		delete(d.rpcPendings, tid)
		d.rpcPendingsL.Unlock()
		return nil, err
	}

	log.Debugf("Successfully sent KRPC message to %s, waiting for response with tid %d", targetAddr, tid)

	select {
	case <-ctx.Done():
		d.rpcPendingsL.Lock()
		delete(d.rpcPendings, tid)
		d.rpcPendingsL.Unlock()
		return nil, ctx.Err()
	case resp := <-ch:
		if respMsg, ok := resp.(map[string]interface{}); ok {
			return bencode.Marshal(respMsg)
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

func (d *DHTServer) findNode(ctx context.Context, targetAddr *net.UDPAddr, targetNode *types.Infohash) ([]Node, error) {
	id := d.rpcIndex.Add(1)
	query := findNodeQuery{
		T: string([]byte{byte(id >> 8), byte(id)}),
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

	respData, err := d.sendKrpcMsg(ctx, targetAddr, id, query)
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

	nodes, err := NewNodesFromBytes(response.R.Nodes)
	if err != nil {
		log.Errorf("Failed to parse nodes from find_node response: %v", err)
		return nil, err
	}

	log.Infof("Received %d nodes from find_node request", len(nodes))
	return nodes, nil
}

type getPeersQuery struct {
	T string `bencode:"t"`
	Y string `bencode:"y"`
	Q string `bencode:"q"`
	A struct {
		ID       []byte `bencode:"id"`
		InfoHash []byte `bencode:"info_hash"`
	} `bencode:"a"`
}

type getPeersResponse struct {
	T string `bencode:"t"`
	Y string `bencode:"y"`
	R struct {
		ID        []byte   `bencode:"id"`
		ValuesRaw [][]byte `bencode:"values,omitempty"`
		NodesRaw  []byte   `bencode:"nodes,omitempty"`
	} `bencode:"r,omitempty"`
	E []string `bencode:"e,omitempty"`
}

func (d *DHTServer) getPeers(ctx context.Context, targetAddr *net.UDPAddr, targetInfoHash *types.Infohash) ([]Node, []net.TCPAddr, error) {
	id := d.rpcIndex.Add(1)
	query := getPeersQuery{
		T: string([]byte{byte(id >> 8), byte(id)}),
		Y: "q",
		Q: "get_peers",
		A: struct {
			ID       []byte `bencode:"id"`
			InfoHash []byte `bencode:"info_hash"`
		}{
			ID:       d.Id[:],
			InfoHash: targetInfoHash[:],
		},
	}

	respData, err := d.sendKrpcMsg(ctx, targetAddr, id, query)
	if err != nil {
		log.Errorf("get_peers query failed: %v, ignore node: %v", err, targetAddr.String())
		return nil, nil, err
	}

	var response getPeersResponse
	err = bencode.Unmarshal(respData, &response)
	if err != nil {
		log.Errorf("failed to unmarshal find_node response: %v", err)
		return nil, nil, err
	}

	if len(response.E) > 0 {
		return nil, nil, errors.New("DHT error response: " + response.E[1])
	}

	nodesRaw := response.R.NodesRaw
	valuesRaw := response.R.ValuesRaw

	if len(valuesRaw) != 0 {
		values := make([]net.TCPAddr, 0)
		for _, valueRaw := range valuesRaw {
			values = append(values, NewAddrFromByte(valueRaw))
		}
		return nil, values, nil
	}

	if len(nodesRaw) != 0 {
		nodes, err := NewNodesFromBytes(nodesRaw)
		if err != nil {
			log.Errorf("Failed to parse nodes from get_peers response: %v", err)
			return nil, nil, err
		}

		return nodes, nil, nil
	}

	return nil, nil, fmt.Errorf("no nodes or values found in get_peers response")
}
