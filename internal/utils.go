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
	bptr.buff = bptr.buff[:16]

	_, err := rand.Read(bptr.buff)
	if err != nil {
		bptr.buff = bptr.buff[:0]
		bufferPool.Put(bptr)
		panic(err)
	}

	bptr.buff[6] = (bptr.buff[6] & 0x0F) | 0x40
	bptr.buff[8] = (bptr.buff[8] & 0x3F) | 0x80

	// 36 bytes UUID string
	out := make([]byte, 36)

	encode(out[0:2], bptr.buff[0])
	encode(out[2:4], bptr.buff[1])
	encode(out[4:6], bptr.buff[2])
	encode(out[6:8], bptr.buff[3])
	out[8] = '-'

	encode(out[9:11], bptr.buff[4])
	encode(out[11:13], bptr.buff[5])
	out[13] = '-'

	encode(out[14:16], bptr.buff[6])
	encode(out[16:18], bptr.buff[7])
	out[18] = '-'

	encode(out[19:21], bptr.buff[8])
	encode(out[21:23], bptr.buff[9])
	out[23] = '-'

	encode(out[24:26], bptr.buff[10])
	encode(out[26:28], bptr.buff[11])
	encode(out[28:30], bptr.buff[12])
	encode(out[30:32], bptr.buff[13])
	encode(out[32:34], bptr.buff[14])
	encode(out[34:36], bptr.buff[15])

	bptr.buff = bptr.buff[:0]
	bufferPool.Put(bptr)
	return ZeroAllocBytesToString(out)
}
