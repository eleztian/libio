package fixed

import "errors"

type Allocator struct {
	freeList chan []byte
	bufSize  int // size of each buffer
}

func NewBytePool(bufNum int, bufSize int) *Allocator {
	return &Allocator{
		freeList: make(chan []byte, bufNum),
		bufSize:  bufSize,
	}
}

// Get returns a buffer from the fixed size pool buffer or create a new buffer.
func (fp *Allocator) Get(size int) (b []byte) {
	if fp.bufSize != size {
		return nil
	}
	select {
	case b = <-fp.freeList:
	default:
		b = make([]byte, fp.bufSize)
	}
	return
}

// Put add the buffer into the free buffer pool for reuse. return error if the buffer
// size is not the same with the fixed size pool buffer's. This is intended to expose
// error usage of fixed size pool buffer.
func (fp *Allocator) Put(b []byte) error {
	if len(b) != fp.bufSize {
		return errors.New("invalid buffer size that's put into fixed size pool buffer")
	}

	select {
	case fp.freeList <- b:
	default:
	}

	return nil
}
