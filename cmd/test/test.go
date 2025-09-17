package test

import (
	"context"
	"net/http"
	"os"

	"github.com/lyyyuna/zhongzi-go/pkg/log"
	"github.com/lyyyuna/zhongzi-go/pkg/torrent"
	"github.com/spf13/cobra"
)

var TestCmd = &cobra.Command{
	Use:   "test",
	Short: "Test torrent functionality",
	Run: func(cmd *cobra.Command, args []string) {
		go func() {
			http.ListenAndServe("localhost:6060", nil)
		}()

		log.SetInfoLevel()

		data, err := os.ReadFile("ubuntu-24.10-live-server-amd64.iso.torrent")
		if err != nil {
			log.Fatalf("Error reading torrent file: %v", err)
		}

		tf, err := torrent.NewWithFile(data)
		if err != nil {
			log.Fatalf("Error parsing torrent file: %v", err)
		}

		log.Infof("Torrent info: %v, pieces: %v", tf.InfoHash, len(tf.Pieces))

		tc := torrent.NewTorrentClient(tf)

		tc.Run(context.Background())

		log.Info("DHT server stopped")
	},
}
