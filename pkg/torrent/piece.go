package torrent

import (
	. "github.com/lyyyuna/zhongzi-go/pkg/types"
)

type TorrentPiece struct {
	Index       int
	Length      int
	Offset      int
	Checksum    Checksum
	Data        []byte
	Blocks      []TorrentBlock
	FileMapping []PieceFileMapping
	Downloaded  bool
}

type PieceFileMapping struct {
	Filename    string
	StartInFile int
	EndInFile   int
	SizeInFile  int
}

type TorrentBlock struct {
	Index  int
	Offset int
	Length int
}

func NewTorrentPiece(index, length, offset int, checksum Checksum) TorrentPiece {
	p := TorrentPiece{
		Index:       index,
		Length:      length,
		Offset:      offset,
		Checksum:    checksum,
		Blocks:      make([]TorrentBlock, 0),
		FileMapping: make([]PieceFileMapping, 0),
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

// GetFileMappings 返回这个 piece 映射到的所有文件
func (p *TorrentPiece) GetFileMappings() []PieceFileMapping {
	return p.FileMapping
}

// GetMappedFiles 返回这个 piece 涉及的所有文件名
func (p *TorrentPiece) GetMappedFiles() []string {
	files := make([]string, 0, len(p.FileMapping))
	for _, mapping := range p.FileMapping {
		files = append(files, mapping.Filename)
	}
	return files
}

// GetDataForFile 获取该 piece 中属于指定文件的数据段
func (p *TorrentPiece) GetDataForFile(filename string) []byte {
	if p.Data == nil {
		return nil
	}

	var dataOffset int
	for _, mapping := range p.FileMapping {
		if mapping.Filename == filename {
			start := dataOffset
			end := min(dataOffset+mapping.SizeInFile, len(p.Data))
			return p.Data[start:end]
		}
		dataOffset += mapping.SizeInFile
	}
	return nil
}
