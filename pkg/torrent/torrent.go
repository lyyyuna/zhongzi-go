package torrent

import (
	"crypto/sha1"
	"path/filepath"

	"github.com/lyyyuna/bencode"
	"github.com/lyyyuna/zhongzi-go/pkg/log"
	. "github.com/lyyyuna/zhongzi-go/pkg/types"
)

type Torrent struct {
	Name string

	AnnounceUrl []string

	Files []TorrentFile

	Pieces []TorrentPiece

	InfoHash Infohash

	Length int

	PieceLength int
}

type torrentInfo struct {
	Announce string `bencode:"announce"`

	AnnounceList [][]string `bencode:"announce-list"`

	Info struct {
		Name string `bencode:"name"`

		PieceLength int `bencode:"piece length"`

		Files []struct {
			Length int      `bencode:"length"`
			Paths  []string `bencode:"path"`
		} `bencode:"files"`

		Length int `bencode:"length"`

		Pieces []byte `bencode:"pieces"`
	} `bencode:"info"`
}

func NewTorrent() *Torrent {
	return &Torrent{}
}

func NewWithFile(data []byte) (*Torrent, error) {
	var raw map[string]any
	err := bencode.Unmarshal(data, &raw)
	if err != nil {
		log.Errorf("failed to unmarshal torrent file: %v", err)
		return nil, err
	}

	info_raw_data, err := bencode.Marshal(raw["info"])
	if err != nil {
		log.Errorf("failed to marshal info: %v", err)
		return nil, err
	}

	info_hash := sha1.Sum(info_raw_data)

	t := &Torrent{
		InfoHash:    info_hash,
		AnnounceUrl: make([]string, 0),
		Files:       make([]TorrentFile, 0),
		Pieces:      make([]TorrentPiece, 0),
	}

	var tinfo torrentInfo
	err = bencode.Unmarshal(data, &tinfo)
	if err != nil {
		log.Errorf("failed to unmarshal torrent info: %v", err)
		return nil, err
	}

	// get total file length
	if len(tinfo.Info.Files) == 0 {
		t.Length = tinfo.Info.Length
	} else { // it is multi files torrent
		for _, file := range tinfo.Info.Files {
			t.Length += file.Length
		}
	}

	// get piece length
	t.PieceLength = tinfo.Info.PieceLength

	// get all announce urls
	t.AnnounceUrl = append(t.AnnounceUrl, tinfo.Announce)

	for _, announces := range tinfo.AnnounceList {
		for _, annouce := range announces {
			t.AnnounceUrl = append(t.AnnounceUrl, annouce)
		}
	}

	// get name
	t.Name = tinfo.Info.Name

	// get files
	if len(tinfo.Info.Files) == 0 {
		t.Files = append(t.Files, TorrentFile{
			Length:      t.Length,
			Path:        t.Name,
			StartOffset: 0,
			EndOffset:   t.Length - 1,
		})
	} else {
		currentOffset := 0
		for _, file := range tinfo.Info.Files {
			t.Files = append(t.Files, TorrentFile{
				Length:      file.Length,
				Path:        filepath.Join(t.Name, filepath.Join(file.Paths...)),
				StartOffset: currentOffset,
				EndOffset:   currentOffset + file.Length - 1,
			})

			currentOffset += file.Length
		}
	}

	// get pieces
	func() {
		offset := 0
		cnt_length := 0
		for offset < len(tinfo.Info.Pieces) {
			this_piece_length := t.PieceLength
			if cnt_length+t.PieceLength > t.Length {
				this_piece_length = t.Length - cnt_length
			}
			cnt_length += this_piece_length

			var checksum Checksum
			copy(checksum[:], tinfo.Info.Pieces[offset:offset+20])

			t.Pieces = append(t.Pieces, NewTorrentPiece(offset/20, this_piece_length, cnt_length, checksum))

			offset += 20
		}
	}()

	return t, nil
}

type PieceLocation struct {
	Filename    string
	StartInFile int
	EndInFile   int
	SizeInFile  int
}

func (t *Torrent) FindPieceLocation(pieceIndex int) []PieceLocation {
	pieceLocations := make([]PieceLocation, 0)

	pieceStart := pieceIndex * t.PieceLength
	pieceEnd := pieceStart + t.PieceLength - 1

	firstFileIndex := -1

	low, high := 0, len(t.Files)
	for low < high {
		mid := (low + high) / 2
		entry := t.Files[mid]

		if entry.StartOffset <= pieceStart && pieceStart <= entry.EndOffset {
			firstFileIndex = mid
			break
		} else if pieceStart < mid {
			high = mid - 1
		} else {
			low = mid + 1
		}
	}

	if firstFileIndex == -1 {
		log.Errorf("piece index %v invalid, cannot locate it in files", pieceIndex)
		return nil
	}

	remainingBytes := t.PieceLength
	currentOffset := pieceStart

	for i := firstFileIndex; i < len(t.Files); i++ {
		if remainingBytes <= 0 {
			break
		}

		currentFile := t.Files[i]
		fileStart := currentFile.StartOffset
		fileEnd := currentFile.EndOffset

		// 计算在当前文件中的部分
		segmentStart := max(currentOffset, fileStart)
		segmentEnd := min(pieceEnd, fileEnd)
		segmentSize := segmentEnd - segmentStart + 1

		// 确保不会超出剩余需要的字节数
		actualSize := min(segmentSize, remainingBytes)
		if actualSize <= 0 {
			continue
		}

		pieceLocations = append(pieceLocations, PieceLocation{
			Filename:    currentFile.Path,
			StartInFile: segmentStart - fileStart,
			EndInFile:   segmentStart - fileStart + actualSize - 1,
			SizeInFile:  actualSize,
		})

		remainingBytes -= actualSize
		currentOffset += actualSize
	}

	return pieceLocations
}
