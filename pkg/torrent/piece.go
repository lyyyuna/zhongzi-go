package torrent

import (
	. "github.com/lyyyuna/zhongzi-go/pkg/types"
)

type TorrentPiece struct {
	Index    int
	Length   int
	Offset   int
	Checksum Checksum
	Data     []byte
	Blocks   []TorrentBlock
}

type TorrentBlock struct {
	Index  int
	Offset int
	Length int
}

func NewTorrentPiece(index, length, offset int, checksum Checksum) TorrentPiece {
	p := TorrentPiece{
		Index:    index,
		Length:   length,
		Offset:   offset,
		Checksum: checksum,
		Blocks:   make([]TorrentBlock, 0),
	}

	blockIndex := 0
	blockLength := 2 << 14
	for blockIndex*blockLength+blockLength < length {
		p.Blocks = append(p.Blocks, TorrentBlock{
			Index:  blockIndex,
			Offset: blockIndex * blockLength,
			Length: blockLength,
		})
		blockIndex += 1
	}

	lastBlockLength := length - blockIndex*blockLength
	p.Blocks = append(p.Blocks, TorrentBlock{
		Index:  blockIndex,
		Offset: blockIndex * blockLength,
		Length: lastBlockLength,
	})

	return p
}
