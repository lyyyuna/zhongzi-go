package torrent

import (
	"encoding/hex"
	"sync"
	"time"
)

type TorrentStats struct {
	Name           string
	InfoHash       string
	TotalSize      int64
	PieceCount     int
	PieceLength    int
	DownloadedSize int64  // Actual completed piece data for progress
	TotalBytesRead int64  // Total bytes downloaded including retries for speed calculation
	Progress       float64
	DownloadSpeed  int64
	UploadSpeed    int64
	PeersCount     int
	Downloaded     []bool
	StartTime      time.Time
	DHTCollecting  bool
	
	downloadSpeedLock  sync.RWMutex
	downloadBytes      int64
	lastSpeedUpdate    time.Time
	recentSpeeds       []speedSample
	lastDownloadUpdate time.Time
}

type speedSample struct {
	bytes int64
	time  time.Time
}

func newTorrentStats(tc *TorrentClient) *TorrentStats {
	downloaded := make([]bool, len(tc.torrent.Pieces))
	
	now := time.Now()
	return &TorrentStats{
		Name:               tc.torrent.Name,
		InfoHash:           hex.EncodeToString(tc.infoHash[:]),
		TotalSize:          int64(tc.torrent.Length),
		PieceCount:         len(tc.torrent.Pieces),
		PieceLength:        tc.torrent.PieceLength,
		Downloaded:         downloaded,
		StartTime:          now,
		lastSpeedUpdate:    now,
		lastDownloadUpdate: now,
		recentSpeeds:       make([]speedSample, 0),
	}
}

func (ts *TorrentStats) updateBlockDownloaded(blockSize int) {
	ts.TotalBytesRead += int64(blockSize)
	ts.updateDownloadSpeed(int64(blockSize))
}

func (ts *TorrentStats) updatePieceDownloaded(pieceIndex int, pieceSize int) {
	if pieceIndex >= 0 && pieceIndex < len(ts.Downloaded) {
		if !ts.Downloaded[pieceIndex] {
			ts.Downloaded[pieceIndex] = true
			ts.DownloadedSize += int64(pieceSize)
			ts.updateProgress()
		}
	}
}

func (ts *TorrentStats) updateProgress() {
	if ts.TotalSize > 0 {
		ts.Progress = float64(ts.DownloadedSize) / float64(ts.TotalSize) * 100
	}
}

func (ts *TorrentStats) updateDownloadSpeed(bytes int64) {
	ts.downloadSpeedLock.Lock()
	defer ts.downloadSpeedLock.Unlock()
	
	now := time.Now()
	ts.lastDownloadUpdate = now
	
	ts.recentSpeeds = append(ts.recentSpeeds, speedSample{
		bytes: bytes,
		time:  now,
	})
	
	cutoff := now.Add(-5 * time.Second)
	filtered := ts.recentSpeeds[:0]
	for _, sample := range ts.recentSpeeds {
		if sample.time.After(cutoff) {
			filtered = append(filtered, sample)
		}
	}
	ts.recentSpeeds = filtered
	
	if len(ts.recentSpeeds) > 0 {
		totalBytes := int64(0)
		var earliest, latest time.Time
		
		for i, sample := range ts.recentSpeeds {
			totalBytes += sample.bytes
			if i == 0 {
				earliest = sample.time
				latest = sample.time
			} else {
				if sample.time.Before(earliest) {
					earliest = sample.time
				}
				if sample.time.After(latest) {
					latest = sample.time
				}
			}
		}
		
		timeSpan := latest.Sub(earliest).Seconds()
		if timeSpan <= 0 {
			timeSpan = 1.0 // 至少按1秒计算
		}
		
		ts.DownloadSpeed = int64(float64(totalBytes) / timeSpan)
	}
}

func (ts *TorrentStats) CheckSpeedTimeout() {
	ts.downloadSpeedLock.Lock()
	defer ts.downloadSpeedLock.Unlock()
	
	now := time.Now()
	
	if now.Sub(ts.lastDownloadUpdate) >= 3*time.Second {
		ts.DownloadSpeed = 0
		ts.recentSpeeds = ts.recentSpeeds[:0]
	}
}

func (ts *TorrentStats) updatePeersCount(count int) {
	ts.PeersCount = count
}

func (ts *TorrentStats) updateDHTCollecting(collecting bool) {
	ts.DHTCollecting = collecting
}

func (ts *TorrentStats) GetDownloadedPieceCount() int {
	count := 0
	for _, downloaded := range ts.Downloaded {
		if downloaded {
			count++
		}
	}
	return count
}

func (ts *TorrentStats) GetProgress() float64 {
	return ts.Progress
}

func (ts *TorrentStats) GetDownloadSpeed() int64 {
	ts.downloadSpeedLock.RLock()
	defer ts.downloadSpeedLock.RUnlock()
	return ts.DownloadSpeed
}

func (ts *TorrentStats) GetRemainingTime() time.Duration {
	if ts.DownloadSpeed <= 0 {
		return time.Duration(0)
	}
	remainingBytes := ts.TotalSize - ts.DownloadedSize
	return time.Duration(remainingBytes/ts.DownloadSpeed) * time.Second
}

func (tc *TorrentClient) Stats() *TorrentStats {
	if tc.stats == nil {
		tc.stats = newTorrentStats(tc)
	}
	
	tc.availablePeersLock.Lock()
	tc.stats.updatePeersCount(len(tc.availablePeers))
	tc.availablePeersLock.Unlock()
	
	tc.stats.updateDHTCollecting(tc.isDHTCollecting())
	
	return tc.stats
}
