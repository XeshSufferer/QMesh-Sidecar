package internal

import (
	"QMesh-Sidecar/internal/protos/pb/gen"
	"bytes"
	"io"
	"strings"
	"sync"

	"github.com/valyala/fasthttp"
)

var methodBytes = [][]byte{
	ZeroAllocStringToBytes("UNSPECIFIED"),
	ZeroAllocStringToBytes("GET"),
	ZeroAllocStringToBytes("POST"),
	ZeroAllocStringToBytes("PUT"),
	ZeroAllocStringToBytes("PATCH"),
	ZeroAllocStringToBytes("DELETE"),
	ZeroAllocStringToBytes("HEAD"),
	ZeroAllocStringToBytes("OPTIONS"),
	ZeroAllocStringToBytes("TRACE"),
	ZeroAllocStringToBytes("CONNECT"),
}

var table = [][]byte{
	ZeroAllocStringToBytes("Content-Type"), ZeroAllocStringToBytes("application/json"),
	ZeroAllocStringToBytes("Content-Type"), ZeroAllocStringToBytes("application/x-www-form-urlencoded"),
	ZeroAllocStringToBytes("Content-Type"), ZeroAllocStringToBytes("text/html; charset=utf-8"),
	ZeroAllocStringToBytes("Content-Type"), ZeroAllocStringToBytes("text/plain; charset=utf-8"),
	ZeroAllocStringToBytes("Content-Type"), ZeroAllocStringToBytes("application/grpc"),
	ZeroAllocStringToBytes("Content-Type"), ZeroAllocStringToBytes("application/octet-stream"),
	ZeroAllocStringToBytes("Content-Type"), ZeroAllocStringToBytes("application/x-protobuf"),

	ZeroAllocStringToBytes("Connection"), ZeroAllocStringToBytes("keep-alive"),
	ZeroAllocStringToBytes("Connection"), ZeroAllocStringToBytes("close"),
	ZeroAllocStringToBytes("Accept-Encoding"), ZeroAllocStringToBytes("gzip, deflate, br"),
	ZeroAllocStringToBytes("Transfer-Encoding"), ZeroAllocStringToBytes("chunked"),
	ZeroAllocStringToBytes("Vary"), ZeroAllocStringToBytes("Accept-Encoding"),

	ZeroAllocStringToBytes("Cache-Control"), ZeroAllocStringToBytes("no-cache"),
	ZeroAllocStringToBytes("Cache-Control"), ZeroAllocStringToBytes("no-store"),
	ZeroAllocStringToBytes("Cache-Control"), ZeroAllocStringToBytes("max-age=0"),

	ZeroAllocStringToBytes("Access-Control-Allow-Origin"), ZeroAllocStringToBytes("*"),
	ZeroAllocStringToBytes("Access-Control-Allow-Methods"), ZeroAllocStringToBytes("GET, POST, OPTIONS"),
	ZeroAllocStringToBytes("Accept"), ZeroAllocStringToBytes("application/json"),
}

var (
	builders = sync.Pool{
		New: func() any {
			return &strings.Builder{}
		},
	}

	byteBuffers = sync.Pool{
		New: func() any {
			return bytes.NewBuffer(make([]byte, 0, 256))
		},
	}

	responses = sync.Pool{
		New: func() any {
			return &gen.TunnelResponse{
				PackedHeaders: make([]uint32, 0, len(table)/2),
				RawHeaders:    make([][]byte, 0, 32),
			}
		},
	}

	requests = sync.Pool{
		New: func() any {
			return &gen.TunnelRequest{
				PackedHeaders: make([]uint32, 0, len(table)/2),
				RawHeaders:    make([][]byte, 0, 32),
			}
		},
	}
)

// --- REQUEST ---

func DecodeRequestFast(treq *gen.TunnelRequest) (*fasthttp.Request, error) {
	req := fasthttp.AcquireRequest()

	if int(treq.Method) < len(methodBytes) {
		req.Header.SetMethodBytes(methodBytes[treq.Method])
	}

	buff := getBuff()
	defer putBuff(buff)

	buff.Write(ZeroAllocStringToBytes("http://localhost:8080"))
	buff.Write(treq.Path)
	req.SetRequestURI(ZeroAllocBytesToString(buff.Bytes()))
	req.SetBody(treq.Body)

	// packed headers
	for _, v := range treq.PackedHeaders {
		if k, val, ok := getHeaderByID(v); ok {
			req.Header.SetBytesKV(k, val)
		}
	}

	// raw headers
	for i := 0; i < len(treq.RawHeaders); i += 2 {
		if i+1 < len(treq.RawHeaders) {
			req.Header.SetBytesKV(treq.RawHeaders[i], treq.RawHeaders[i+1])
		}
	}

	return req, nil
}

