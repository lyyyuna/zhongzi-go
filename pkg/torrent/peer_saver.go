package torrent

import (
	"context"
	"os"
	"path/filepath"

	"github.com/lyyyuna/zhongzi-go/pkg/log"
)

func (tc *TorrentClient) fileSaver(ctx context.Context) error {
	downloadedPiece := 0

	// 准备好文件
	fdMap := make(map[string]*os.File)

	for _, file := range tc.torrent.Files {
		filePath := filepath.Join(tc.downloadPath, file.Path)
		err := os.MkdirAll(filepath.Dir(filePath), 0666)
		if err != nil {
			log.Errorf("mkdir failed: %v", err)
			return err
		}

		f, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY, 0666)
		if err != nil {
			log.Errorf("open file failed: %v", err)
			return err
		}
		fdMap[file.Path] = f
	}

	for piece := range tc.pieceFileQueue {
		select {
		case <-ctx.Done():
			log.Infof("file saver shutdown")
			return ctx.Err()
		default:
		}

		piece.Downloaded = true

		fileMappings := piece.GetFileMappings()
		for _, fileMapping := range fileMappings {
			fd := fdMap[fileMapping.Filename]

			_, err := fd.Seek(int64(piece.Index*tc.torrent.PieceLength), 0)
			if err != nil {
				log.Errorf("seek file failed: %v", err)
				return err
			}

			_, err = fd.Write(piece.Data)
			if err != nil {
				log.Errorf("write file failed: %v", err)
				return err
			}

			err = fd.Sync()
			if err != nil {
				log.Errorf("sync file failed: %v", err)
				return err
			}
		}

		log.Infof("piece %v saved, %v/%v", piece.Index, downloadedPiece, len(tc.torrent.Pieces))

		downloadedPiece++
		if downloadedPiece == len(tc.torrent.Pieces) {
			log.Infof("download completed")
			return nil
		}
	}

	return nil
}
