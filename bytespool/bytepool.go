package bytespool

type BytesPool interface {
	Get(size int) []byte
	Put([]byte) error
}