func EncodeRequestFast(req *fasthttp.Request) (*gen.TunnelRequest, error) {
	treq := getRequest()

	treq.Method = gen.HttpMethod(gen.HttpMethod_value[ZeroAllocBytesToString(req.Header.Method())])
	treq.Path = req.URI().Path()
	treq.Body = req.Body()

	req.Header.VisitAll(func(key, value []byte) {
		if id, ok := findHeaderID(key, value); ok {
			treq.PackedHeaders = append(treq.PackedHeaders, id)
		} else {
			treq.RawHeaders = append(treq.RawHeaders, key, value)
		}
	})

	return treq, nil
}

// --- RESPONSE

func EncodeResponseFast(resp *fasthttp.Response) (*gen.TunnelResponse, error) {
	tresp := getResponse()

	tresp.CodeResponse = uint32(resp.StatusCode())
	tresp.Body = resp.Body()

	resp.Header.VisitAll(func(key, value []byte) {
		if id, ok := findHeaderID(key, value); ok {
			tresp.PackedHeaders = append(tresp.PackedHeaders, id)
		} else {
			tresp.RawHeaders = append(tresp.RawHeaders, key, value)
		}
	})

	return tresp, nil
}

func DecodeResponseFast(tresp *gen.TunnelResponse) (*fasthttp.Response, error) {
	resp := fasthttp.AcquireResponse()

	resp.SetStatusCode(int(tresp.CodeResponse))
	resp.SetBody(tresp.Body)

	for _, v := range tresp.PackedHeaders {
		if k, val, ok := getHeaderByID(v); ok {
			resp.Header.SetBytesKV(k, val)
		}
	}

	for i := 0; i < len(tresp.RawHeaders); i += 2 {
		if i+1 < len(tresp.RawHeaders) {
			resp.Header.SetBytesKV(tresp.RawHeaders[i], tresp.RawHeaders[i+1])
		}
	}

	return resp, nil
}

// --- UTILS ---

func ReleaseRequest(req *fasthttp.Request) {
	fasthttp.ReleaseRequest(req)
}

func ReleaseResponse(resp *fasthttp.Response) {
	fasthttp.ReleaseResponse(resp)
}

func ReleaseEncodedResponse(resp *gen.TunnelResponse) {
	putResponse(resp)
}

func ReleaseEncodedRequest(req *gen.TunnelRequest) {
	putRequest(req)
}

func buildHeader(k, v []byte, builder io.Writer) {
	builder.Write(k)
	builder.Write(ZeroAllocStringToBytes(": "))
	builder.Write(v)
}

func getRequest() *gen.TunnelRequest {
	return requests.Get().(*gen.TunnelRequest)
}

func putRequest(req *gen.TunnelRequest) {
	req.Method = 0
	req.Path = nil
	req.Body = nil
	req.PackedHeaders = req.PackedHeaders[:0]
	req.RawHeaders = req.RawHeaders[:0]
	requests.Put(req)
}

func getResponse() *gen.TunnelResponse {
	return responses.Get().(*gen.TunnelResponse)
}

func putResponse(resp *gen.TunnelResponse) {
	resp.CodeResponse = 0
	resp.Body = nil
	resp.PackedHeaders = resp.PackedHeaders[:0]
	resp.RawHeaders = resp.RawHeaders[:0]
	responses.Put(resp)
}

func getBuilder() *strings.Builder {
	return builders.Get().(*strings.Builder)
}

func putBuilder(b *strings.Builder) {
	b.Reset()
	builders.Put(b)
}

func getBuff() *bytes.Buffer {
	return byteBuffers.Get().(*bytes.Buffer)
}

func putBuff(arr *bytes.Buffer) {
	arr.Reset()
	byteBuffers.Put(arr)
}

func findHeaderID(key, val []byte) (uint32, bool) {
	var id uint32 = 1

	for i := 0; i < len(table); i += 2 {
		if bytes.Equal(table[i], key) && bytes.Equal(table[i+1], val) {
			return id, true
		}
		id++
	}
	return 0, false
}

func getHeaderByID(id uint32) (k, v []byte, ok bool) {
	if id == 0 {
		return nil, nil, false
	}
	i := int((id - 1) * 2)
	if i+1 >= len(table) {
		return nil, nil, false
	}
	return table[i], table[i+1], true
}
