package internal

import (
	"io"
	"sync"

	"google.golang.org/protobuf/proto"
)

var (
	bufferPool = sync.Pool{
		New: func() interface{} {
			b := make([]byte, 0, 1024)
			return &b
		},
	}
)

func MarshalToWriter(w io.Writer, m proto.Message) (int, error) {
	pBuf := bufferPool.Get().(*[]byte)
	buf := (*pBuf)[:0]
	options := proto.MarshalOptions{}

	data, err := options.MarshalAppend(buf, m)
	if err != nil {
		bufferPool.Put(pBuf)
		return 0, err
	}

	n, err := w.Write(data)

	*pBuf = data
	bufferPool.Put(pBuf)

	return n, err
}
