package download

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/lyyyuna/zhongzi-go/pkg/torrent"
	"github.com/rivo/tview"
)

type DownloadUI struct {
	app            *tview.Application
	torrentClient  *torrent.TorrentClient
	ctx            context.Context
	
	infoText       *tview.TextView
	progressBar    *tview.TextView
	pieceView      *tview.TextView
	speedText      *tview.TextView
	
	stopChan       chan struct{}
	updateTicker   *time.Ticker
}

func NewDownloadUI(tc *torrent.TorrentClient, ctx context.Context) *DownloadUI {
	app := tview.NewApplication()
	
	ui := &DownloadUI{
		app:           app,
		torrentClient: tc,
		ctx:           ctx,
		stopChan:      make(chan struct{}),
		updateTicker:  time.NewTicker(500 * time.Millisecond),
	}
	
	ui.setupUI()
	return ui
}

func (ui *DownloadUI) setupUI() {
	ui.infoText = tview.NewTextView()
	ui.infoText.SetText("Torrent Download Status").
		SetTextAlign(tview.AlignCenter)
	ui.infoText.SetBorder(true).SetTitle("Torrent Info")
	
	ui.progressBar = tview.NewTextView()
	ui.progressBar.SetBorder(true).SetTitle("Download Progress")
	
	ui.pieceView = tview.NewTextView()
	ui.pieceView.SetWrap(true)
	ui.pieceView.SetBorder(true).SetTitle("Piece Status")
	
	ui.speedText = tview.NewTextView()
	ui.speedText.SetBorder(true).SetTitle("Speed & Statistics")
	
	grid := tview.NewGrid().
		SetRows(3, 3, 0, 6).
		SetColumns(0, 0).
		SetBorders(false)
	
	grid.AddItem(ui.infoText, 0, 0, 1, 2, 0, 0, false).
		AddItem(ui.progressBar, 1, 0, 1, 2, 0, 0, false).
		AddItem(ui.pieceView, 2, 0, 1, 2, 0, 0, false).
		AddItem(ui.speedText, 3, 0, 1, 2, 0, 0, false)
	
	ui.app.SetRoot(grid, true).EnableMouse(true)
	
	ui.app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Rune() == 'q' || event.Key() == tcell.KeyEscape {
			ui.Stop()
			return nil
		}
		return event
	})
	
	grid.SetTitle("Zhongzi Torrent Client - Press 'q' or Esc to quit").SetBorder(true)
}

func (ui *DownloadUI) Start() error {
	go ui.updateLoop()
	
	go func() {
		err := ui.torrentClient.Run(ui.ctx)
		if err != nil {
			ui.app.Stop()
		}
	}()
	
	return ui.app.Run()
}

func (ui *DownloadUI) Stop() {
	close(ui.stopChan)
	ui.updateTicker.Stop()
	ui.app.Stop()
}

func (ui *DownloadUI) updateLoop() {
	for {
		select {
		case <-ui.stopChan:
			return
		case <-ui.updateTicker.C:
			ui.updateUI()
		}
	}
}

func (ui *DownloadUI) updateUI() {
	stats := ui.torrentClient.Stats()
	if stats == nil {
		return
	}
	
	stats.CheckSpeedTimeout()
	
	ui.app.QueueUpdateDraw(func() {
		ui.updateInfoText(stats)
		ui.updateProgressBar(stats)
		ui.updatePieceView(stats)
		ui.updateSpeedText(stats)
	})
}

func (ui *DownloadUI) updateInfoText(stats *torrent.TorrentStats) {
	info := fmt.Sprintf("Name: %s\nInfoHash: %s\nTotal Size: %s\nPieces: %d",
		stats.Name,
		stats.InfoHash[:16]+"...", 
		formatBytes(stats.TotalSize),
		stats.PieceCount)
	
	ui.infoText.SetText(info)
}

func (ui *DownloadUI) updateProgressBar(stats *torrent.TorrentStats) {
	progress := stats.GetProgress()
	downloaded := formatBytes(stats.DownloadedSize)
	total := formatBytes(stats.TotalSize)
	
	_, _, width, _ := ui.progressBar.GetRect()
	barWidth := max(width-4, 10) // Account for border and padding
	
	filledWidth := int(progress * float64(barWidth) / 100)
	emptyWidth := barWidth - filledWidth
	
	progressText := fmt.Sprintf("Progress: %.1f%% (%s / %s)\n[%s%s]",
		progress,
		downloaded,
		total,
		strings.Repeat("█", filledWidth),
		strings.Repeat("░", emptyWidth))
	
	ui.progressBar.SetText(progressText)
}

func (ui *DownloadUI) updatePieceView(stats *torrent.TorrentStats) {
	_, _, width, _ := ui.pieceView.GetRect()
	piecesPerRow := max(width-4, 10) // Account for border and padding
	
	var pieceDisplay strings.Builder
	
	pieceDisplay.WriteString(fmt.Sprintf("Downloaded: %d/%d pieces\n\n",
		stats.GetDownloadedPieceCount(), stats.PieceCount))
	
	for i, downloaded := range stats.Downloaded {
		if i > 0 && i%piecesPerRow == 0 {
			pieceDisplay.WriteString("\n")
		}
		
		if downloaded {
			pieceDisplay.WriteString("█") 
		} else {
			pieceDisplay.WriteString("░") 
		}
	}
	
	ui.pieceView.SetText(pieceDisplay.String())
}

func (ui *DownloadUI) updateSpeedText(stats *torrent.TorrentStats) {
	speed := stats.GetDownloadSpeed()
	remainingTime := stats.GetRemainingTime()
	elapsed := time.Since(stats.StartTime)
	
	dhtStatus := "💤 Idle"
	if stats.DHTCollecting {
		dhtStatus = "🔍 Collecting Peers"
	}
	
	speedInfo := fmt.Sprintf("Download Speed: %s/s\nPeers: %d\nDHT: %s\nElapsed: %s\nRemaining: %s",
		formatBytes(speed),
		stats.PeersCount,
		dhtStatus,
		formatDuration(elapsed),
		formatDuration(remainingTime))
	
	ui.speedText.SetText(speedInfo)
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	
	units := []string{"KB", "MB", "GB", "TB"}
	return fmt.Sprintf("%.1f %s", float64(bytes)/float64(div), units[exp])
}

func formatDuration(d time.Duration) string {
	if d == 0 {
		return "∞"
	}
	
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60
	
	if hours > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
	}
	return fmt.Sprintf("%02d:%02d", minutes, seconds)
}