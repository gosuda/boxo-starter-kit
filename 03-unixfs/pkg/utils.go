package unixfs

const (
	KiB = 1 << 10
	MiB = KiB << 10
	GiB = MiB << 10
)

func GetChunkSize(size int, defaultChunkSize int64) (chunkSize int64) {
	if size < 0 {
		size = 0
	}

	switch {
	case size <= 1*MiB:
		return max(32*KiB, min(defaultChunkSize, int64(size)))
	case size <= 64*MiB:
		return defaultChunkSize
	case size <= 1*GiB:
		return max(defaultChunkSize, 1*MiB)
	default:
		return 4 * MiB
	}
}
