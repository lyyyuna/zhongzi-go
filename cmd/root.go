package cmd

import (
	"context"
	"encoding/hex"
	"fmt"
	"os"

	"github.com/lyyyuna/zhongzi-go/pkg/dht"
	"github.com/lyyyuna/zhongzi-go/pkg/log"
	"github.com/lyyyuna/zhongzi-go/pkg/types"
	"github.com/spf13/cobra"
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
		logLevel, _ := cmd.Flags().GetString("log-level")
		switch logLevel {
		case "debug":
			log.SetDebugLevel()
		case "info":
			log.SetInfoLevel()
		default:
			log.SetInfoLevel()
		}
		log.Info("Starting zhongzi-go DHT test")

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		server := dht.NewDHTServer()

		if err := server.Run(); err != nil {
			log.Fatalf("Failed to start DHT server: %v", err)
		}

		log.Info("Starting initial bootstrap...")
		if err := server.Bootstrap(ctx); err != nil {
			log.Errorf("Bootstrap failed: %v", err)
		} else {
			log.Info("Initial bootstrap completed successfully")
		}

		var result [20]byte
		hex.Decode(result[:], []byte("8df9e68813c4232db0506c897ae4c210daa98250"))
		infohash := types.Infohash(result)

		peers := server.GetPeers(ctx, &infohash)
		for _, peer := range peers {
			log.Infof("Found peer: %s", peer.String())
		}

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
	dhtCmd.Flags().StringP("log-level", "l", "info", "Set log level (debug, info)")
	rootCmd.AddCommand(dhtCmd)
}
