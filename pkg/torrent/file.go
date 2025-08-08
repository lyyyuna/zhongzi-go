package torrent

// TorrentFile 代表种子中的一个文件
type TorrentFile struct {
	Length int

	Path string

	StartOffset int // 在整个下载 binary 中的 offset

	EndOffset int
}
