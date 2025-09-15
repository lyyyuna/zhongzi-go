package torrent

import "encoding/binary"

type HaveMessage struct {
	PieceIndex int
}

func NewHaveMessage(data []byte) *HaveMessage {
	pieceIndex := int(binary.BigEndian.Uint32(data))
	return &HaveMessage{
		PieceIndex: pieceIndex,
	}
}

func (h *HaveMessage) Encode() []byte {
	buf := make([]byte, 9)

	// Message length (5 bytes: 1 byte for message ID + 4 bytes for piece index)
	binary.BigEndian.PutUint32(buf[0:4], 5)

	// Message ID for Have message
	buf[4] = byte(Have)

	// Piece index
	binary.BigEndian.PutUint32(buf[5:9], uint32(h.PieceIndex))

	return buf
}

type BitfieldMessage struct {
	Bitfield []byte
}

func NewBitfieldMessage(data []byte) *BitfieldMessage {
	return &BitfieldMessage{
		Bitfield: data,
	}
}

type PieceMessage struct {
	Index int
	Begin int
	Block []byte
}

func NewPieceMessage(data []byte) *PieceMessage {
	index := int(binary.BigEndian.Uint32(data[0:4]))
	begin := int(binary.BigEndian.Uint32(data[4:8]))
	block := data[8:]
	return &PieceMessage{
		Index: index,
		Begin: begin,
		Block: block,
	}
}

type RequestMessage struct {
	Index  int
	Begin  int
	Length int
}

func MakeRequestMessage(index, begin, length int) *RequestMessage {
	return &RequestMessage{
		Index:  index,
		Begin:  begin,
		Length: length,
	}
}

func (r *RequestMessage) Encode() []byte {
	buf := make([]byte, 17)

	binary.BigEndian.PutUint32(buf[0:4], 13)

	buf[4] = byte(Request)

	binary.BigEndian.PutUint32(buf[5:9], uint32(r.Index))
	binary.BigEndian.PutUint32(buf[9:13], uint32(r.Begin))
	binary.BigEndian.PutUint32(buf[13:17], uint32(r.Length))

	return buf
}
