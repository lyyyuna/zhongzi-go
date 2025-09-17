package download

import (
	"context"
	"os"

	"github.com/lyyyuna/zhongzi-go/pkg/log"
	"github.com/lyyyuna/zhongzi-go/pkg/torrent"
	"github.com/spf13/cobra"
)

var DownloadCmd = &cobra.Command{
	Use:   "download",
	Short: "Download using torrent file",
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		log.SetInfoLevel()

		filename := args[0]
		data, err := os.ReadFile(filename)
		if err != nil {
			panic(err)
		}

		tf, err := torrent.NewWithFile(data)
		if err != nil {
			panic(err)
		}

		tc := torrent.NewTorrentClient(tf)

		ctx := context.Background()
		ui := NewDownloadUI(tc, ctx)
		
		if err := ui.Start(); err != nil {
			panic(err)
		}
	},
}
