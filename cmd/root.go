package cmd

import (
	"fmt"
	"os"

	"github.com/lyyyuna/zhongzi-go/cmd/download"
	"github.com/lyyyuna/zhongzi-go/cmd/test"
	"github.com/spf13/cobra"

	_ "net/http/pprof"
)

var rootCmd = &cobra.Command{
	Use:   "zhongzi",
	Short: "zhongzi BitTorrent client with DHT support",
	Run:   func(cmd *cobra.Command, args []string) {},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(test.TestCmd)
	rootCmd.AddCommand(download.DownloadCmd)
}
