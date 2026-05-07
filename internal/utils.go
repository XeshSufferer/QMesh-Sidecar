package internal

import (
	"crypto/rand"
	"io"
	"sync"
	"unsafe"

	"google.golang.org/protobuf/proto"
)

type buff struct {
	buff []byte
}

var hex = "0123456789abcdef"

var (
	bufferPool = sync.Pool{
		New: func() any {
			return &buff{buff: make([]byte, 0, 1024)}
		},
	}
)

func MarshalToWriter(w io.Writer, m proto.Message) (int, error) {
	pBuf := bufferPool.Get().(*buff)
	pBuf.buff = pBuf.buff[:0]
	defer bufferPool.Put(pBuf)

	options := proto.MarshalOptions{}

	data, err := options.MarshalAppend(pBuf.buff, m)
	if err != nil {
		return 0, err
	}

	if len(data) != len(pBuf.buff) {
		pBuf.buff = data
	}

	return w.Write(data)
}

func ZeroAllocBytesToString(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}

func ZeroAllocStringToBytes(s string) []byte {
	return unsafe.Slice((*byte)(unsafe.Pointer(unsafe.StringData(s))), len(s))
}

func encode(dst []byte, b byte) {
	dst[0] = hex[b>>4]
	dst[1] = hex[b&0x0f]
}

func UUIDv4() string {
	bptr := bufferPool.Get().(*buff)
	b := *bptr

	_, err := rand.Read(b.buff)
	if err != nil {
		bufferPool.Put(bptr)
		panic(err)
	}

	b.buff[6] = (b.buff[6] & 0x0F) | 0x40
	b.buff[8] = (b.buff[8] & 0x3F) | 0x80

	// 36 bytes UUID string
	out := make([]byte, 36)

	encode(out[0:2], b.buff[0])
	encode(out[2:4], b.buff[1])
	encode(out[4:6], b.buff[2])
	encode(out[6:8], b.buff[3])
	out[8] = '-'

	encode(out[9:11], b.buff[4])
	encode(out[11:13], b.buff[5])
	out[13] = '-'

	encode(out[14:16], b.buff[6])
	encode(out[16:18], b.buff[7])
	out[18] = '-'

	encode(out[19:21], b.buff[8])
	encode(out[21:23], b.buff[9])
	out[23] = '-'

	encode(out[24:26], b.buff[10])
	encode(out[26:28], b.buff[11])
	encode(out[28:30], b.buff[12])
	encode(out[30:32], b.buff[13])
	encode(out[32:34], b.buff[14])
	encode(out[34:36], b.buff[15])

	bufferPool.Put(bptr)
	return ZeroAllocBytesToString(out)
}
