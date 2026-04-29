package internal

import (
	"bytes"
	"io"
	"sync"

	"google.golang.org/protobuf/proto"
)

var (
	bufferPool = sync.Pool{
		New: func() interface{} {
			return bytes.NewBuffer(make([]byte, 0, 1024))
		},
	}
)

func MarshalToWriter(w io.Writer, m proto.Message) (int, error) {
	pBuf := bufferPool.Get().(*bytes.Buffer)
	pBuf.Reset()
	defer bufferPool.Put(pBuf)

	options := proto.MarshalOptions{}

	data, err := options.MarshalAppend(pBuf.Bytes(), m)
	if err != nil {
		return 0, err
	}

	return w.Write(data)
}
