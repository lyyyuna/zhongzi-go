package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/lyyyuna/zhongzi-go/pkg/log"
	"github.com/lyyyuna/zhongzi-go/pkg/torrent"
	"github.com/spf13/cobra"

	"net/http"
	_ "net/http/pprof"
)

var rootCmd = &cobra.Command{
	Use:   "zhongzi",
	Short: "zhongzi BitTorrent client with DHT support",
	Run:   func(cmd *cobra.Command, args []string) {},
}

var dhtCmd = &cobra.Command{
	Use:   "dht",
	Short: "Test DHT functionality",
	Run: func(cmd *cobra.Command, args []string) {
		go func() {
			http.ListenAndServe("localhost:6060", nil)
		}()

		log.SetInfoLevel()

		data, err := os.ReadFile("test.torrent")
		if err != nil {
			log.Fatalf("Error reading torrent file: %v", err)
		}

		tf, err := torrent.NewWithFile(data)
		if err != nil {
			log.Fatalf("Error parsing torrent file: %v", err)
		}

		log.Infof("Torrent info: %v, pieces: %v", tf.InfoHash, len(tf.Pieces))

		tc := torrent.NewTorrentClient(tf)

		tc.Start(context.Background())

		log.Info("DHT server stopped")
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(dhtCmd)
}
