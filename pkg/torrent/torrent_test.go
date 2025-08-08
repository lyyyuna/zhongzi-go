package torrent_test

import (
	"os"
	"testing"

	"github.com/lyyyuna/zhongzi-go/pkg/torrent"
)

func TestMarshalFile(t *testing.T) {
	data, _ := os.ReadFile("test.torrent")
	_, err := torrent.NewWithFile(data)
	if err != nil {
		t.Fatalf("failed to create torrent from file: %v", err)
	}
}
