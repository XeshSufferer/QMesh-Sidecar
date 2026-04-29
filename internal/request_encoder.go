package internal

import (
	"QMesh-Sidecar/internal/protos/pb/gen"
	"strings"
	"sync"

	"github.com/valyala/fasthttp"
)

type header struct {
	Name    string
	Content string
}

var tableSTR2UINT map[string]uint32 = map[string]uint32{
	// Content-Type
	"Content-Type: application/json":                  1,
	"Content-Type: application/x-www-form-urlencoded": 2,
	"Content-Type: text/html; charset=utf-8":          3,
	"Content-Type: text/plain; charset=utf-8":         4,
	"Content-Type: application/grpc":                  5,
	"Content-Type: application/octet-stream":          6,
	"Content-Type: application/x-protobuf":            7,

	// Connection & Encoding
	"Connection: keep-alive":             8,
	"Connection: close":                  9,
	"Accept-Encoding: gzip, deflate, br": 10,
	"Transfer-Encoding: chunked":         11,
	"Vary: Accept-Encoding":              12,

	// Cache Control
	"Cache-Control: no-cache":  13,
	"Cache-Control: no-store":  14,
	"Cache-Control: max-age=0": 15,

	// CORS
	"Access-Control-Allow-Origin: *":                   16,
	"Access-Control-Allow-Methods: GET, POST, OPTIONS": 17,
	"Accept: application/json":                         18,
}

var tableUINT2HEADER map[uint32]header = map[uint32]header{
	1:  {"Content-Type", "application/json"},
	2:  {"Content-Type", "application/x-www-form-urlencoded"},
	3:  {"Content-Type", "text/html; charset=utf-8"},
	4:  {"Content-Type", "text/plain; charset=utf-8"},
	5:  {"Content-Type", "application/grpc"},
	6:  {"Content-Type", "application/octet-stream"},
	7:  {"Content-Type", "application/x-protobuf"},
	8:  {"Connection", "keep-alive"},
	9:  {"Connection", "close"},
	10: {"Accept-Encoding", "gzip, deflate, br"},
	11: {"Transfer-Encoding", "chunked"},
	12: {"Vary", "Accept-Encoding"},
	13: {"Cache-Control", "no-cache"},
	14: {"Cache-Control", "no-store"},
	15: {"Cache-Control", "max-age=0"},
	16: {"Access-Control-Allow-Origin", "*"},
	17: {"Access-Control-Allow-Methods", "GET, POST, OPTIONS"},
	18: {"Accept", "application/json"},
}

var (
	builders = sync.Pool{
		New: func() any {
			return &strings.Builder{}
		},
	}
)

// --- REQUEST ---

func DecodeRequestFast(treq *gen.TunnelRequest) (*fasthttp.Request, error) {
	req := fasthttp.AcquireRequest()

	req.Header.SetMethod(treq.Method.String())
	req.URI().SetScheme("http")
	req.URI().SetHost("localhost:8080")
	req.URI().SetPathBytes(treq.Path)
	req.SetBody(treq.Body)

	for _, v := range treq.PackedHeaders {
		if h, ok := tableUINT2HEADER[v]; ok {
			req.Header.Set(h.Name, h.Content)
		}
	}

	for i := 0; i < len(treq.RawHeaders); i += 2 {
		if i+1 < len(treq.RawHeaders) {
			req.Header.Set(treq.RawHeaders[i], treq.RawHeaders[i+1])
		}
	}

	return req, nil
}

func EncodeRequestFast(req *fasthttp.Request) (*gen.TunnelRequest, error) {
	treq := &gen.TunnelRequest{
		Method: gen.HttpMethod(gen.HttpMethod_value[string(req.Header.Method())]),
		Path:   req.URI().Path(),
		Body:   req.Body(),
	}

	builder := getBuilder()
	defer putBuilder(builder)

	req.Header.VisitAll(func(key, value []byte) {
		buildHeader(key, value, builder)

		fullHeader := builder.String()
		builder.Reset()

		if id, ok := tableSTR2UINT[fullHeader]; ok {
			treq.PackedHeaders = append(treq.PackedHeaders, id)
		} else {
			k, v := string(key), string(value)
			treq.RawHeaders = append(treq.RawHeaders, k, v)
		}
	})

	return treq, nil
}

// --- RESPONSE ---

func EncodeResponseFast(resp *fasthttp.Response) (*gen.TunnelResponse, error) {
	tresp := &gen.TunnelResponse{
		CodeResponse: uint32(resp.StatusCode()),
		Body:         resp.Body(),
	}

	builder := getBuilder()
	defer putBuilder(builder)

	resp.Header.VisitAll(func(key, value []byte) {
		buildHeader(key, value, builder)
		fullHeader := builder.String()
		builder.Reset()

		if id, ok := tableSTR2UINT[fullHeader]; ok {
			tresp.PackedHeaders = append(tresp.PackedHeaders, id)
		} else {
			k, v := string(key), string(value)
			tresp.RawHeaders = append(tresp.RawHeaders, k, v)
		}
	})

	return tresp, nil
}

func DecodeResponseFast(tresp *gen.TunnelResponse) (*fasthttp.Response, error) {
	resp := fasthttp.AcquireResponse()

	resp.SetStatusCode(int(tresp.CodeResponse))
	resp.SetBody(tresp.Body)

	for _, v := range tresp.PackedHeaders {
		if h, ok := tableUINT2HEADER[v]; ok {
			resp.Header.Set(h.Name, h.Content)
		}
	}

	for i := 0; i < len(tresp.RawHeaders); i += 2 {
		if i+1 < len(tresp.RawHeaders) {
			resp.Header.Set(tresp.RawHeaders[i], tresp.RawHeaders[i+1])
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

func buildHeader(k, v []byte, builder *strings.Builder) {
	builder.Write(k)
	builder.WriteString(": ")
	builder.Write(v)
}

func getBuilder() *strings.Builder {
	return builders.Get().(*strings.Builder)
}

func putBuilder(b *strings.Builder) {
	b.Reset()
	builders.Put(b)
}
