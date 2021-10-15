package libio

import (
	"github.com/eleztian/pipe/bytespool/ladder"
	"io"
)

func Copy(dst io.Writer, src io.Reader) (int64, error) {
	buf := ladder.Get(32 * 1024)
	defer ladder.Put(buf)
	return io.CopyBuffer(dst, src, buf)
}
